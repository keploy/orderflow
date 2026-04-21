# Istio + Envoy in OrderFlow

## What is Envoy?

Envoy is a high-performance **L4/L7 proxy** written in C++. It runs as a
sidecar container injected automatically into every pod in the mesh. Your Go
application container never talks directly to another service — all traffic
goes through Envoy first.

```
┌─────────────────────────────────────────────┐
│  Producer Pod                               │
│                                             │
│  ┌──────────────┐    ┌──────────────────┐   │
│  │  Go App      │───►│  Envoy Sidecar   │───┼──► (network)
│  │  :8080       │◄───│  :15001 (egress) │   │
│  └──────────────┘    │  :15006 (ingress)│   │
│                      └──────────────────┘   │
└─────────────────────────────────────────────┘
```

Your app thinks it's making a direct call to `postgres-shard1:5432`.
In reality iptables rules redirect that TCP connection through Envoy,
which applies policies before forwarding.

## What is Istio?

Istio is a **service mesh control plane** that manages all the Envoy proxies
across your cluster. It has two planes:

- **Control Plane** (`istiod`): pushes routing rules, certs, and policies to
  every Envoy sidecar via xDS API
- **Data Plane**: all the Envoy sidecars — they enforce what istiod tells them

## General purposes of a service mesh

| Purpose              | What it does                                                   |
|----------------------|----------------------------------------------------------------|
| **mTLS**             | Encrypts and authenticates ALL pod-to-pod traffic automatically|
| **Traffic management** | Retries, timeouts, circuit breaking, canary deployments     |
| **Observability**    | Automatic metrics, traces, and access logs for every call      |
| **Authorization**    | Fine-grained "which service can call which" policies           |
| **Load balancing**   | Round robin, least-conn, consistent hash — configurable per service |
| **Fault injection**  | Inject delays/errors to test resilience (chaos engineering)    |

## What we implement in OrderFlow

We implement **two Istio features**:

### 1. Circuit Breaker (DestinationRule) on Postgres shards

```
Producer → [Envoy sidecar] → postgres-shard1
                │
           Circuit Breaker
           - trips after 5 consecutive 5xx errors
           - fast-fails for 30 seconds
           - prevents thread exhaustion in the producer
           - auto-recovers when shard comes back
```

Without this: one slow/dead Postgres shard causes the producer to queue
requests until it runs out of goroutines and crashes the whole service.

With this: Envoy trips the breaker, returns an immediate error, producer
handles it gracefully, other requests keep flowing.

### 2. Retry Policy (VirtualService) on Producer HTTP

```
Frontend → [Envoy ingress sidecar] → Producer
                    │
               Retry Policy
               - retries on 503/504 (transient failures)
               - max 3 attempts, 2s per-attempt timeout
               - 25ms between retries
               - only on GET (safe to retry, POST is excluded)
```

### 3. Mutual TLS (PeerAuthentication)

All traffic inside the `orderflow` namespace is automatically encrypted and
mutually authenticated. No certificates to manage manually — istiod handles
rotation.

```
producer ←──[mTLS]──► consumer
producer ←──[mTLS]──► postgres-shard1
producer ←──[mTLS]──► kafka
```

Note: Kafka and Postgres sidecars proxy the TCP layer (L4), not HTTP (L7),
so circuit breaking on those uses outlier detection (connection-level), not
HTTP retry policies.

```
# 1. Install Istio (once)
make istio-install

# 2. Deploy the app (sidecars auto-inject)
make deploy-dev

# 3. Verify every pod shows 2/2 READY (app + envoy)
make istio-status

# 4. Demo the circuit breaker live
make test-circuit-breaker

# 5. Open Kiali to see the mesh visually
make kiali
```
