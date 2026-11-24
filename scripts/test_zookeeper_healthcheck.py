#!/usr/bin/env python3
"""Unit tests pinning docker-compose.yml's ZooKeeper srvr readiness probe."""

import os
import pathlib
import socket
import subprocess
import threading
import unittest

import yaml

COMPOSE_PATH = pathlib.Path(__file__).resolve().parent.parent / "docker-compose.yml"


def load_zookeeper_service():
    with COMPOSE_PATH.open("r", encoding="utf-8") as fh:
        doc = yaml.safe_load(fh)
    return doc["services"]["zookeeper"]


def parse_duration_seconds(val):
    if isinstance(val, (int, float)):
        return int(val)
    s = str(val).strip()
    if s.endswith("s"):
        return int(s[:-1])
    if s.endswith("m"):
        return int(s[:-1]) * 60
    raise AssertionError(f"unexpected duration format: {val!r}")


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
    env["ZOOKEEPER_PROBE_HOST"] = "127.0.0.1"
    env["ZOOKEEPER_PROBE_PORT"] = str(port)
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


class ZookeeperHealthcheckTest(unittest.TestCase):
    def test_uses_srvr_wire_probe_with_ci_startup_headroom_and_bounded_heap(self):
        svc = load_zookeeper_service()
        env = svc.get("environment") or {}
        heap = str(env.get("KAFKA_HEAP_OPTS", ""))
        self.assertIn("-Xmx256m", heap)

        hc = svc["healthcheck"]
        test_cmd = hc.get("test")
        self.assertIsInstance(test_cmd, list)
        self.assertGreaterEqual(len(test_cmd), 2)
        self.assertEqual(test_cmd[0], "CMD")
        joined = " ".join(test_cmd)
        self.assertNotIn("nc ", joined)
        self.assertNotIn("localhost", joined)
        self.assertIn("srvr", joined)
        self.assertIn("Mode: ", joined)

        self.assertGreaterEqual(parse_duration_seconds(hc.get("start_period", "0s")), 30)
        self.assertEqual(parse_duration_seconds(hc.get("interval", "0s")), 10)
        self.assertGreaterEqual(parse_duration_seconds(hc.get("timeout", "0s")), 10)
        self.assertGreaterEqual(int(hc.get("retries", 0)), 15)

    def test_accepts_valid_srvr_standalone_response(self):
        test_cmd = load_zookeeper_service()["healthcheck"]["test"]
        seen = []

        def handle(conn):
            req = conn.recv(16)
            seen.append(req)
            if req == b"srvr":
                conn.sendall(
                    b"Zookeeper version: 3.6.3--6401e4ad2087061bc6b9f80dec2d69f2e3c8660a\n"
                    b"Latency min/avg/max: 0/0.0/0\n"
                    b"Received: 1\n"
                    b"Sent: 0\n"
                    b"Connections: 1\n"
                    b"Outstanding: 0\n"
                    b"Zxid: 0x0\n"
                    b"Mode: standalone\n"
                    b"Node count: 5\n"
                )

        with FakeTCPServer(handle) as srv:
            res = run_probe(test_cmd, srv.port)
        self.assertEqual(res.returncode, 0, f"probe failed: {res.stdout!r}")
        self.assertEqual(seen, [b"srvr"])

    def test_rejects_not_serving_response(self):
        test_cmd = load_zookeeper_service()["healthcheck"]["test"]

        def handle(conn):
            _ = conn.recv(16)
            conn.sendall(b"This ZooKeeper instance is not currently serving requests\n")

        with FakeTCPServer(handle) as srv:
            res = run_probe(test_cmd, srv.port)
        self.assertNotEqual(res.returncode, 0, f"not-serving response succeeded: {res.stdout!r}")

    def test_rejects_closed_port(self):
        test_cmd = load_zookeeper_service()["healthcheck"]["test"]
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.bind(("127.0.0.1", 0))
        port = s.getsockname()[1]
        s.close()
        res = run_probe(test_cmd, port)
        self.assertNotEqual(res.returncode, 0, f"closed port succeeded: {res.stdout!r}")

    def test_rejects_disabled_4lw_response(self):
        test_cmd = load_zookeeper_service()["healthcheck"]["test"]

        def handle(conn):
            _ = conn.recv(16)
            conn.sendall(b"srvr is not executed because it is not in the whitelist.\n")

        with FakeTCPServer(handle) as srv:
            res = run_probe(test_cmd, srv.port)
        self.assertNotEqual(res.returncode, 0, f"whitelist error succeeded: {res.stdout!r}")

    def test_rejects_http_non_zookeeper_response(self):
        test_cmd = load_zookeeper_service()["healthcheck"]["test"]

        def handle(conn):
            _ = conn.recv(16)
            conn.sendall(b"HTTP/1.1 200 OK\r\n\r\n")

        with FakeTCPServer(handle) as srv:
            res = run_probe(test_cmd, srv.port)
        self.assertNotEqual(res.returncode, 0, f"HTTP server succeeded: {res.stdout!r}")

    def test_rejects_empty_eof_response(self):
        test_cmd = load_zookeeper_service()["healthcheck"]["test"]

        def handle(conn):
            _ = conn.recv(16)

        with FakeTCPServer(handle) as srv:
            res = run_probe(test_cmd, srv.port)
        self.assertNotEqual(res.returncode, 0, f"empty EOF succeeded: {res.stdout!r}")


if __name__ == "__main__":
    unittest.main()
