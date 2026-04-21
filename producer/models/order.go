package models

import "time"

type Order struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	ProductName string    `json:"product_name"`
	Quantity    int       `json:"quantity"`
	Price       float64   `json:"price"`
	Status      string    `json:"status"`
	ReceiptKey  string    `json:"receipt_s3_key,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateOrderRequest struct {
	UserID      string  `json:"user_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	Price       float64 `json:"price"`
}

type KafkaOrderEvent struct {
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
