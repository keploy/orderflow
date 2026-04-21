.PHONY: help build-images load-images deploy-dev deploy-prod delete \
        status logs-producer logs-consumer \
        port-forward-api port-forward-kafka port-forward-pg1 port-forward-pg2 \
        test-shard1 test-shard2 test-bulk health list-orders \
        check-pg1 check-pg2 watch-kafka check-s3 \
        minikube-setup rollout-producer rollout-consumer delete-all

NAMESPACE    := orderflow
PRODUCER_IMG := orderflow-producer:latest
CONSUMER_IMG := orderflow-consumer:latest

# ── Mac ARM64 (Apple Silicon) settings ────────────────────────────────────────
# --driver=docker   : required on Apple Silicon — the default hyperkit driver
#                     does not exist on ARM. Docker Desktop provides the VM.
# --platforms       : cross-compile is not needed; we build native arm64 directly.
# minikube image load: preferred over eval $(minikube docker-env) on ARM because
#                     the docker-env approach can silently load amd64 layers.
MINIKUBE_DRIVER  := docker
BUILD_PLATFORM   := linux/arm64

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-28s\033[0m %s\n", $$1, $$2}'

## ─── BUILD ───────────────────────────────────────────────────────────────────

build-images: ## Build arm64 Docker images for producer and consumer
	@echo "Building producer image (linux/arm64)..."
	docker build --platform $(BUILD_PLATFORM) -t $(PRODUCER_IMG) ../producer
	@echo "Building consumer image (linux/arm64)..."
	docker build --platform $(BUILD_PLATFORM) -t $(CONSUMER_IMG) ../consumer
	@echo "✓ Images built for $(BUILD_PLATFORM)"

load-images: ## Load arm64 images into Minikube's internal Docker daemon
	@echo "Loading producer image into Minikube..."
	minikube image load $(PRODUCER_IMG)
	@echo "Loading consumer image into Minikube..."
	minikube image load $(CONSUMER_IMG)
	@echo "✓ Images loaded into Minikube"
	@echo "Verifying images are present in Minikube:"
	minikube image ls | grep orderflow

## ─── DEPLOY ──────────────────────────────────────────────────────────────────

deploy-dev: ## Deploy using dev overlay (1 replica, local images)
	kubectl apply -k overlays/dev
	@echo "✓ Deployed dev overlay to namespace '$(NAMESPACE)'"
	@echo "Run: make port-forward-api"

deploy-prod: ## Deploy using prod overlay (scaled, registry images)
	kubectl apply -k overlays/prod
	@echo "✓ Deployed prod overlay"

deploy-base: ## Deploy base manifests directly (no overlay)
	kubectl apply -k base

delete: ## Remove all orderflow resources (keeps namespace)
	kubectl delete -k base --ignore-not-found
	@echo "✓ All resources deleted"

delete-all: ## Remove everything including namespace and Minikube volumes
	kubectl delete namespace $(NAMESPACE) --ignore-not-found
	@echo "✓ Namespace $(NAMESPACE) deleted"

## ─── MINIKUBE SETUP (Mac ARM64) ──────────────────────────────────────────────

minikube-setup: ## Full first-time setup for Mac ARM64 (Apple Silicon)
	@echo "=== OrderFlow — Minikube Setup for Mac ARM64 ==="
	@echo ""
	@echo "Starting Minikube with Docker driver (required on Apple Silicon)..."
	minikube start \
		--driver=$(MINIKUBE_DRIVER) \
		--cpus=4 \
		--memory=6144 \
		--disk-size=20g
	@echo ""
	@echo "Enabling Ingress addon..."
	minikube addons enable ingress
	@echo ""
	@echo "Building arm64 images..."
	$(MAKE) build-images
	@echo ""
	@echo "Loading images into Minikube..."
	$(MAKE) load-images
	@echo ""
	@echo "Deploying dev overlay..."
	$(MAKE) deploy-dev
	@echo ""
	@echo "============================================"
	@echo "  ✓ Setup complete!"
	@echo "============================================"
	@echo ""
	@echo "Next steps:"
	@echo "  make status            # watch pods come up (~60s)"
	@echo "  make port-forward-api  # forward API to localhost:8080"
	@echo "  make test-shard1       # place a test order"
	@echo "  make watch-kafka       # stream live Kafka events"

minikube-stop: ## Stop Minikube
	minikube stop

minikube-delete: ## Delete Minikube cluster entirely (wipes all data)
	minikube delete

## ─── STATUS ──────────────────────────────────────────────────────────────────

status: ## Show all pod, service and statefulset status
	@echo "\n=== PODS ===\n"
	kubectl get pods -n $(NAMESPACE) -o wide
	@echo "\n=== SERVICES ===\n"
	kubectl get svc -n $(NAMESPACE)
	@echo "\n=== STATEFULSETS ===\n"
	kubectl get statefulsets -n $(NAMESPACE)
	@echo "\n=== DEPLOYMENTS ===\n"
	kubectl get deployments -n $(NAMESPACE)
	@echo "\n=== HPA ===\n"
	kubectl get hpa -n $(NAMESPACE)

## ─── PORT FORWARDING ─────────────────────────────────────────────────────────

port-forward-api: ## Forward producer API → localhost:8080
	@echo "Forwarding producer → localhost:8080 (Ctrl+C to stop)"
	kubectl port-forward -n $(NAMESPACE) svc/producer 8082:8080

port-forward-kafka: ## Forward Kafka → localhost:9092
	@echo "Forwarding Kafka → localhost:9092 (Ctrl+C to stop)"
	kubectl port-forward -n $(NAMESPACE) svc/kafka-external 9092:9092

port-forward-pg1: ## Forward Postgres Shard 1 → localhost:5432
	@echo "Forwarding Postgres Shard 1 → localhost:5432 (Ctrl+C to stop)"
	kubectl port-forward -n $(NAMESPACE) svc/postgres-shard1 5432:5432

port-forward-pg2: ## Forward Postgres Shard 2 → localhost:5433
	@echo "Forwarding Postgres Shard 2 → localhost:5433 (Ctrl+C to stop)"
	kubectl port-forward -n $(NAMESPACE) svc/postgres-shard2 5433:5432

port-forward-localstack: ## Forward LocalStack S3 → localhost:4566
	@echo "Forwarding LocalStack → localhost:4566 (Ctrl+C to stop)"
	kubectl port-forward -n $(NAMESPACE) svc/localstack 4566:4566

## ─── LOGS ────────────────────────────────────────────────────────────────────

logs-producer: ## Stream producer logs
	kubectl logs -n $(NAMESPACE) -l app=producer -f --tail=100

logs-consumer: ## Stream consumer logs
	kubectl logs -n $(NAMESPACE) -l app=consumer -f --tail=100

logs-kafka: ## Stream Kafka logs
	kubectl logs -n $(NAMESPACE) -l app=kafka -f --tail=50

logs-pg1: ## Stream Postgres Shard 1 logs
	kubectl logs -n $(NAMESPACE) -l app=postgres-shard1 -f --tail=50

## ─── TESTING ─────────────────────────────────────────────────────────────────

health: ## API health check (requires port-forward-api in another terminal)
	curl -s http://localhost:8080/health | jq .

list-orders: ## List all orders (requires port-forward-api)
	curl -s http://localhost:8080/api/orders | jq .

test-shard1: ## Place order for 'alice' → Postgres Shard 1 (A-M)
	curl -s -X POST http://localhost:8080/api/orders \
		-H "Content-Type: application/json" \
		-d '{"user_id":"alice","product_name":"Mechanical Keyboard","quantity":2,"price":149.99}' | jq .

test-shard2: ## Place order for 'zara' → Postgres Shard 2 (N-Z)
	curl -s -X POST http://localhost:8080/api/orders \
		-H "Content-Type: application/json" \
		-d '{"user_id":"zara","product_name":"4K Monitor","quantity":1,"price":399.99}' | jq .

test-bulk: ## Place 5 orders across both shards
	@echo "--- alice (shard1) ---"
	curl -s -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" \
		-d '{"user_id":"alice","product_name":"Keyboard","quantity":1,"price":99.99}' | jq .shard
	@echo "--- bob (shard1) ---"
	curl -s -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" \
		-d '{"user_id":"bob","product_name":"Mouse","quantity":2,"price":49.99}' | jq .shard
	@echo "--- nancy (shard2) ---"
	curl -s -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" \
		-d '{"user_id":"nancy","product_name":"Monitor","quantity":1,"price":299.99}' | jq .shard
	@echo "--- zara (shard2) ---"
	curl -s -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" \
		-d '{"user_id":"zara","product_name":"Webcam","quantity":3,"price":79.99}' | jq .shard
	@echo "--- omar (shard2) ---"
	curl -s -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" \
		-d '{"user_id":"omar","product_name":"Desk Mat","quantity":1,"price":39.99}' | jq .shard

## ─── DATA INSPECTION ─────────────────────────────────────────────────────────

check-pg1: ## Query Postgres Shard 1 directly (users A-M)
	kubectl exec -it -n $(NAMESPACE) postgres-shard1-0 -- \
		psql -U orderuser -d orders_shard1 \
		-c "SELECT id, user_id, product_name, quantity, price, status, created_at FROM orders ORDER BY created_at DESC LIMIT 10;"

check-pg2: ## Query Postgres Shard 2 directly (users N-Z)
	kubectl exec -it -n $(NAMESPACE) postgres-shard2-0 -- \
		psql -U orderuser -d orders_shard2 \
		-c "SELECT id, user_id, product_name, quantity, price, status, created_at FROM orders ORDER BY created_at DESC LIMIT 10;"

watch-kafka: ## Watch Kafka topic live from inside the cluster
	kubectl exec -it -n $(NAMESPACE) kafka-0 -- \
		kafka-console-consumer \
		--bootstrap-server kafka:29092 \
		--topic orders \
		--from-beginning \
		--property print.key=true \
		--property key.separator="→ "

check-s3: ## List S3 receipts in LocalStack
	kubectl exec -it -n $(NAMESPACE) \
		$(shell kubectl get pod -n $(NAMESPACE) -l app=localstack -o jsonpath='{.items[0].metadata.name}') -- \
		awslocal s3 ls s3://order-receipts/receipts/ --recursive

## ─── ROLLOUTS ────────────────────────────────────────────────────────────────

rollout-producer: ## Rebuild arm64 image and rolling-restart producer
	$(MAKE) build-images
	$(MAKE) load-images
	kubectl rollout restart deployment/producer -n $(NAMESPACE)
	kubectl rollout status deployment/producer -n $(NAMESPACE)

rollout-consumer: ## Rebuild arm64 image and rolling-restart consumer
	$(MAKE) build-images
	$(MAKE) load-images
	kubectl rollout restart deployment/consumer -n $(NAMESPACE)
	kubectl rollout status deployment/consumer -n $(NAMESPACE)

## ─── ISTIO ───────────────────────────────────────────────────────────────────

istio-install: ## Install Istio on Minikube (Mac ARM64) — run once before deploy-dev
	@echo "Installing Istio for Mac ARM64..."
	chmod +x base/istio/install-istio.sh
	bash base/istio/install-istio.sh

istio-status: ## Check Istio control plane and sidecar injection status
	@echo "\n=== ISTIO CONTROL PLANE ===\n"
	kubectl get pods -n istio-system
	@echo "\n=== SIDECAR INJECTION (look for 2/2 READY — app + envoy) ===\n"
	kubectl get pods -n $(NAMESPACE) -o wide
	@echo "\n=== ISTIO DESTINATION RULES ===\n"
	kubectl get destinationrules -n $(NAMESPACE)
	@echo "\n=== ISTIO VIRTUAL SERVICES ===\n"
	kubectl get virtualservices -n $(NAMESPACE)
	@echo "\n=== PEER AUTHENTICATION (mTLS) ===\n"
	kubectl get peerauthentication -n $(NAMESPACE)

istio-verify-mtls: ## Verify mTLS is working between all pods
	@echo "Checking mTLS status for all services..."
	istioctl x describe service producer -n $(NAMESPACE)
	@echo ""
	istioctl x describe service kafka -n $(NAMESPACE)

istio-verify-circuit-breaker: ## Show circuit breaker config on Envoy sidecar
	@echo "Checking circuit breaker config on producer's Envoy sidecar..."
	istioctl proxy-config cluster \
		$(shell kubectl get pod -n $(NAMESPACE) -l app=producer -o jsonpath='{.items[0].metadata.name}') \
		-n $(NAMESPACE) \
		--fqdn postgres-shard1.orderflow.svc.cluster.local

kiali: ## Open Kiali service mesh dashboard in browser
	@echo "Starting Kiali dashboard..."
	istioctl dashboard kiali

kiali-bg: ## Start Kiali in background (port 20001)
	@echo "Forwarding Kiali → http://localhost:20001 (Ctrl+C to stop)"
	kubectl port-forward -n istio-system svc/kiali 20001:20001

test-circuit-breaker: ## Simulate circuit breaker — kill postgres-shard1 and watch fast-fails
	@echo "=== Circuit Breaker Demo ==="
	@echo ""
	@echo "Step 1: Place a healthy order (should succeed)"
	curl -s -X POST http://localhost:8080/api/orders \
		-H "Content-Type: application/json" \
		-d '{"user_id":"alice","product_name":"Test Item","quantity":1,"price":9.99}' | jq .status
	@echo ""
	@echo "Step 2: Scale down postgres-shard1 to 0 (simulate failure)"
	kubectl scale statefulset postgres-shard1 --replicas=0 -n $(NAMESPACE)
	@echo "Waiting 5s for circuit breaker to detect failures..."
	sleep 5
	@echo ""
	@echo "Step 3: Place order for alice (A-M → shard1 → circuit should trip)"
	curl -s -X POST http://localhost:8080/api/orders \
		-H "Content-Type: application/json" \
		-d '{"user_id":"alice","product_name":"Test Item","quantity":1,"price":9.99}' | jq .
	@echo ""
	@echo "Step 4: Place order for zara (N-Z → shard2 → should STILL SUCCEED)"
	curl -s -X POST http://localhost:8080/api/orders \
		-H "Content-Type: application/json" \
		-d '{"user_id":"zara","product_name":"Test Item","quantity":1,"price":9.99}' | jq .status
	@echo ""
	@echo "Step 5: Restore shard1"
	kubectl scale statefulset postgres-shard1 --replicas=1 -n $(NAMESPACE)
	@echo "✓ Done. Shard1 is recovering."

istio-uninstall: ## Completely remove Istio from the cluster
	istioctl uninstall --purge -y
	kubectl delete namespace istio-system --ignore-not-found
	@echo "✓ Istio removed"
