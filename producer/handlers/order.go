package handlers

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"orderflow/producer/kafka"
	"orderflow/producer/models"
	"orderflow/producer/storage"
)

type OrderHandler struct {
	db       *storage.ShardedDB
	s3       *storage.S3Store
	producer *kafka.Producer
}

func NewOrderHandler(db *storage.ShardedDB, s3 *storage.S3Store, producer *kafka.Producer) *OrderHandler {
	return &OrderHandler{db: db, s3: s3, producer: producer}
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.ProductName == "" || req.Quantity <= 0 || req.Price <= 0 {
		jsonError(w, "user_id, product_name, quantity, and price are required", http.StatusBadRequest)
		return
	}

	hash := md5.Sum([]byte(fmt.Sprintf("%s-%s", req.UserID, req.ProductName)))
	deterministicUUID := fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		hash[0:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])

	now := time.Now()
	order := &models.Order{
		ID:          deterministicUUID,
		UserID:      req.UserID,
		ProductName: req.ProductName,
		Quantity:    req.Quantity,
		Price:       req.Price,
		Status:      "pending",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 1. Write to sharded Postgres
	shardNum, err := h.db.InsertOrder(order)
	if err != nil {
		log.Printf("DB insert error: %v", err)
		jsonError(w, "failed to save order", http.StatusInternalServerError)
		return
	}
	log.Printf("Order %s saved to shard%d for user %s", order.ID, shardNum, order.UserID)

	// 2. Upload receipt to S3
	receiptKey, err := h.s3.UploadReceipt(order, shardNum)
	if err != nil {
		log.Printf("S3 upload error: %v", err)
		// non-fatal, continue
	} else {
		order.ReceiptKey = receiptKey
		h.db.UpdateReceiptKey(order.ID, order.UserID, receiptKey)
		log.Printf("Receipt uploaded to S3: %s", receiptKey)
	}

	// 3. Publish to Kafka
	event := &models.KafkaOrderEvent{
		EventType: "order.created",
		OrderID:   order.ID,
		UserID:    order.UserID,
		Product:   order.ProductName,
		Quantity:  order.Quantity,
		Total:     float64(order.Quantity) * order.Price,
		Status:    order.Status,
		Shard:     shardNum,
		Timestamp: now,
	}
	if err := h.producer.PublishOrderEvent(event); err != nil {
		log.Printf("Kafka publish error: %v", err)
	} else {
		log.Printf("Kafka event published for order %s", order.ID)
	}

	order.Status = "created"
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"order":      order,
		"shard":      fmt.Sprintf("shard%d", shardNum),
		"shard_info": storage.ShardName(order.UserID),
		"receipt":    receiptKey,
	})
}

func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	var orders []*models.Order
	var err error

	if userID != "" {
		orders, err = h.db.GetOrdersByUser(userID)
	} else {
		orders, err = h.db.GetAllOrders(50)
	}
	if err != nil {
		jsonError(w, "failed to fetch orders", http.StatusInternalServerError)
		return
	}
	if orders == nil {
		orders = []*models.Order{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"orders": orders,
		"count":  len(orders),
	})
}

func (h *OrderHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "orderflow-producer",
	})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
