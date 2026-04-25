package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"avikmukherjee/m/audit-service/internal/model"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type AuditStore struct {
	client *minio.Client
	bucket string
}

// NewAuditStore connects to MinIO and ensures the audit bucket exists.
func NewAuditStore(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*AuditStore, error) {
	var client *minio.Client
	var err error

	// Retry loop — MinIO may not be ready immediately on first docker compose up
	for i := range 10 {
		client, err = minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: useSSL,
		})
		if err == nil {
			// Verify connectivity with a bucket check
			_, bucketErr := client.BucketExists(context.Background(), bucket)
			if bucketErr == nil {
				log.Println("[audit-store] connected to MinIO")
				break
			}
		}
		wait := time.Duration(i+1) * time.Second
		log.Printf("[audit-store] MinIO not ready, retrying in %s...", wait)
		time.Sleep(wait)
	}
	if err != nil {
		return nil, fmt.Errorf("minio connect: %w", err)
	}

	store := &AuditStore{client: client, bucket: bucket}
	if err := store.ensureBucket(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

// Write serialises the AuditRecord to JSON and stores it in MinIO.
// The object key is derived from AuditRecord.StoragePath() so records
// are partitioned by event_type/YYYY/MM/DD/ for easy retrieval.
func (s *AuditStore) Write(ctx context.Context, record *model.AuditRecord) error {
	payload, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal audit record: %w", err)
	}

	objectKey := record.StoragePath()
	reader := bytes.NewReader(payload)

	_, err = s.client.PutObject(ctx, s.bucket, objectKey, reader, int64(len(payload)),
		minio.PutObjectOptions{
			ContentType: "application/json",
			UserMetadata: map[string]string{
				"event-type":   string(record.EventType),
				"recorded-at":  record.RecordedAt.UTC().Format(time.RFC3339),
				"source-topic": record.SourceTopic,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("minio put object %s: %w", objectKey, err)
	}

	log.Printf("[audit-store] 📁 written: %s/%s", s.bucket, objectKey)
	return nil
}

// List returns all object keys under a given prefix (e.g. "transaction/2026/04/20/")
func (s *AuditStore) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		keys = append(keys, obj.Key)
	}
	return keys, nil
}

// ensureBucket creates the audit bucket if it doesn't already exist.
func (s *AuditStore) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		log.Printf("[audit-store] bucket created: %s", s.bucket)
	} else {
		log.Printf("[audit-store] bucket exists: %s", s.bucket)
	}
	return nil
}
