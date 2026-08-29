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

var ErrUserDuplicate = errors.New("user duplicate")

type SearchUsersParams struct {
	Phone              *string
	Nickname           *string
	Country            *string
	Province           *string
	Gender             *uint8
	BirthdayAfter      *time.Time
	BirthdayOnOrBefore *time.Time
	Cursor             *uint
	Limit              int
}

type UserRepository interface {
	ExistsByPhone(ctx context.Context, phone string) (bool, error)
	CreateWithProfile(ctx context.Context, user *model.User, profile *model.UserProfile) error
	FindByPhone(ctx context.Context, phone string) (*model.User, error)
	FindDetailByUUID(ctx context.Context, userUUID string) (*model.User, error)
	FindByUUID(ctx context.Context, userUUID string) (*model.User, error)
	FindByID(ctx context.Context, id uint) (*model.User, error)
	UpdateLastLoginAt(ctx context.Context, id uint, at time.Time) error
	SearchUsers(ctx context.Context, params *SearchUsersParams) ([]model.User, error)
}

type userRepository struct {
	db *gorm.DB
}

var _ UserRepository = (*userRepository)(nil)

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	count, err := gorm.G[model.User](r.db).
		Select("id").
		Where("phone = ?", phone).
		Count(ctx, "id")

	if err != nil {
		return false, fmt.Errorf("check user exists by phone: %w", err)
	}

	return count > 0, nil
}

func (r *userRepository) CreateWithProfile(ctx context.Context, user *model.User, profile *model.UserProfile) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := gorm.G[model.User](tx).Create(ctx, user); err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		profile.UserID = user.ID
		if err := gorm.G[model.UserProfile](tx).Create(ctx, profile); err != nil {
			return fmt.Errorf("create user profile: %w", err)
		}
		return nil
	})

	if err == nil {
		return nil
	}

	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrUserDuplicate
	}

	return fmt.Errorf("create user with profile: %w", err)
}

func (r *userRepository) FindByPhone(ctx context.Context, phone string) (*model.User, error) {
	user, err := gorm.G[model.User](r.db).Where("phone = ?", phone).First(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find user by phone: %w", err)
	}

	return &user, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*model.User, error) {
	user, err := gorm.G[model.User](r.db).Where("id = ?", id).First(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}

	return &user, err
}

func (r *userRepository) UpdateLastLoginAt(ctx context.Context, id uint, at time.Time) error {
	_, err := gorm.G[model.User](r.db).Where("id = ?", id).Update(ctx, "last_login_at", at)
	if err != nil {
		return fmt.Errorf("update last login time: %w", err)
	}

	return nil
}

func (r *userRepository) FindDetailByUUID(ctx context.Context, userUUID string) (*model.User, error) {
	detail, err := gorm.G[model.User](r.db).
		Joins(clause.InnerJoin.Association("Profile"), nil).
		Joins(clause.LeftJoin.Association("Profile.AvatarMedia"), nil).
		Where("`user`.`uuid` = ?", userUUID).
		Take(ctx)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find user detail by uuid: %w", err)
	}

	return &detail, nil
}

func (r *userRepository) FindByUUID(ctx context.Context, userUUID string) (*model.User, error) {
	user, err := gorm.G[model.User](r.db).Where("uuid = ?", userUUID).Take(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find user by uuid: %w", err)
	}
	return &user, nil
}

func (r *userRepository) SearchUsers(ctx context.Context, params *SearchUsersParams) ([]model.User, error) {
	query := gorm.G[model.User](r.db).
		Joins(clause.InnerJoin.Association("Profile"), nil).
		Joins(clause.LeftJoin.Association("Profile.AvatarMedia"), nil).
		Where("`user`.`status` = ?", model.UserStatusNormal)
	if params.Phone != nil {
		query = query.Where("`user`.`phone` = ?", *params.Phone)
	}

	if params.Nickname != nil {
		query = query.Where("`Profile`.`nickname` LIKE ?", *params.Nickname+"%")
	}

	if params.Country != nil {
		query = query.Where("`Profile`.`country` = ?", *params.Country)
	}

	if params.Province != nil {
		query = query.Where("`Profile`.`province` = ?", *params.Province)
	}

	if params.Gender != nil {
		query = query.Where("`Profile`.`gender` = ?", *params.Gender)
	}

	if params.BirthdayAfter != nil {
		query = query.Where("`Profile`.`birthday` > ?", *params.BirthdayAfter)
	}

	if params.BirthdayOnOrBefore != nil {
		query = query.Where("`Profile`.`birthday` <= ?", *params.BirthdayOnOrBefore)
	}

	if params.Cursor != nil {
		query = query.Where("`user`.`id` < ?", *params.Cursor)
	}

	users, err := query.
		Order(clause.OrderByColumn{Column: clause.Column{Table: "user", Name: "id"}, Desc: true}).
		Limit(params.Limit).Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	return users, nil
}
