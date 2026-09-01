package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mmchat/internal/model"

	"gorm.io/gorm"
)

type MediaRepository interface {
	Create(ctx context.Context, media *model.Media) error
	FindByID(ctx context.Context, id uint) (*model.Media, error)
	ListByIDs(ctx context.Context, ids []uint) ([]model.Media, error)
	MarkUploaded(ctx context.Context, ID uint, etag string, uploadedAt time.Time) (bool, error)
}

type mediaRepository struct {
	db *gorm.DB
}

var _ MediaRepository = (*mediaRepository)(nil)

func NewMediaRepository(db *gorm.DB) MediaRepository {
	return &mediaRepository{db: db}
}

func (r *mediaRepository) Create(ctx context.Context, media *model.Media) error {
	if err := gorm.G[model.Media](r.db).Create(ctx, media); err != nil {
		return fmt.Errorf("create media: %w", err)
	}
	return nil
}

func (r *mediaRepository) FindByID(ctx context.Context, id uint) (*model.Media, error) {
	record, err := gorm.G[model.Media](r.db).Where("id = ?", id).Take(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find media by id: %w", err)
	}
	return &record, nil
}

func (r *mediaRepository) ListByIDs(ctx context.Context, ids []uint) ([]model.Media, error) {
	if len(ids) == 0 {
		return []model.Media{}, nil
	}

	media, err := gorm.G[model.Media](r.db).Where("id IN ?", ids).Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("list media by ids: %w", err)
	}
	return media, nil
}

func (r *mediaRepository) MarkUploaded(ctx context.Context, ID uint, etag string, uploadedAt time.Time) (bool, error) {
	rowsAffected, err := gorm.G[model.Media](r.db).
		Where("id = ? && status = ?", ID, model.MediaStatusPending).
		Updates(ctx, model.Media{
			Status:     model.MediaStatusUploaded,
			ETag:       etag,
			UploadedAt: &uploadedAt,
		})
	if err != nil {
		return false, fmt.Errorf("mark media uploaded: %w", err)
	}
	return rowsAffected == 1, nil
}
