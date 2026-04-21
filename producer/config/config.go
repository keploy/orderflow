package config

import "os"

type Config struct {
	KafkaBrokers string
	PGShard1DSN  string
	PGShard2DSN  string
	S3Endpoint   string
	S3Bucket     string
	AWSRegion    string
	AWSKeyID     string
	AWSSecretKey string
	Port         string
}

func Load() *Config {
	return &Config{
		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		PGShard1DSN:  getEnv("PG_SHARD1_DSN", "postgres://orderuser:orderpass@localhost:5432/orders_shard1?sslmode=disable"),
		PGShard2DSN:  getEnv("PG_SHARD2_DSN", "postgres://orderuser:orderpass@localhost:5433/orders_shard2?sslmode=disable"),
		S3Endpoint:   getEnv("S3_ENDPOINT", "http://localhost:4566"),
		S3Bucket:     getEnv("S3_BUCKET", "order-receipts"),
		AWSRegion:    getEnv("AWS_REGION", "us-east-1"),
		AWSKeyID:     getEnv("AWS_ACCESS_KEY_ID", "test"),
		AWSSecretKey: getEnv("AWS_SECRET_ACCESS_KEY", "test"),
		Port:         getEnv("PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
