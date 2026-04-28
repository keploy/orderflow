package models

import (
	"encoding/json"
	"time"
)

// fixedNanoLayout always emits 9 fractional-second digits so the JSON byte
// width of timestamps is constant across runs.
const fixedNanoLayout = "2006-01-02T15:04:05.000000000Z"

type Order struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	ProductName string    `json:"product_name"`
	Quantity    int       `json:"quantity"`
	Price       float64   `json:"price"`
	Status      string    `json:"status"`
	ReceiptKey  string    `json:"receipt_s3_ke,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (o Order) MarshalJSON() ([]byte, error) {
	type alias Order
	return json.Marshal(&struct {
		alias
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{
		alias:     alias(o),
		CreatedAt: o.CreatedAt.UTC().Format(fixedNanoLayout),
		UpdatedAt: o.UpdatedAt.UTC().Format(fixedNanoLayout),
	})
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
