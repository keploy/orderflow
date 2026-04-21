package main

import (
	"log"
	"net/http"
	"time"

	"orderflow/producer/config"
	"orderflow/producer/handlers"
	"orderflow/producer/kafka"
	"orderflow/producer/storage"
)

func main() {

	cfg := config.Load()
	log.Println("Starting OrderFlow Producer API...")

	// Retry DB connections (services may start slow)
	var db *storage.ShardedDB
	var err error
	for i := 0; i < 10; i++ {
		db, err = storage.NewShardedDB(cfg.PGShard1DSN, cfg.PGShard2DSN)
		if err == nil {
			break
		}
		log.Printf("DB not ready, retrying in 3s... (%v)", err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to sharded DB: %v", err)
	}
	defer db.Close()
	log.Println("✓ Connected to sharded Postgres (shard1: A-M, shard2: N-Z)")

	s3Store, err := storage.NewS3Store(cfg.S3Endpoint, cfg.S3Bucket, cfg.AWSRegion, cfg.AWSKeyID, cfg.AWSSecretKey)
	if err != nil {
		log.Fatalf("Failed to init S3: %v", err)
	}
	log.Println("✓ S3 client initialized (bucket:", cfg.S3Bucket, ")")

	var producer *kafka.Producer
	for i := 0; i < 10; i++ {
		producer, err = kafka.NewProducer(cfg.KafkaBrokers)
		if err == nil {
			break
		}
		log.Printf("Kafka not ready, retrying in 3s... (%v)", err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to create Kafka producer: %v", err)
	}
	defer producer.Close()
	log.Println("✓ Kafka producer connected to", cfg.KafkaBrokers)

	handler := handlers.NewOrderHandler(db, s3Store, producer)

	mux := http.NewServeMux()
	// CORS middleware
	withCORS := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			h(w, r)
		}
	}

	mux.HandleFunc("/health", withCORS(handler.Health))
	mux.HandleFunc("/api/orders", withCORS(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handler.CreateOrder(w, r)
		case http.MethodGet:
			handler.GetOrders(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	log.Printf("✓ Producer API listening on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))

}
