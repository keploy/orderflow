# OrderFlow — Real-time Order Processing Pipeline

A production-grade Go application demonstrating a real-time e-commerce order processing system with:

- **Producer API** → writes to sharded Postgres + S3 + publishes to Kafka
- **Consumer Service** → reads from Kafka only (no S3/Postgres access by design)
- **Frontend Dashboard** → real-time order placement and monitoring

---

## Architecture

```
Frontend (index.html)
       │
       │  POST /api/orders
       ▼
┌─────────────────┐
│  Producer API   │  (Go HTTP Server, :8080)
│  (main.go)      │
└────┬────┬───────┘
     │    │    │
     │    │    │ 1. INSERT
     │    │    ▼
     │    │  ┌─────────────────┐  ┌─────────────────┐
     │    │  │ Postgres Shard1 │  │ Postgres Shard2 │
     │    │  │ Users A-M :5432 │  │ Users N-Z :5433 │
     │    │  └─────────────────┘  └─────────────────┘
     │    │
     │    │ 2. PUT (receipt .txt)
     │    ▼
     │  ┌──────────────────────┐
     │  │  S3 (LocalStack)     │
     │  │  bucket:order-receipts│
     │  │  port :4566          │
     │  └──────────────────────┘
     │
     │ 3. PUBLISH (order.created event)
     ▼
┌─────────────────┐     CONSUME     ┌─────────────────┐
│  Kafka          │ ──────────────► │  Consumer       │
│  topic: orders  │                 │  (Go Service)   │
│  port: :9092    │                 │  Kafka only     │
└─────────────────┘                 │  ❌ No S3/Postgres│
                                    └─────────────────┘
```

### Sharding Strategy

Orders are sharded by the first letter of `user_id`:
- **Shard 1** (port 5432): Users whose ID starts with **A–M**
- **Shard 2** (port 5433): Users whose ID starts with **N–Z**

---

## Folder Structure

```
orderflow/
├── docker-compose.yml          # All services orchestration
├── frontend/
│   └── index.html              # Dashboard UI
├── producer/                   # Producer API (Go)
│   ├── main.go                 # Entry point, HTTP server
│   ├── Dockerfile
│   ├── go.mod
│   ├── config/
│   │   └── config.go           # ENV-based config
│   ├── models/
│   │   └── order.go            # Order structs + Kafka event
│   ├── handlers/
│   │   └── order.go            # HTTP handlers (create, list)
│   ├── kafka/
│   │   └── producer.go         # Kafka producer client
│   └── storage/
│       ├── postgres.go         # Sharded Postgres (shard1 + shard2)
│       └── s3.go               # S3 receipt upload
├── consumer/                   # Consumer Service (Go)
│   ├── main.go                 # Kafka consumer loop
│   ├── Dockerfile
│   ├── go.mod
│   ├── config/
│   │   └── config.go
│   └── handlers/
│       └── processor.go        # Event processor (Kafka only, no S3/PG)
└── scripts/
    ├── init_shard1.sql         # Postgres schema for shard 1
    ├── init_shard2.sql         # Postgres schema for shard 2
    └── init_localstack.sh      # Creates S3 bucket on startup
```

---

## Prerequisites

- **Docker** and **Docker Compose** installed
- **Go 1.21+** (only for local dev without Docker)
- **librdkafka** (only for local dev — `brew install librdkafka` on Mac)

---

## Running with Docker Compose (Recommended)

### Step 1 — Clone and navigate
```bash
cd orderflow
```

### Step 2 — Make the LocalStack init script executable
```bash
chmod +x scripts/init_localstack.sh
```

### Step 3 — Start all services
```bash
docker-compose up --build
```

Wait ~30 seconds for all services to initialize. You'll see:
```
producer  | ✓ Connected to sharded Postgres (shard1: A-M, shard2: N-Z)
producer  | ✓ S3 client initialized (bucket: order-receipts)
producer  | ✓ Kafka producer connected to kafka:9092
producer  | ✓ Producer API listening on :8080
consumer  | ✓ Subscribed to topic 'orders' with group 'order-consumer-group'
consumer  | ✓ Waiting for messages...
```

### Step 4 — Open the Frontend
Open `frontend/index.html` in your browser (just double-click the file), or:
```bash
open frontend/index.html   # Mac
xdg-open frontend/index.html  # Linux
```

---

## Running Locally (Without Docker)

### Start dependencies only
```bash
docker-compose up zookeeper kafka postgres_shard1 postgres_shard2 localstack
```

### Run Producer
```bash
cd producer
go mod tidy
export PG_SHARD1_DSN="postgres://orderuser:orderpass@localhost:5432/orders_shard1?sslmode=disable"
export PG_SHARD2_DSN="postgres://orderuser:orderpass@localhost:5433/orders_shard2?sslmode=disable"
export S3_ENDPOINT="http://localhost:4566"
export KAFKA_BROKERS="localhost:9092"
CGO_ENABLED=1 go run main.go
```

### Run Consumer (separate terminal)
```bash
cd consumer
go mod tidy
export KAFKA_BROKERS="localhost:9092"
CGO_ENABLED=1 go run main.go
```

---

## API Reference

### Health Check
```bash
curl http://localhost:8080/health
```

### Create Order
```bash
curl -X POST http://localhost:8082/api/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "alice",
    "product_name": "Mechanical Keyboard",
    "quantity": 2,
    "price": 149.99
  }'
```

**Response:**
```json
{
  "order": {
    "id": "uuid-here",
    "user_id": "alice",
    "product_name": "Mechanical Keyboard",
    "quantity": 2,
    "price": 149.99,
    "status": "created"
  },
  "shard": "shard1",
  "shard_info": "shard1 (A-M)",
  "receipt": "receipts/alice/uuid-here.txt"
}
```

### List All Orders
```bash
curl http://localhost:8080/api/orders
```

### List Orders by User
```bash
curl "http://localhost:8080/api/orders?user_id=alice"
```

---

## Test Sharding

```bash
# This goes to Shard 1 (A-M)
curl -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id":"alice","product_name":"Monitor","quantity":1,"price":399.99}'

# This goes to Shard 2 (N-Z)
curl -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id":"zara","product_name":"Keyboard","quantity":2,"price":99.99}'

# Verify directly in Postgres
docker exec -it orderflow-postgres_shard1-1 psql -U orderuser -d orders_shard1 -c "SELECT id, user_id, product_name FROM orders;"
docker exec -it orderflow-postgres_shard2-1 psql -U orderuser -d orders_shard2 -c "SELECT id, user_id, product_name FROM orders;"
```

## Verify S3 Receipts
```bash
aws --endpoint-url=http://localhost:4566 s3 ls s3://order-receipts/receipts/ --recursive
```

## Watch Kafka Events
```bash
docker exec -it orderflow-kafka-1 kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic orders \
  --from-beginning
```

---

## Environment Variables

### Producer
| Variable | Default | Description |
|----------|---------|-------------|
| `KAFKA_BROKERS` | `localhost:9092` | Kafka broker addresses |
| `PG_SHARD1_DSN` | local shard1 | Postgres DSN for shard 1 |
| `PG_SHARD2_DSN` | local shard2 | Postgres DSN for shard 2 |
| `S3_ENDPOINT` | `http://localhost:4566` | S3/LocalStack endpoint |
| `S3_BUCKET` | `order-receipts` | S3 bucket name |
| `PORT` | `8080` | HTTP server port |

### Consumer
| Variable | Default | Description |
|----------|---------|-------------|
| `KAFKA_BROKERS` | `localhost:9092` | Kafka broker addresses |
| `KAFKA_GROUP_ID` | `order-consumer-group` | Consumer group ID |
| `KAFKA_TOPIC` | `orders` | Topic to consume |

---

## Stop Everything
```bash
docker-compose down -v
```
