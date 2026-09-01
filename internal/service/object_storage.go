package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"mmchat/internal/common"
	"mmchat/internal/component"
	"mmchat/internal/config"
	"mmchat/internal/zlog"

	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const presignGetCacheSafety = 30 * time.Second

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
	cache  *redis.Client
	bucket string
}

var _ ObjectStorageService = (*minioStorageService)(nil)

func NewObjectStorageService(
	cfg *config.MinIOConfig,
	cache *redis.Client,
) (ObjectStorageService, error) {
	if cache == nil {
		return nil, errors.New("object storage cache is nil")
	}
	client, err := component.InitMinio(cfg)
	if err != nil {
		return nil, fmt.Errorf("init minio client: %w", err)
	}
	return &minioStorageService{client: client, cache: cache, bucket: cfg.Bucket}, nil
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
	cacheSafety := min(presignGetCacheSafety, cmd.ExpireIn/10)
	cacheKey := common.RedisKeys.PresignGetObjectKey(s.bucket, cmd.StorageKey)
	value, err := s.cache.Get(ctx, cacheKey).Bytes()
	if err == nil {
		cached := &PresignGetObjectRes{}
		if json.Unmarshal(value, cached) == nil && cached.ExpiresAt.After(now.Add(cacheSafety)) {
			*res = *cached
			return nil
		}
	} else if !errors.Is(err, redis.Nil) {
		zlog.Warn("get cached presigned object URL", zap.Error(err))
	}

	presignUrl, err := s.client.PresignedGetObject(ctx, s.bucket, cmd.StorageKey, cmd.ExpireIn, nil)
	if err != nil {
		return fmt.Errorf("presign MinIO Get object: %w", err)
	}

	res.URL = presignUrl.String()
	res.ExpiresAt = now.Add(cmd.ExpireIn)
	cacheTTL := cmd.ExpireIn - cacheSafety
	value, err = json.Marshal(res)
	if err == nil {
		if err := s.cache.Set(ctx, cacheKey, value, cacheTTL).Err(); err != nil {
			zlog.Warn("cache presigned object URL", zap.Error(err))
		}
	}
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
