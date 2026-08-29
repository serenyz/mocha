package repository

import (
	"context"
	"errors"
	"fmt"
	"mmchat/internal/model"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MediaRepository interface {
	Create(ctx context.Context, media *model.Media) error
	FindByUUID(ctx context.Context, mediaUUID string) (*model.Media, error)
	FindDetailByUUID(ctx context.Context, mediaUUID string) (*model.Media, error)
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

func (r *mediaRepository) FindDetailByUUID(ctx context.Context, mediaUUID string) (*model.Media, error) {
	detail, err := gorm.G[model.Media](r.db).
		Joins(clause.InnerJoin.Association("User"), nil).
		Where("`media`.`uuid` = ?", mediaUUID).
		Take(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find media detail by uuid: %w", err)
	}
	return &detail, nil
}

func (r *mediaRepository) FindByUUID(ctx context.Context, mediaUUID string) (*model.Media, error) {
	record, err := gorm.G[model.Media](r.db).Where("uuid = ?", mediaUUID).Take(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find media by uuid: %w", err)
	}
	return &record, nil
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
