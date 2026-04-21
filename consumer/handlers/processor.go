package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type OrderEvent struct {
	EventType string    `json:"event_type"`
	OrderID   string    `json:"order_id"`
	UserID    string    `json:"user_id"`
	Product   string    `json:"product_name"`
	Quantity  int       `json:"quantity"`
	Total     float64   `json:"total"`
	Status    string    `json:"status"`
	Shard     int       `json:"shard"`
	Timestamp time.Time `json:"timestamp"`
}

type NotificationService struct{}

func NewNotificationService() *NotificationService {
	return &NotificationService{}
}

// Process handles incoming Kafka messages
// NOTE: Consumer has NO access to S3 or Postgres - only processes Kafka events
func (n *NotificationService) Process(key, value []byte) error {
	var event OrderEvent
	if err := json.Unmarshal(value, &event); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}

	// Simulate different processing based on event type
	switch event.EventType {
	case "order.created":
		return n.handleOrderCreated(event)
	default:
		log.Printf("[CONSUMER] Unknown event type: %s", event.EventType)
	}
	return nil
}

func (n *NotificationService) handleOrderCreated(event OrderEvent) error {
	// Simulate sending notification / email / downstream processing
	notification := buildNotification(event)
	
	log.Printf("[CONSUMER] ════════════════════════════════════")
	log.Printf("[CONSUMER] 📦 NEW ORDER RECEIVED")
	log.Printf("[CONSUMER] Order ID:  %s", event.OrderID)
	log.Printf("[CONSUMER] User:      %s", event.UserID)
	log.Printf("[CONSUMER] Product:   %s (x%d)", event.Product, event.Quantity)
	log.Printf("[CONSUMER] Total:     $%.2f", event.Total)
	log.Printf("[CONSUMER] Shard:     shard%d", event.Shard)
	log.Printf("[CONSUMER] Partition: based on user_id key")
	log.Printf("[CONSUMER] ────────────────────────────────────")
	log.Printf("[CONSUMER] 📧 NOTIFICATION SENT:")
	log.Printf("[CONSUMER] %s", notification)
	log.Printf("[CONSUMER] ════════════════════════════════════")

	// Simulate email sending delay
	time.Sleep(50 * time.Millisecond)
	return nil
}

func buildNotification(event OrderEvent) string {
	return fmt.Sprintf(
		"Dear %s, your order for %d x %s (Total: $%.2f) has been confirmed! Order ID: %s",
		event.UserID, event.Quantity, event.Product, event.Total, event.OrderID,
	)
}
