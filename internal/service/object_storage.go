package service

import (
	"context"
	"errors"
	"fmt"
	"mmchat/internal/component"
	"mmchat/internal/config"
	"time"

	"github.com/minio/minio-go/v7"
)

type PresignPutObjectCommand struct {
	StorageKey    string
	ContentType   string
	ContentLength int64
	ExpiresIn     time.Duration
}

type PresignPutObjectRes struct {
	Method    string
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}

type StatObjectRes struct {
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
}

type PresignGetObjectCommand struct {
	StorageKey string
	ExpireIn   time.Duration
}

type PresignGetObjectRes struct {
	URL       string
	ExpiresAt time.Time
}

type ObjectStorageService interface {
	PresignPutObject(ctx context.Context, cmd *PresignPutObjectCommand, res *PresignPutObjectRes) error
	PresignGetObject(ctx context.Context, cmd *PresignGetObjectCommand, res *PresignGetObjectRes) error
	StatObject(ctx context.Context, storageKey string, res *StatObjectRes) error
}

var ErrObjectNotFound = errors.New("object not found")

type minioStorageService struct {
	client *minio.Client
	bucket string
}

var _ ObjectStorageService = (*minioStorageService)(nil)

func NewObjectStorageService(cfg *config.MinIOConfig) (ObjectStorageService, error) {
	client, err := component.InitMinio(cfg)
	if err != nil {
		return nil, fmt.Errorf("init minio client: %w", err)
	}
	return &minioStorageService{client: client, bucket: cfg.Bucket}, nil
}

func (s *minioStorageService) PresignPutObject(ctx context.Context, cmd *PresignPutObjectCommand, res *PresignPutObjectRes) error {
	now := time.Now().UTC()

	presignUrl, err := s.client.PresignedPutObject(ctx, s.bucket, cmd.StorageKey, cmd.ExpiresIn)
	if err != nil {
		return fmt.Errorf("presign MinIO PUT object: %w", err)
	}

	res.URL = presignUrl.String()
	res.Headers = map[string]string{"Content-Type": cmd.ContentType}
	res.ExpiresAt = now.Add(cmd.ExpiresIn)
	return nil
}

func (s *minioStorageService) PresignGetObject(ctx context.Context, cmd *PresignGetObjectCommand, res *PresignGetObjectRes) error {
	now := time.Now().UTC()

	presignUrl, err := s.client.PresignedGetObject(ctx, s.bucket, cmd.StorageKey, cmd.ExpireIn, nil)
	if err != nil {
		return fmt.Errorf("presign MinIO Get object: %w", err)
	}

	res.URL = presignUrl.String()
	res.ExpiresAt = now.Add(cmd.ExpireIn)
	return nil
}

func (s *minioStorageService) StatObject(ctx context.Context, storageKey string, res *StatObjectRes) error {
	info, err := s.client.StatObject(ctx, s.bucket, storageKey, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
			return ErrObjectNotFound
		}
		return fmt.Errorf("stat MinIO object: %w", err)
	}

	res.ETag = info.ETag
	res.Size = info.Size
	res.ContentType = info.ContentType
	res.LastModified = info.LastModified
	return nil
}
