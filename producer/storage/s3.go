package storage

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"orderflow/producer/models"
)

type S3Store struct {
	client   *s3.Client
	bucket   string
	endpoint string
}

func NewS3Store(endpoint, bucket, region, keyID, secret string) (*S3Store, error) {
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, reg string, opts ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               endpoint,
			SigningRegion:     region,
			HostnameImmutable: true,
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(keyID, secret, "")),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("s3 config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &S3Store{client: client, bucket: bucket, endpoint: endpoint}, nil
}

func (s *S3Store) UploadReceipt(order *models.Order, shardNum int) (string, error) {
	total := float64(order.Quantity) * order.Price
	receipt := fmt.Sprintf(`ORDER RECEIPT
=====================================
Order ID:    %s
User ID:     %s
Shard:       shard%d
-------------------------------------
Product:     %s
Quantity:    %d
Unit Price:  $%.2f
Total:       $%.2f
-------------------------------------
Status:      %s
Created At:  %s
=====================================
Thank you for your order!
`, order.ID, order.UserID, shardNum,
		order.ProductName, order.Quantity, order.Price, total,
		order.Status, order.CreatedAt.Format(time.RFC3339))

	key := fmt.Sprintf("receipts/%s/%s.txt", order.UserID, order.ID)

	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader([]byte(receipt)),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		return "", fmt.Errorf("s3 upload: %w", err)
	}

	return key, nil
}
