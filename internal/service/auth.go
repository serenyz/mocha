package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"mmchat/internal/api"
	"mmchat/internal/common"
	"mmchat/internal/model"
	"mmchat/internal/repository"
	"mmchat/internal/zlog"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type SendCodeCommand struct {
	Phone string
}

type SendCodeRes struct {
	ExpireIn int
	RetryIn  int
}

type RegisterCommand struct {
	Phone    string
	Password string
	Nickname string
	Code     string
}

type LoginCommand struct {
	Phone    string
	Password string
}

type LoginRes struct {
	AccessToken      string
	RefreshToken     string
	ExpiresIn        int64
	RefreshExpiresIn int64
	User             UserDetail
}

type UserDetail struct {
	ID       uint
	Nickname string
}

type RefreshCommand struct {
	RefreshToken string
}

type RefreshRes struct {
	AccessToken      string
	RefreshToken     string
	ExpiresIn        int64
	RefreshExpiresIn int64
}

type AuthService interface {
	SendRegisterCode(ctx context.Context, cmd *SendCodeCommand) error
	Register(ctx context.Context, cmd *RegisterCommand) error
	Login(ctx context.Context, cmd *LoginCommand, res *LoginRes) error
	Refresh(ctx context.Context, cmd *RefreshCommand, res *RefreshRes) error
	Logout(ctx context.Context, userID uint, sessionID string) error
}

type authService struct {
	sms                  SmsService
	userRepo             repository.UserRepository
	profileRepo          repository.UserProfileRepository
	refreshTokenProvider RefreshTokenProvider
	accessTokenIssuer    AccessTokenIssuer
	sessionService       SessionService
}

var _ AuthService = (*authService)(nil)

func NewAuthService(sms SmsService, userRepo repository.UserRepository, profileRepo repository.UserProfileRepository, refreshTokenProvider RefreshTokenProvider, accessTokenIssuer AccessTokenIssuer, sessionService SessionService) AuthService {
	return &authService{sms: sms, userRepo: userRepo, profileRepo: profileRepo, refreshTokenProvider: refreshTokenProvider, accessTokenIssuer: accessTokenIssuer, sessionService: sessionService}
}

func (s *authService) SendRegisterCode(ctx context.Context, cmd *SendCodeCommand) error {
	phone, err := common.NormalizeMainlandPhone(cmd.Phone)
	if err != nil {
		return err
	}

	if err := s.sms.Allow(ctx, phone); err != nil {
		return err
	}

	// 发送验证码
	code, err := generateVerificationCode()
	if err != nil {
		return err
	}
	if err := s.sms.SendCode(ctx, phone, code); err != nil {
		return err
	}
	return nil
}

func generateVerificationCode() (string, error) {
	number, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", number.Int64()), nil
}

func (s *authService) Register(ctx context.Context, cmd *RegisterCommand) error {
	phone, err := common.NormalizeMainlandPhone(cmd.Phone)
	if err != nil {
		return err
	}

	if err = common.ValidatePassword(cmd.Password); err != nil {
		return err
	}

	nickname, err := common.NormalizeNickname(cmd.Nickname)
	if err != nil {
		return err
	}

	exist, err := s.userRepo.ExistsByPhone(ctx, phone)
	if err != nil {
		return err
	}
	if exist {
		return api.ErrPhoneRegistered
	}

	if err := s.sms.VerifyCode(ctx, phone, cmd.Code); err != nil {
		return err
	}

	PasswordHash, err := bcrypt.GenerateFromPassword([]byte(cmd.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		Phone:        &phone,
		PasswordHash: string(PasswordHash),
		Role:         model.UserRoleNormal,
		Status:       model.UserStatusNormal,
	}
	profile := &model.UserProfile{Nickname: nickname}
	err = s.userRepo.CreateWithProfile(ctx, user, profile)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrUserDuplicate):
		return api.ErrPhoneRegistered
	default:
		return fmt.Errorf("register user: %w", err)
	}
}

func (s *authService) Login(ctx context.Context, cmd *LoginCommand, res *LoginRes) error {
	phone, err := common.NormalizeMainlandPhone(cmd.Phone)
	if err != nil {
		return err
	}

	if cmd.Password == "" || len([]byte(cmd.Password)) > 64 {
		return api.ErrInvalidCredentials
	}

	user, err := s.userRepo.FindByPhone(ctx, phone)
	if err != nil {
		return err
	}

	if user == nil {
		return api.ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(cmd.Password))
	if err != nil {
		return api.ErrInvalidCredentials
	}

	if user.Status == model.UserStatusDisabled {
		return api.ErrAccountDisabled
	}

	profile, err := s.profileRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		return err
	}

	sessionUUID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate session uuid: %w", err)
	}
	sessionID := sessionUUID.String()

	refreshToken, err := s.refreshTokenProvider.Generate(user.ID, sessionID)
	if err != nil {
		return err
	}

	accessToken, err := s.accessTokenIssuer.Issue(AccessClaims{
		UserID:    user.ID,
		SessionID: sessionID,
	})
	if err != nil {
		return err
	}

	session := &Session{
		ID:               sessionID,
		UserID:           user.ID,
		RefreshTokenHash: refreshToken.Hash,
	}
	if err := s.sessionService.ReplaceSession(ctx, session); err != nil {
		return err
	}

	if err := s.userRepo.UpdateLastLoginAt(ctx, user.ID, time.Unix(session.CreatedAt, 0)); err != nil {
		zlog.Warn("update last login time failed", zap.Uint("user_id", user.ID), zap.Error(err))
	}

	res.AccessToken = accessToken.Value
	res.RefreshToken = refreshToken.Value
	res.ExpiresIn = accessToken.ExpiresIn
	res.RefreshExpiresIn = s.sessionService.TTLSeconds()
	res.User = UserDetail{
		ID:       user.ID,
		Nickname: profile.Nickname,
	}

	return nil
}

func (s *authService) Refresh(ctx context.Context, cmd *RefreshCommand, res *RefreshRes) error {
	identity, err := s.refreshTokenProvider.ParseIdentity(
		cmd.RefreshToken,
	)
	if err != nil {
		return api.ErrInvalidRefreshToken
	}

	session, err := s.sessionService.GetSession(
		ctx,
		identity.UserID,
	)
	if err != nil {
		return err
	}

	if session == nil ||
		session.UserID != identity.UserID ||
		session.ID != identity.SessionID {
		return api.ErrInvalidRefreshToken
	}

	if !s.refreshTokenProvider.Match(cmd.RefreshToken, session.RefreshTokenHash) {
		return api.ErrInvalidRefreshToken
	}

	remainingTTL := time.Until(
		time.Unix(session.ExpiresAt, 0),
	)
	if remainingTTL < time.Second {
		_, _ = s.sessionService.DeleteSession(ctx, session.UserID, session.ID)
		return api.ErrInvalidRefreshToken
	}

	user, err := s.userRepo.FindByID(
		ctx,
		session.UserID,
	)
	if err != nil {
		return err
	}

	if user == nil {
		_, _ = s.sessionService.DeleteSession(ctx, session.UserID, session.ID)
		return api.ErrInvalidRefreshToken
	}

	if user.Status != model.UserStatusNormal {
		_, _ = s.sessionService.DeleteSession(ctx, session.UserID, session.ID)
		return api.ErrAccountDisabled
	}

	nextRefreshToken, err := s.refreshTokenProvider.Generate(session.UserID, session.ID)
	if err != nil {
		return err
	}

	nextAccessToken, err := s.accessTokenIssuer.Issue(
		AccessClaims{
			UserID:    session.UserID,
			SessionID: session.ID,
		},
	)
	if err != nil {
		return err
	}

	rotated, err := s.sessionService.RotateSession(
		ctx,
		session.UserID,
		session.ID,
		session.RefreshTokenHash,
		nextRefreshToken.Hash,
	)
	if err != nil {
		return err
	}
	if !rotated {
		return api.ErrInvalidRefreshToken
	}

	res.AccessToken = nextAccessToken.Value
	res.RefreshToken = nextRefreshToken.Value
	res.ExpiresIn = nextAccessToken.ExpiresIn
	res.RefreshExpiresIn = int64(remainingTTL / time.Second)

	return nil
}

func (s *authService) Logout(ctx context.Context, userID uint, sessionID string) error {
	if userID == 0 || sessionID == "" {
		return api.ErrUnauthenticated
	}

	if _, err := s.sessionService.DeleteSession(ctx, userID, sessionID); err != nil {
		return fmt.Errorf("delete login session: %w", err)
	}

	return nil
}
