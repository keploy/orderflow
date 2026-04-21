.PHONY: up down build logs producer consumer clean test-order test-shard1 test-shard2 check-s3 check-kafka check-pg1 check-pg2

## Start all services
up:
	chmod +x scripts/init_localstack.sh
	docker-compose up --build

## Start in detached mode
up-d:
	chmod +x scripts/init_localstack.sh
	docker-compose up --build -d

## Stop all services
down:
	docker-compose down

## Stop and wipe volumes
clean:
	docker-compose down -v

## View logs
logs:
	docker-compose logs -f

## View only producer logs
logs-producer:
	docker-compose logs -f producer

## View only consumer logs
logs-consumer:
	docker-compose logs -f consumer

## Build images only
build:
	docker-compose build

## Health check
health:
	curl -s http://localhost:8080/health | jq .

## List all orders
list-orders:
	curl -s http://localhost:8080/api/orders | jq .

## Place a test order (Shard 1 - user starts with A)
test-shard1:
	curl -s -X POST http://localhost:8080/api/orders \
		-H "Content-Type: application/json" \
		-d '{"user_id":"alice","product_name":"Mechanical Keyboard","quantity":2,"price":149.99}' | jq .

## Place a test order (Shard 2 - user starts with Z)
test-shard2:
	curl -s -X POST http://localhost:8080/api/orders \
		-H "Content-Type: application/json" \
		-d '{"user_id":"zara","product_name":"4K Monitor","quantity":1,"price":399.99}' | jq .

## Place 5 test orders across both shards
test-bulk:
	@echo "--- Placing orders across both shards ---"
	curl -s -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" \
		-d '{"user_id":"alice","product_name":"Keyboard","quantity":1,"price":99.99}' | jq .shard
	curl -s -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" \
		-d '{"user_id":"bob","product_name":"Mouse","quantity":2,"price":49.99}' | jq .shard
	curl -s -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" \
		-d '{"user_id":"nancy","product_name":"Monitor","quantity":1,"price":299.99}' | jq .shard
	curl -s -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" \
		-d '{"user_id":"zara","product_name":"Webcam","quantity":3,"price":79.99}' | jq .shard
	curl -s -X POST http://localhost:8080/api/orders -H "Content-Type: application/json" \
		-d '{"user_id":"omar","product_name":"Desk Mat","quantity":1,"price":39.99}' | jq .shard

## Check Postgres Shard 1
check-pg1:
	docker exec -it orderflow-postgres_shard1-1 psql -U orderuser -d orders_shard1 \
		-c "SELECT id, user_id, product_name, quantity, price, status, created_at FROM orders ORDER BY created_at DESC LIMIT 10;"

## Check Postgres Shard 2
check-pg2:
	docker exec -it orderflow-postgres_shard2-1 psql -U orderuser -d orders_shard2 \
		-c "SELECT id, user_id, product_name, quantity, price, status, created_at FROM orders ORDER BY created_at DESC LIMIT 10;"

## Check S3 receipts
check-s3:
	aws --endpoint-url=http://localhost:4566 \
		--region us-east-1 \
		--no-sign-request \
		s3 ls s3://order-receipts/receipts/ --recursive

## Watch Kafka topic live
watch-kafka:
	docker exec orderflow-kafka-1 kafka-console-consumer \
		--bootstrap-server localhost:9092 \
		--topic orders \
		--from-beginning \
		--property print.key=true \
		--property key.separator="→ "
