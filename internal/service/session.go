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

var ErrSessionConflict = errors.New("session already exists")

type Session struct {
	ID               string `json:"id"`
	UserID           uint   `json:"user_id"`
	UserUUID         string `json:"user_uuid"`
	RefreshTokenHash string `json:"refresh_token_hash"`
	CreatedAt        int64  `json:"created_at"`
	ExpiresAt        int64  `json:"expires_at"`
}

type SessionService interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	RotateSession(ctx context.Context, sessionID string, current, next string, remainingTTL time.Duration) (bool, error)
	DeleteSession(ctx context.Context, sessionID string) error
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

func (s *sessionService) CreateSession(ctx context.Context, session *Session) error {
	now := time.Now().UTC()
	session.CreatedAt = now.Unix()
	session.ExpiresAt = now.Add(s.ttl).Unix()

	value, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	key := common.RedisKeys.AuthSessionKey(session.ID)
	created, err := s.client.SetNX(ctx, key, value, s.ttl).Result()
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	if !created {
		return ErrSessionConflict
	}
	return nil
}

func (s *sessionService) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	key := common.RedisKeys.AuthSessionKey(sessionID)
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
	
	if session["refresh_token_hash"] ~= ARGV[1] then
		return 0
	end
	
	session["refresh_token_hash"] = ARGV[2]
	
	redis.call(
		"SET",
		KEYS[1],
		cjson.encode(session),
		"PX",
		ARGV[3]
	)
	
	return 1
`)

func (s *sessionService) RotateSession(ctx context.Context, sessionID string, current, next string, remainingTTL time.Duration) (bool, error) {
	key := common.RedisKeys.AuthSessionKey(sessionID)
	result, err := rotateRefreshTokenScript.Run(ctx, s.client, []string{key}, current, next, remainingTTL.Milliseconds()).Int64()
	if err != nil {
		return false, fmt.Errorf("rotate refresh token: %w", err)
	}
	return result == 1, nil
}

func (s *sessionService) DeleteSession(ctx context.Context, sessionID string) error {
	key := common.RedisKeys.AuthSessionKey(sessionID)
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *sessionService) TTLSeconds() int64 {
	return int64(s.ttl.Seconds())
}
