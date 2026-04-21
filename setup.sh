#!/bin/bash
set -e

echo "============================================"
echo "  OrderFlow Setup Script"
echo "============================================"
echo ""

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker not found. Please install Docker Desktop."
    exit 1
fi
echo "✓ Docker found: $(docker --version)"

# Check Docker Compose
if ! command -v docker-compose &> /dev/null; then
    echo "❌ docker-compose not found."
    exit 1
fi
echo "✓ Docker Compose found: $(docker-compose --version)"

# Make init script executable
chmod +x scripts/init_localstack.sh
echo "✓ LocalStack init script made executable"

echo ""
echo "Starting all services..."
echo "(This may take 2-3 minutes on first run to pull images)"
echo ""

docker-compose up --build -d

echo ""
echo "Waiting for services to be healthy..."
sleep 10

# Wait for producer to be ready
MAX_TRIES=20
for i in $(seq 1 $MAX_TRIES); do
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo "✓ Producer API is up!"
        break
    fi
    echo "  Waiting for producer... ($i/$MAX_TRIES)"
    sleep 3
done

echo ""
echo "============================================"
echo "  All services running!"
echo "============================================"
echo ""
echo "  📊 Frontend:    Open frontend/index.html in your browser"
echo "  🔌 API:         http://localhost:8080"
echo "  🐘 PG Shard 1:  localhost:5432 (users A-M)"
echo "  🐘 PG Shard 2:  localhost:5433 (users N-Z)"
echo "  🪣 S3:          http://localhost:4566"
echo "  📨 Kafka:       localhost:9092"
echo ""
echo "Quick test:"
echo "  make test-shard1   # Order for 'alice' → shard1"
echo "  make test-shard2   # Order for 'zara'  → shard2"
echo "  make logs-consumer # Watch consumer process events"
echo ""
echo "Stop everything:  make down"
echo "Wipe all data:    make clean"
