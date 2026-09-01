package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mmchat/internal/common"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const webSocketTicketSize = 32

var ErrInvalidWebSocketTicket = errors.New("invalid websocket ticket")

type WebSocketTicket struct {
	Value     string
	ExpiresAt time.Time
}

type WebSocketTicketIdentity struct {
	UserID    uint   `json:"user_id"`
	SessionID string `json:"session_id"`
}

type WebSocketTicketIssuer interface {
	Issue(ctx context.Context, userID uint, sessionID string) (*WebSocketTicket, error)
}

type WebSocketTicketConsumer interface {
	Consume(ctx context.Context, ticket string) (*WebSocketTicketIdentity, error)
}

type WebSocketTicketService interface {
	WebSocketTicketIssuer
	WebSocketTicketConsumer
}

type webSocketTicketService struct {
	client   *redis.Client
	sessions SessionService
	ttl      time.Duration
}

var _ WebSocketTicketService = (*webSocketTicketService)(nil)

func NewWebSocketTicketService(
	client *redis.Client,
	sessions SessionService,
	ttl time.Duration,
) (WebSocketTicketService, error) {
	if client == nil || sessions == nil {
		return nil, errors.New("websocket ticket dependency is nil")
	}
	if ttl <= 0 {
		return nil, errors.New("websocket ticket TTL must be positive")
	}
	return &webSocketTicketService{client: client, sessions: sessions, ttl: ttl}, nil
}

func (s *webSocketTicketService) Issue(ctx context.Context, userID uint, sessionID string) (*WebSocketTicket, error) {
	if userID == 0 || sessionID == "" {
		return nil, errors.New("invalid websocket ticket identity")
	}

	identity, err := json.Marshal(WebSocketTicketIdentity{
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal websocket ticket identity: %w", err)
	}

	ticket, err := generateWebSocketTicket()
	if err != nil {
		return nil, err
	}

	stored, err := s.client.SetNX(ctx, webSocketTicketKey(ticket), identity, s.ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("store websocket ticket: %w", err)
	}

	if !stored {
		return nil, errors.New("generate unique websocket ticket failed")
	}

	return &WebSocketTicket{
		Value:     ticket,
		ExpiresAt: time.Now().UTC().Add(s.ttl),
	}, nil
}

func (s *webSocketTicketService) Consume(ctx context.Context, ticket string) (*WebSocketTicketIdentity, error) {
	if !validWebSocketTicket(ticket) {
		return nil, ErrInvalidWebSocketTicket
	}

	value, err := s.client.GetDel(ctx, webSocketTicketKey(ticket)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrInvalidWebSocketTicket
	}
	if err != nil {
		return nil, fmt.Errorf("consume websocket ticket: %w", err)
	}

	var identity WebSocketTicketIdentity
	if err := json.Unmarshal(value, &identity); err != nil {
		return nil, fmt.Errorf("unmarshal websocket ticket identity: %w", err)
	}
	if identity.UserID == 0 || identity.SessionID == "" {
		return nil, errors.New("invalid stored websocket ticket identity")
	}

	session, err := s.sessions.GetSession(ctx, identity.UserID)
	if err != nil {
		return nil, err
	}
	if session == nil ||
		session.UserID != identity.UserID ||
		session.ID != identity.SessionID {
		return nil, ErrInvalidWebSocketTicket
	}

	return &identity, nil
}

func generateWebSocketTicket() (string, error) {
	value := make([]byte, webSocketTicketSize)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate websocket ticket: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func validWebSocketTicket(ticket string) bool {
	if ticket == "" || strings.TrimSpace(ticket) != ticket {
		return false
	}

	value, err := base64.RawURLEncoding.DecodeString(ticket)
	if err != nil || len(value) != webSocketTicketSize {
		return false
	}
	return base64.RawURLEncoding.EncodeToString(value) == ticket
}

func webSocketTicketKey(ticket string) string {
	digest := sha256.Sum256([]byte(ticket))
	return common.RedisKeys.WebSocketTicketKey(hex.EncodeToString(digest[:]))
}
