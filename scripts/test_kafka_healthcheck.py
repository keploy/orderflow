#!/usr/bin/env python3
"""Unit tests pinning docker-compose.yml's non-JVM Kafka ApiVersions probe."""

import os
import pathlib
import socket
import struct
import subprocess
import threading
import unittest

import yaml

COMPOSE_PATH = pathlib.Path(__file__).resolve().parent.parent / "docker-compose.yml"


def load_kafka_healthcheck():
    with COMPOSE_PATH.open("r", encoding="utf-8") as fh:
        doc = yaml.safe_load(fh)
    return doc["services"]["kafka"]["healthcheck"]


def read_api_versions_req(conn):
    buf = bytearray()
    while len(buf) < 14:
        chunk = conn.recv(14 - len(buf))
        if not chunk:
            return None
        buf.extend(chunk)
    size, api_key, api_ver, corr_id, client_len = struct.unpack(">ihhih", bytes(buf))
    if size != 10 or client_len != -1:
        return None
    return api_key, api_ver, corr_id


def encode_api_versions_resp(corr_id, err_code, entries):
    body_len = 4 + 2 + 4 + 6 * len(entries)
    out = bytearray(struct.pack(">iihi", body_len, corr_id, err_code, len(entries)))
    for api_key, min_ver, max_ver in entries:
        out.extend(struct.pack(">hhh", api_key, min_ver, max_ver))
    return bytes(out)


class FakeTCPServer:
    def __init__(self, handler):
        self._handler = handler
        self._sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self._sock.bind(("127.0.0.1", 0))
        self._sock.listen(5)
        self.port = self._sock.getsockname()[1]
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._serve, daemon=True)
        self._thread.start()

    def _serve(self):
        self._sock.settimeout(0.2)
        while not self._stop.is_set():
            try:
                conn, _ = self._sock.accept()
            except socket.timeout:
                continue
            except OSError:
                return
            with conn:
                conn.settimeout(5.0)
                try:
                    self._handler(conn)
                except OSError:
                    pass

    def close(self):
        self._stop.set()
        try:
            self._sock.close()
        except OSError:
            pass
        self._thread.join(timeout=2.0)

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()


def run_probe(test_cmd, port):
    env = os.environ.copy()
    env["KAFKA_PROBE_HOST"] = "127.0.0.1"
    env["KAFKA_PROBE_PORT"] = str(port)
    if test_cmd[0] == "CMD":
        argv = test_cmd[1:]
    elif test_cmd[0] == "CMD-SHELL":
        argv = ["sh", "-c", test_cmd[1]]
    else:
        raise AssertionError(f"unsupported healthcheck form: {test_cmd!r}")
    return subprocess.run(
        argv,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        timeout=10,
        check=False,
    )


class KafkaHealthcheckTest(unittest.TestCase):
    def test_uses_non_jvm_probe_within_existing_bounds(self):
        hc = load_kafka_healthcheck()
        test_cmd = hc.get("test")
        self.assertIsInstance(test_cmd, list)
        self.assertGreaterEqual(len(test_cmd), 2)
        self.assertIn(test_cmd[0], ("CMD", "CMD-SHELL"))
        joined = " ".join(test_cmd)
        for forbidden in (
            "kafka-broker-api-versions",
            "kafka-topics",
            "kafka-metadata",
            "java",
        ):
            self.assertNotIn(forbidden, joined)
        self.assertEqual(hc.get("interval"), "15s")
        self.assertEqual(hc.get("timeout"), "10s")
        self.assertEqual(hc.get("retries"), 10)
        self.assertIsNone(hc.get("start_period"))

    def test_accepts_valid_api_versions_response(self):
        test_cmd = load_kafka_healthcheck()["test"]
        seen = []

        def handle(conn):
            req = read_api_versions_req(conn)
            seen.append(req)
            if req is None or req[0] != 18 or req[1] != 0:
                return
            conn.sendall(encode_api_versions_resp(req[2], 0, [(0, 0, 9), (18, 0, 3)]))

        with FakeTCPServer(handle) as srv:
            res = run_probe(test_cmd, srv.port)
        self.assertEqual(res.returncode, 0, f"probe failed: {res.stdout!r}")
        self.assertEqual(len(seen), 1, "probe exited 0 without contacting broker")
        self.assertIsNotNone(seen[0])
        self.assertEqual(seen[0][:2], (18, 0))

    def test_rejects_closed_port(self):
        test_cmd = load_kafka_healthcheck()["test"]
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.bind(("127.0.0.1", 0))
        port = s.getsockname()[1]
        s.close()
        res = run_probe(test_cmd, port)
        self.assertNotEqual(res.returncode, 0, f"closed port succeeded: {res.stdout!r}")

    def test_rejects_non_kafka_http_response(self):
        test_cmd = load_kafka_healthcheck()["test"]

        def handle(conn):
            conn.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")

        with FakeTCPServer(handle) as srv:
            res = run_probe(test_cmd, srv.port)
        self.assertNotEqual(res.returncode, 0, f"HTTP server succeeded: {res.stdout!r}")

    def test_rejects_nonzero_kafka_error_code(self):
        test_cmd = load_kafka_healthcheck()["test"]

        def handle(conn):
            req = read_api_versions_req(conn)
            if req is not None:
                conn.sendall(encode_api_versions_resp(req[2], 35, [(18, 0, 3)]))

        with FakeTCPServer(handle) as srv:
            res = run_probe(test_cmd, srv.port)
        self.assertNotEqual(res.returncode, 0, f"error_code=35 succeeded: {res.stdout!r}")

    def test_rejects_mismatched_correlation_id(self):
        test_cmd = load_kafka_healthcheck()["test"]

        def handle(conn):
            req = read_api_versions_req(conn)
            if req is not None:
                conn.sendall(encode_api_versions_resp(req[2] ^ 1, 0, [(18, 0, 3)]))

        with FakeTCPServer(handle) as srv:
            res = run_probe(test_cmd, srv.port)
        self.assertNotEqual(res.returncode, 0, f"wrong cid succeeded: {res.stdout!r}")

    def test_rejects_empty_api_versions_list(self):
        test_cmd = load_kafka_healthcheck()["test"]

        def handle(conn):
            req = read_api_versions_req(conn)
            if req is not None:
                conn.sendall(encode_api_versions_resp(req[2], 0, []))

        with FakeTCPServer(handle) as srv:
            res = run_probe(test_cmd, srv.port)
        self.assertNotEqual(res.returncode, 0, f"empty api list succeeded: {res.stdout!r}")

    def test_rejects_truncated_response_body(self):
        test_cmd = load_kafka_healthcheck()["test"]

        def handle(conn):
            req = read_api_versions_req(conn)
            if req is not None:
                resp = encode_api_versions_resp(req[2], 0, [(0, 0, 9), (18, 0, 3)])
                conn.sendall(resp[:12])

        with FakeTCPServer(handle) as srv:
            res = run_probe(test_cmd, srv.port)
        self.assertNotEqual(res.returncode, 0, f"truncated body succeeded: {res.stdout!r}")


if __name__ == "__main__":
    unittest.main()
