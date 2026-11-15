package assessment

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"assessment/internal/config"
)

type MinIOStore struct {
	client *minio.Client
	bucket string
}

var _ ObjectStore = (*MinIOStore)(nil)

func NewMinIOStore(cfg *config.Config) (*MinIOStore, error) {
	client, err := minio.New(cfg.ObjectStorageEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.ObjectStorageAccessKey, cfg.ObjectStorageSecretKey, ""),
		Secure: cfg.ObjectStorageUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("configure object storage: %w", err)
	}
	return &MinIOStore{client: client, bucket: cfg.ObjectStorageBucket}, nil
}

func (s *MinIOStore) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check submission bucket: %w", err)
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		exists, checkErr := s.client.BucketExists(ctx, s.bucket)
		if checkErr == nil && exists {
			return nil
		}
		return fmt.Errorf("create submission bucket: %w", err)
	}
	return nil
}

func (s *MinIOStore) Put(
	ctx context.Context,
	objectKey string,
	mediaType string,
	size int64,
	reader io.Reader,
) error {
	_, err := s.client.PutObject(ctx, s.bucket, objectKey, reader, size,
		minio.PutObjectOptions{ContentType: mediaType})
	if err != nil {
		return fmt.Errorf("upload object: %w", err)
	}
	return nil
}

func (s *MinIOStore) Get(ctx context.Context, objectKey string) (io.ReadCloser, string, error) {
	object, err := s.client.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("get object: %w", err)
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return nil, "", fmt.Errorf("stat object: %w", err)
	}
	return object, info.ContentType, nil
}

func (s *MinIOStore) Delete(ctx context.Context, objectKey string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func (s *MinIOStore) Ready(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check object storage readiness: %w", err)
	}
	if !exists {
		return fmt.Errorf("submission bucket does not exist")
	}
	return nil
}
