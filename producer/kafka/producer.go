package kafka

import (
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"orderflow/producer/models"
)

type Producer struct {
	producer *kafka.Producer
	topic    string
}

func NewProducer(brokers string) (*Producer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"acks":              "all",
		"retries":           3,
	})
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}

	// Start delivery report handler
	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					fmt.Printf("Kafka delivery failed: %v\n", ev.TopicPartition.Error)
				} else {
					fmt.Printf("Kafka delivered to %v [partition %d] offset %v\n",
						*ev.TopicPartition.Topic, ev.TopicPartition.Partition, ev.TopicPartition.Offset)
				}
			}
		}
	}()

	return &Producer{producer: p, topic: "orders"}, nil
}

func (p *Producer) PublishOrderEvent(event *models.KafkaOrderEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return p.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &p.topic,
			Partition: kafka.PartitionAny,
		},
		Key:   []byte(event.UserID), // key by user for ordering
		Value: data,
	}, nil)
}

func (p *Producer) Close() {
	p.producer.Flush(5000)
	p.producer.Close()
}
