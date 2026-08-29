package repository

import (
	"context"
	"errors"
	"fmt"
	"mmchat/internal/model"
	"time"

	"gorm.io/gorm"
)

type ProfileUpdateParams struct {
	Nickname      *string
	Gender        *uint8
	Signature     *string
	Birthday      *time.Time
	Country       *string
	Province      *string
	AvatarMediaID *uint
}

type UserProfileRepository interface {
	FindByUserID(ctx context.Context, userID uint) (*model.UserProfile, error)
	UpdateByUserID(ctx context.Context, userID uint, updateParams *ProfileUpdateParams) error
}

type userProfileRepository struct {
	db *gorm.DB
}

var _ UserProfileRepository = (*userProfileRepository)(nil)

func NewUserProfileRepository(db *gorm.DB) UserProfileRepository {
	return &userProfileRepository{db: db}
}

func (r *userProfileRepository) FindByUserID(ctx context.Context, userId uint) (*model.UserProfile, error) {
	profile, err := gorm.G[model.UserProfile](r.db).Where("user_id = ?", userId).First(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find profile by user id: %w", err)
	}

	return &profile, nil
}

func (r *userProfileRepository) UpdateByUserID(ctx context.Context, userID uint, updateParams *ProfileUpdateParams) error {
	updates := make(map[string]any)
	if updateParams.Nickname != nil {
		updates["nickname"] = *updateParams.Nickname
	}
	if updateParams.Gender != nil {
		updates["gender"] = *updateParams.Gender
	}
	if updateParams.Signature != nil {
		updates["signature"] = *updateParams.Signature
	}

	if updateParams.Birthday != nil {
		updates["birthday"] = *updateParams.Birthday
	}
	if updateParams.Country != nil {
		updates["country"] = *updateParams.Country
	}
	if updateParams.Province != nil {
		updates["province"] = *updateParams.Province
	}

	if updateParams.AvatarMediaID != nil {
		updates["avatar_media_id"] = *updateParams.AvatarMediaID
	}

	if len(updates) == 0 {
		return nil
	}

	_, err := gorm.G[map[string]any](r.db).
		Table("user_profile").
		Where("user_id = ?", userID).
		Updates(ctx, updates)
	if err != nil {
		return fmt.Errorf("update profile by user id: %w", err)
	}

	return nil
}
