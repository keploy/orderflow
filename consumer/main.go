package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"orderflow/consumer/config"
	"orderflow/consumer/handlers"
)

func main() {
	cfg := config.Load()
	processor := handlers.NewNotificationService()

	log.Println("Starting OrderFlow Consumer...")
	log.Println("⚠️  Consumer policy: NO access to S3 or Postgres - Kafka events only")

	var consumer *kafka.Consumer
	var err error
	for i := 0; i < 15; i++ {
		consumer, err = kafka.NewConsumer(&kafka.ConfigMap{
			"bootstrap.servers":  cfg.KafkaBrokers,
			"group.id":           cfg.GroupID,
			"auto.offset.reset":  "earliest",
			"enable.auto.commit": true,
		})
		if err == nil {
			break
		}
		log.Printf("Kafka not ready, retrying... (%v)", err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	defer consumer.Close()

	if err := consumer.Subscribe(cfg.Topic, nil); err != nil {
		log.Fatalf("Subscribe error: %v", err)
	}
	log.Printf("✓ Subscribed to topic '%s' with group '%s'", cfg.Topic, cfg.GroupID)
	log.Println("✓ Waiting for messages...")

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	run := true
	for run {
		select {
		case sig := <-sigchan:
			log.Printf("Caught signal %v: terminating", sig)
			run = false
		default:
			ev := consumer.Poll(100)
			if ev == nil {
				continue
			}
			switch e := ev.(type) {
			case *kafka.Message:
				log.Printf("[CONSUMER] Received message from partition %d, offset %v",
					e.TopicPartition.Partition, e.TopicPartition.Offset)
				if err := processor.Process(e.Key, e.Value); err != nil {
					log.Printf("[CONSUMER] Error processing message: %v", err)
				}
			case kafka.Error:
				log.Printf("[CONSUMER] Kafka error: %v", e)
			}
		}
	}
}
