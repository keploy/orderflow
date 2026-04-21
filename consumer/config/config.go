package config

import "os"

type Config struct {
	KafkaBrokers string
	GroupID      string
	Topic        string
}

func Load() *Config {
	return &Config{
		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		GroupID:      getEnv("KAFKA_GROUP_ID", "order-consumer-group"),
		Topic:        getEnv("KAFKA_TOPIC", "orders"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
