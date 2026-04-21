package storage

import (
	"database/sql"
	"fmt"
	"unicode"

	_ "github.com/lib/pq"
	"orderflow/producer/models"
)

type ShardedDB struct {
	shard1 *sql.DB
	shard2 *sql.DB
}

func NewShardedDB(dsn1, dsn2 string) (*ShardedDB, error) {
	db1, err := sql.Open("postgres", dsn1)
	if err != nil {
		return nil, fmt.Errorf("shard1 connect: %w", err)
	}
	if err := db1.Ping(); err != nil {
		return nil, fmt.Errorf("shard1 ping: %w", err)
	}

	db2, err := sql.Open("postgres", dsn2)
	if err != nil {
		return nil, fmt.Errorf("shard2 connect: %w", err)
	}
	if err := db2.Ping(); err != nil {
		return nil, fmt.Errorf("shard2 ping: %w", err)
	}

	return &ShardedDB{shard1: db1, shard2: db2}, nil
}

// ShardFor determines which shard based on user_id first character (A-M = shard1, N-Z = shard2)
func (s *ShardedDB) ShardFor(userID string) (*sql.DB, int) {
	if len(userID) == 0 {
		return s.shard1, 1
	}
	firstChar := unicode.ToUpper(rune(userID[0]))
	if firstChar >= 'A' && firstChar <= 'M' {
		return s.shard1, 1
	}
	return s.shard2, 2
}

func (s *ShardedDB) InsertOrder(order *models.Order) (int, error) {
	db, shardNum := s.ShardFor(order.UserID)
	query := `
		INSERT INTO orders (id, user_id, product_name, quantity, price, status, receipt_s3_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := db.Exec(query,
		order.ID, order.UserID, order.ProductName, order.Quantity,
		order.Price, order.Status, order.ReceiptKey, order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert order shard%d: %w", shardNum, err)
	}
	return shardNum, nil
}

func (s *ShardedDB) UpdateReceiptKey(orderID, userID, receiptKey string) error {
	db, _ := s.ShardFor(userID)
	_, err := db.Exec(`UPDATE orders SET receipt_s3_key=$1, updated_at=NOW() WHERE id=$2`, receiptKey, orderID)
	return err
}

func (s *ShardedDB) GetOrdersByUser(userID string) ([]*models.Order, error) {
	db, _ := s.ShardFor(userID)
	rows, err := db.Query(`SELECT id, user_id, product_name, quantity, price, status, COALESCE(receipt_s3_key,''), created_at, updated_at FROM orders WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*models.Order
	for rows.Next() {
		o := &models.Order{}
		if err := rows.Scan(&o.ID, &o.UserID, &o.ProductName, &o.Quantity, &o.Price, &o.Status, &o.ReceiptKey, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func (s *ShardedDB) GetAllOrders(limit int) ([]*models.Order, error) {
	query := `SELECT id, user_id, product_name, quantity, price, status, COALESCE(receipt_s3_key,''), created_at, updated_at FROM orders ORDER BY created_at DESC LIMIT $1`

	var allOrders []*models.Order
	for _, db := range []*sql.DB{s.shard1, s.shard2} {
		rows, err := db.Query(query, limit)
		if err != nil {
			continue
		}
		defer rows.Close()
		for rows.Next() {
			o := &models.Order{}
			if err := rows.Scan(&o.ID, &o.UserID, &o.ProductName, &o.Quantity, &o.Price, &o.Status, &o.ReceiptKey, &o.CreatedAt, &o.UpdatedAt); err != nil {
				continue
			}
			allOrders = append(allOrders, o)
		}
	}
	return allOrders, nil
}

func (s *ShardedDB) Close() {
	s.shard1.Close()
	s.shard2.Close()
}

// ShardName returns shard name for a user
func ShardName(userID string) string {
	if len(userID) == 0 {
		return "shard1 (A-M)"
	}
	firstChar := unicode.ToUpper(rune(userID[0]))
	if firstChar >= 'A' && firstChar <= 'M' {
		return "shard1 (A-M)"
	}
	return "shard2 (N-Z)"
}
