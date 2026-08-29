package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mmchat/internal/common"
	"time"

	"github.com/redis/go-redis/v9"
)

type Session struct {
	ID               string `json:"id"`
	UserID           uint   `json:"user_id"`
	RefreshTokenHash string `json:"refresh_token_hash"`
	CreatedAt        int64  `json:"created_at"`
	ExpiresAt        int64  `json:"expires_at"`
}

type SessionService interface {
	ReplaceSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, userID uint) (*Session, error)
	RotateSession(
		ctx context.Context,
		userID uint,
		sessionID string,
		currentRefreshTokenHash string,
		nextRefreshTokenHash string,
	) (bool, error)
	DeleteSession(
		ctx context.Context,
		userID uint,
		sessionID string,
	) (bool, error)
	TTLSeconds() int64
}

type sessionService struct {
	client *redis.Client
	ttl    time.Duration
}

var _ SessionService = (*sessionService)(nil)

func NewSessionService(client *redis.Client, ttl time.Duration) SessionService {
	return &sessionService{
		client: client,
		ttl:    ttl,
	}
}

func (s *sessionService) ReplaceSession(
	ctx context.Context,
	session *Session,
) error {
	if session == nil || session.UserID == 0 || session.ID == "" {
		return errors.New("invalid session")
	}

	now := time.Now().UTC()
	session.CreatedAt = now.Unix()
	session.ExpiresAt = now.Add(s.ttl).Unix()

	value, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	key := common.RedisKeys.AuthSessionKey(session.UserID)
	if err := s.client.Set(ctx, key, value, s.ttl).Err(); err != nil {
		return fmt.Errorf("replace session: %w", err)
	}

	return nil
}

func (s *sessionService) GetSession(
	ctx context.Context,
	userID uint,
) (*Session, error) {
	key := common.RedisKeys.AuthSessionKey(userID)

	value, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal([]byte(value), &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &session, nil
}

var rotateRefreshTokenScript = redis.NewScript(`
	local value = redis.call("GET", KEYS[1])
	if not value then
		return 0
	end

	local session = cjson.decode(value)

	if session["id"] ~= ARGV[1] then
		return 0
	end

	if session["refresh_token_hash"] ~= ARGV[2] then
		return 0
	end

	local ttl = redis.call("PTTL", KEYS[1])
	if ttl <= 0 then
		return 0
	end

	session["refresh_token_hash"] = ARGV[3]

	redis.call(
		"SET",
		KEYS[1],
		cjson.encode(session),
		"PX",
		ttl
	)

	return 1
`)

func (s *sessionService) RotateSession(
	ctx context.Context,
	userID uint,
	sessionID string,
	currentRefreshTokenHash string,
	nextRefreshTokenHash string,
) (bool, error) {
	key := common.RedisKeys.AuthSessionKey(userID)

	result, err := rotateRefreshTokenScript.Run(
		ctx,
		s.client,
		[]string{key},
		sessionID,
		currentRefreshTokenHash,
		nextRefreshTokenHash,
	).Int64()
	if err != nil {
		return false, fmt.Errorf("rotate refresh token: %w", err)
	}

	return result == 1, nil
}

var deleteSessionScript = redis.NewScript(`
	local value = redis.call("GET", KEYS[1])
	if not value then
		return 0
	end

	local session = cjson.decode(value)

	if session["id"] ~= ARGV[1] then
		return 0
	end

	redis.call("DEL", KEYS[1])
	return 1
`)

func (s *sessionService) DeleteSession(
	ctx context.Context,
	userID uint,
	sessionID string,
) (bool, error) {
	key := common.RedisKeys.AuthSessionKey(userID)

	deleted, err := deleteSessionScript.Run(
		ctx,
		s.client,
		[]string{key},
		sessionID,
	).Int64()
	if err != nil {
		return false, fmt.Errorf("delete session: %w", err)
	}

	return deleted == 1, nil
}

func (s *sessionService) TTLSeconds() int64 {
	return int64(s.ttl.Seconds())
}
