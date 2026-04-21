# OrderFlow — Kubernetes Deployment

Complete Kubernetes manifests for deploying the OrderFlow order processing pipeline.

## Folder Structure

```
orderflow-k8s/
├── Makefile                          ← All k8s operations
├── README.md                         ← This file
│
├── base/                             ← Base manifests (environment-agnostic)
│   ├── kustomization.yaml
│   ├── namespace.yaml
│   ├── configmap.yaml               ← Shared env config (Kafka, DB DSNs, S3)
│   ├── secrets.yaml                 ← Credentials (Postgres, AWS keys)
│   ├── ingress.yaml                 ← nginx Ingress for producer API
│   ├── network-policy.yaml          ← Enforces consumer ≠ S3/Postgres at network layer
│   ├── zookeeper/
│   │   └── statefulset.yaml         ← Zookeeper StatefulSet + headless Service
│   ├── kafka/
│   │   └── statefulset.yaml         ← Kafka StatefulSet + internal/external Services
│   ├── postgres/
│   │   ├── shard1.yaml              ← PG Shard 1 (users A-M) + init schema
│   │   └── shard2.yaml              ← PG Shard 2 (users N-Z) + init schema
│   ├── localstack/
│   │   └── deployment.yaml          ← LocalStack S3 + bucket init
│   ├── producer/
│   │   └── deployment.yaml          ← Producer API + Service + HPA
│   └── consumer/
│       └── deployment.yaml          ← Consumer (Kafka-only, no S3/PG env vars)
│
└── overlays/
    ├── dev/
    │   └── kustomization.yaml       ← Dev: 1 replica, local images, NodePort
    └── prod/
        └── kustomization.yaml       ← Prod: 3 replicas, registry images, larger limits
```

---

## Architecture in Kubernetes

```
                 ┌──────────────────────────────────────────────┐
                 │           orderflow (namespace)               │
                 │                                              │
  Internet ─────►│  Ingress (nginx)                             │
                 │    └──► producer (Deployment, 2 replicas)    │
                 │           ├──► kafka (StatefulSet)           │
                 │           ├──► postgres-shard1 (StatefulSet) │
                 │           ├──► postgres-shard2 (StatefulSet) │
                 │           └──► localstack (Deployment)       │
                 │                    ↑                         │
                 │           consumer (Deployment) ─────────────┘
                 │             ONLY talks to kafka
                 │             NetworkPolicy BLOCKS S3 + Postgres
                 └──────────────────────────────────────────────┘
```

### Key Design Decisions

| Component | Kind | Why |
|-----------|------|-----|
| Zookeeper, Kafka, Postgres | StatefulSet | Need stable network identity and persistent storage |
| Producer, Consumer, LocalStack | Deployment | Stateless — can be recreated freely |
| Consumer isolation | NetworkPolicy | Enforces "no S3/Postgres" at the kernel level, not just config |
| Kafka listeners | PLAINTEXT:29092 (internal) / PLAINTEXT_HOST:9092 (external) | Separates in-cluster from out-of-cluster traffic |
| InitContainers | `wait-for-deps` on producer, `wait-for-kafka` on consumer | Prevents startup race conditions |

---

## Quick Start — Minikube (Local Dev)

### Prerequisites
```bash
# Install tools
brew install minikube kubectl kustomize jq

# Or on Linux
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
sudo install minikube-linux-amd64 /usr/local/bin/minikube
```

### One-command setup
```bash
cd orderflow-k8s
make minikube-setup
```

This will:
1. Start Minikube with 4 CPUs and 6GB RAM
2. Enable the Ingress addon
3. Build Docker images from source
4. Load images into Minikube's Docker daemon
5. Deploy the dev overlay

### Manual step-by-step

```bash
# 1. Start Minikube
minikube start --cpus=4 --memory=6144

# 2. Enable ingress addon
minikube addons enable ingress

# 3. Build images (from the root of your project, not k8s folder)
docker build -t orderflow-producer:latest ../producer
docker build -t orderflow-consumer:latest ../consumer

# 4. Load images into Minikube
minikube image load orderflow-producer:latest
minikube image load orderflow-consumer:latest

# 5. Deploy
kubectl apply -k overlays/dev

# 6. Watch pods come up
kubectl get pods -n orderflow -w
```

### Access the API

```bash
# Forward the API port (keep this running in a terminal)
make port-forward-api

# In another terminal, test it
make health
make test-shard1
make test-shard2
make test-bulk
```

---

## Verify Everything Works

### Check all pods are Running
```bash
make status
# All pods should show STATUS=Running and READY=1/1 (or 2/2 etc.)
```

### Test sharding
```bash
# Both of these should succeed and show different shard values
make test-shard1   # user 'alice' → shard1 (A-M)
make test-shard2   # user 'zara'  → shard2 (N-Z)
```

### Inspect Postgres directly
```bash
make check-pg1  # Query shard1 pod directly via kubectl exec
make check-pg2  # Query shard2 pod directly via kubectl exec
```

### Watch Kafka events live
```bash
make watch-kafka   # Streams from inside the kafka-0 pod
```

### Verify S3 receipts
```bash
make check-s3   # Lists receipts in LocalStack bucket
```

### Confirm consumer isolation (NetworkPolicy)
```bash
# This should FAIL — consumer cannot reach Postgres
kubectl exec -it -n orderflow \
  $(kubectl get pod -n orderflow -l app=consumer -o jsonpath='{.items[0].metadata.name}') \
  -- nc -zv postgres-shard1 5432
# Expected: command terminated with exit code 1

# This should SUCCEED — consumer can reach Kafka
kubectl exec -it -n orderflow \
  $(kubectl get pod -n orderflow -l app=consumer -o jsonpath='{.items[0].metadata.name}') \
  -- nc -zv kafka 29092
# Expected: open
```

---

## Prod Deployment (Cloud — GKE / EKS / AKS)

### 1. Push images to a registry
```bash
docker tag orderflow-producer:latest your-registry.io/orderflow/producer:1.0.0
docker tag orderflow-consumer:latest your-registry.io/orderflow/consumer:1.0.0
docker push your-registry.io/orderflow/producer:1.0.0
docker push your-registry.io/orderflow/consumer:1.0.0
```

### 2. Update the prod overlay
Edit `overlays/prod/kustomization.yaml`:
```yaml
images:
  - name: orderflow-producer
    newName: your-registry.io/orderflow/producer   # ← your actual registry
    newTag: "1.0.0"
  - name: orderflow-consumer
    newName: your-registry.io/orderflow/consumer   # ← your actual registry
    newTag: "1.0.0"
```

### 3. Update secrets for prod
```bash
# Generate base64 values for real prod passwords
echo -n "your-real-password" | base64

# Edit base/secrets.yaml with real values, or use External Secrets / Vault
```

### 4. Deploy
```bash
kubectl apply -k overlays/prod
make status
```

### 5. Configure Ingress DNS
In prod, point your domain's DNS A record to the LoadBalancer IP, and update `base/ingress.yaml`:
```yaml
rules:
  - host: api.yourcompany.com   # ← real domain
```

---

## Common Commands

```bash
make status              # Overview of all pods/services/HPAs
make logs-producer       # Stream producer logs
make logs-consumer       # Stream consumer logs
make rollout-producer    # Trigger rolling restart of producer
make rollout-consumer    # Trigger rolling restart of consumer
make delete              # Remove all resources (keeps namespace)
make delete-all          # Remove everything including namespace
```

---

## Troubleshooting

### Pod stuck in `Pending`
```bash
kubectl describe pod <pod-name> -n orderflow
# Usually: insufficient CPU/memory → increase minikube resources
minikube start --cpus=4 --memory=8192
```

### Producer `CrashLoopBackOff`
```bash
kubectl logs -n orderflow -l app=producer --previous
# Check initContainer logs:
kubectl logs -n orderflow <producer-pod> -c wait-for-deps
```

### Kafka connection refused
```bash
# Verify Kafka pod is up and listener is on 29092
kubectl exec -it -n orderflow kafka-0 -- kafka-broker-api-versions --bootstrap-server localhost:29092
```

### Image pull errors (local dev)
```bash
# Re-load images after rebuilding
make build-images
make load-images
make rollout-producer
make rollout-consumer
```

### Reset everything
```bash
make delete-all
make minikube-setup
```
