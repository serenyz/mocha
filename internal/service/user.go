package service

import (
	"context"
	"mmchat/internal/api"
	"mmchat/internal/common"
	"mmchat/internal/model"
	"mmchat/internal/repository"
	"time"
)

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
	maxSearchAge       = 150
)

type GetMeCommand struct {
	UUID string
}

type publicProfile struct {
	UUID     string
	Nickname string
	avatar
	Gender    uint8
	Signature string
	Birthday  time.Time
	Country   string
	Province  string
}

type avatar struct {
	AvatarURL    string
	URLExpiredAt time.Time
	MediaUUID    string
}

type GetMeRes struct {
	Phone string
	Email *string
	publicProfile
	CreatedAt time.Time
}

type UpdateMeCommand struct {
	UUID      string
	Nickname  *string
	Gender    *uint8
	Signature *string
	Birthday  *string
	Country   *string
	Province  *string
}

type SearchUsersCommand struct {
	Phone    *string
	Nickname *string
	Country  *string
	Province *string
	Gender   *uint8
	Age      *int
	Cursor   *uint
	Limit    int
}

type SearchUsersRes struct {
	Users      []*publicProfile
	NextCursor *uint
	HasMore    bool
	Limit      int
}

type UpdateAvatarCommand struct {
	UserUUID  string
	MediaUUID string
}

type UpdateAvatarRes struct {
	avatar
}

type UpdateMeRes GetMeRes

type UserService interface {
	GetMe(ctx context.Context, cmd *GetMeCommand, res *GetMeRes) error
	UpdateMe(ctx context.Context, cmd *UpdateMeCommand, res *UpdateMeRes) error
	UpdateAvatar(ctx context.Context, cmd *UpdateAvatarCommand, res *UpdateAvatarRes) error
	SearchUsers(ctx context.Context, cmd *SearchUsersCommand, res *SearchUsersRes) error
}

type userService struct {
	userRepo    repository.UserRepository
	profileRepo repository.UserProfileRepository
	mediaRepo   repository.MediaRepository
	objs        ObjectStorageService
}

var _ UserService = (*userService)(nil)

func NewUserService(
	userRepo repository.UserRepository,
	profileRepo repository.UserProfileRepository,
	mediaRepo repository.MediaRepository,
	objs ObjectStorageService,
) UserService {
	return &userService{
		userRepo:    userRepo,
		profileRepo: profileRepo,
		mediaRepo:   mediaRepo,
		objs:        objs,
	}
}

func (s *userService) GetMe(ctx context.Context, cmd *GetMeCommand, res *GetMeRes) error {
	detail, err := s.userRepo.FindDetailByUUID(ctx, cmd.UUID)
	if err != nil {
		return err
	}

	res.UUID = detail.UUID
	res.Phone = *detail.Phone
	res.Email = detail.Email
	res.Nickname = detail.Profile.Nickname
	res.Gender = detail.Profile.Gender
	res.Signature = detail.Profile.Signature
	res.CreatedAt = detail.CreatedAt
	res.Birthday = detail.Profile.Birthday
	res.Country = detail.Profile.Country
	res.Province = detail.Profile.Province

	if err := s.setAvatarURL(ctx, detail.Profile.AvatarMedia, &res.publicProfile); err != nil {
		return err
	}

	return nil
}

func (s *userService) UpdateMe(ctx context.Context, cmd *UpdateMeCommand, res *UpdateMeRes) error {
	old, err := s.userRepo.FindDetailByUUID(ctx, cmd.UUID)
	if err != nil {
		return err
	}

	res.UUID = old.UUID
	res.Phone = *old.Phone
	res.Email = old.Email
	res.CreatedAt = old.CreatedAt
	res.Nickname = old.Profile.Nickname
	res.Signature = old.Profile.Signature
	res.Gender = old.Profile.Gender
	res.Birthday = old.Profile.Birthday
	res.Country = old.Profile.Country
	res.Province = old.Profile.Province
	updateParams := &repository.ProfileUpdateParams{}

	if cmd.Nickname != nil {
		nickname, err := common.NormalizeNickname(*cmd.Nickname)
		if err != nil {
			return err
		}
		res.Nickname = nickname
		updateParams.Nickname = &nickname
	}

	if cmd.Gender != nil {
		if *cmd.Gender > 2 {
			return api.ErrInvalidGender
		}
		res.Gender = *cmd.Gender
		updateParams.Gender = cmd.Gender
	}

	if cmd.Signature != nil {
		signature, err := common.NormalizeSignature(*cmd.Signature)
		if err != nil {
			return err
		}
		res.Signature = signature
		updateParams.Signature = &signature
	}

	if cmd.Birthday != nil {
		birthday, err := common.ParseBirthday(*cmd.Birthday, time.Now())
		if err != nil {
			return err
		}
		res.Birthday = birthday
		updateParams.Birthday = &birthday
	}

	if cmd.Country != nil {
		country, err := common.NormalizeCountry(*cmd.Country)
		if err != nil {
			return err
		}
		res.Country = country
		updateParams.Country = &country
	}

	if cmd.Province != nil {
		province, err := common.NormalizeProvince(*cmd.Province)
		if err != nil {
			return err
		}
		res.Province = province
		updateParams.Province = &province
	}

	if err := s.profileRepo.UpdateByUserID(ctx, old.ID, updateParams); err != nil {
		return err
	}

	if err := s.setAvatarURL(ctx, old.Profile.AvatarMedia, &res.publicProfile); err != nil {
		return err
	}

	return nil
}

func (s *userService) UpdateAvatar(ctx context.Context, cmd *UpdateAvatarCommand, res *UpdateAvatarRes) error {
	mediaRecord, err := s.mediaRepo.FindDetailByUUID(ctx, cmd.MediaUUID)
	if err != nil {
		return err
	}
	if mediaRecord == nil || mediaRecord.User == nil ||
		mediaRecord.User.UUID != cmd.UserUUID ||
		mediaRecord.Status != model.MediaStatusUploaded ||
		mediaRecord.Type != model.MediaTypeImage {
		return api.ErrMediaNotFound
	}

	getObjectRes := &PresignGetObjectRes{}
	if err := s.objs.PresignGetObject(ctx, &PresignGetObjectCommand{
		StorageKey: mediaRecord.StorageKey,
		ExpireIn:   getObjectTTL,
	}, getObjectRes); err != nil {
		return err
	}

	if err := s.profileRepo.UpdateByUserID(ctx, *mediaRecord.UserID, &repository.ProfileUpdateParams{AvatarMediaID: new(mediaRecord.ID)}); err != nil {
		return err
	}

	res.MediaUUID = mediaRecord.UUID
	res.AvatarURL = getObjectRes.URL
	res.URLExpiredAt = getObjectRes.ExpiresAt
	return nil
}

func (s *userService) SearchUsers(ctx context.Context, cmd *SearchUsersCommand, res *SearchUsersRes) error {
	res.Limit = cmd.Limit
	if res.Limit == 0 {
		res.Limit = defaultSearchLimit
	}
	if res.Limit < 1 || res.Limit > maxSearchLimit {
		return api.ErrInvalidArgument
	}

	if cmd.Cursor != nil && *cmd.Cursor == 0 {
		return api.ErrInvalidArgument
	}

	params := &repository.SearchUsersParams{}
	if cmd.Phone != nil {
		if hasOtherSearchCondition(cmd) {
			return api.ErrInvalidArgument
		}
		phone, err := common.NormalizeMainlandPhone(*cmd.Phone)
		if err != nil {
			return err
		}

		params.Phone = &phone
		params.Limit = 1
	} else {
		if !hasOtherSearchCondition(cmd) {
			return api.ErrInvalidArgument
		}

		if cmd.Nickname != nil {
			nickname, err := common.NormalizeNickname(*cmd.Nickname)
			if err != nil {
				return err
			}
			params.Nickname = &nickname
		}

		if cmd.Country != nil {
			country, err := common.NormalizeCountry(*cmd.Country)
			if err != nil {
				return err
			}
			if country == "" {
				return api.ErrInvalidCountry
			}
			params.Country = &country
		}

		if cmd.Province != nil {
			province, err := common.NormalizeProvince(*cmd.Province)
			if err != nil {
				return err
			}
			if province == "" {
				return api.ErrInvalidProvince
			}
			params.Province = &province
		}

		if cmd.Gender != nil {
			if *cmd.Gender > 2 {
				return api.ErrInvalidGender
			}
			params.Gender = cmd.Gender
		}

		if cmd.Age != nil {
			age := *cmd.Age
			if age < 0 || age > maxSearchAge {
				return api.ErrInvalidArgument
			}
			after, onOrBefore := birthdayRangeForAge(*cmd.Age, time.Now())
			params.BirthdayAfter = &after
			params.BirthdayOnOrBefore = &onOrBefore
		}

		params.Cursor = cmd.Cursor
	}

	params.Limit = res.Limit + 1
	users, err := s.userRepo.SearchUsers(ctx, params)
	if err != nil {
		return err
	}
	res.HasMore = len(users) > res.Limit
	if res.HasMore {
		users = users[:res.Limit]
	}
	res.Users = make([]*publicProfile, len(users))
	for i, user := range users {
		res.Users[i] = &publicProfile{
			UUID:      user.UUID,
			Nickname:  user.Profile.Nickname,
			Gender:    user.Profile.Gender,
			Signature: user.Profile.Signature,
			Birthday:  user.Profile.Birthday,
			Country:   user.Profile.Country,
			Province:  user.Profile.Province,
		}

		if err := s.setAvatarURL(ctx, user.Profile.AvatarMedia, res.Users[i]); err != nil {
			return err
		}
	}

	if res.HasMore {
		res.NextCursor = new(users[len(users)-1].ID)
	}

	return nil
}

func hasOtherSearchCondition(cmd *SearchUsersCommand) bool {
	return cmd.Nickname != nil || cmd.Country != nil || cmd.Province != nil ||
		cmd.Gender != nil || cmd.Age != nil
}

func birthdayRangeForAge(age int, now time.Time) (after time.Time, onOrBefore time.Time) {
	location := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)

	after = yearsBefore(today, age+1)
	onOrBefore = yearsBefore(today, age)
	return
}

func yearsBefore(value time.Time, years int) time.Time {
	year := value.Year() - years
	month := value.Month()
	day := value.Day()

	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, value.Location()).Day()

	if day > lastDay {
		day = lastDay
	}

	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func (s *userService) setAvatarURL(ctx context.Context, avatarMedia *model.Media, profile *publicProfile) error {
	if avatarMedia == nil {
		return nil
	}

	getObjectRes := &PresignGetObjectRes{}
	if err := s.objs.PresignGetObject(ctx, &PresignGetObjectCommand{
		StorageKey: avatarMedia.StorageKey,
		ExpireIn:   getObjectTTL,
	},
		getObjectRes,
	); err != nil {
		return err
	}

	profile.AvatarURL = getObjectRes.URL
	profile.URLExpiredAt = getObjectRes.ExpiresAt
	profile.MediaUUID = avatarMedia.UUID
	return nil
}
