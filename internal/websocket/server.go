package websocket

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"mmchat/internal/config"
	"mmchat/internal/service"
	"mmchat/internal/zlog"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Identity struct {
	UserID    uint
	SessionID string
}

type Server struct {
	hub      *Hub
	handler  EventHandler
	config   clientConfig
	upgrader websocket.Upgrader
	tickets  service.WebSocketTicketConsumer
}

func NewServer(
	cfg *config.WebSocketConfig,
	tickets service.WebSocketTicketConsumer,
	hub *Hub,
	handler EventHandler,
) (*Server, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	if tickets == nil || hub == nil || handler == nil {
		return nil, errors.New("websocket dependency is nil")
	}

	return &Server{
		hub:     hub,
		handler: handler,
		tickets: tickets,
		config: clientConfig{
			pingInterval:   cfg.PingInterval.Duration,
			pongTimeout:    cfg.PongTimeout.Duration,
			writeTimeout:   cfg.WriteTimeout.Duration,
			maxMessageSize: cfg.MaxMessageSize,
			sendQueueSize:  cfg.SendQueueSize,
		},
		upgrader: websocket.Upgrader{
			HandshakeTimeout: cfg.HandshakeTimeout.Duration,
			CheckOrigin:      originChecker(cfg.AllowedOrigins),
		},
	}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !websocket.IsWebSocketUpgrade(r) {
		http.Error(w, "invalid websocket request", http.StatusBadRequest)
		return
	}
	if !s.upgrader.CheckOrigin(r) {
		http.Error(w, "websocket origin not allowed", http.StatusForbidden)
		return
	}

	identity, err := s.tickets.Consume(r.Context(), r.URL.Query().Get("ticket"))
	if errors.Is(err, service.ErrInvalidWebSocketTicket) {
		http.Error(w, "invalid websocket ticket", http.StatusUnauthorized)
		return
	}
	if err != nil {
		zlog.Error("consume websocket ticket", zap.String("remote_addr", r.RemoteAddr), zap.Error(err))
		http.Error(w, "websocket authentication unavailable", http.StatusServiceUnavailable)
		return
	}

	connection, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		zlog.Warn("upgrade websocket", zap.String("remote_addr", r.RemoteAddr), zap.Error(err))
		return
	}

	client := newClient(s.hub, connection, s.handler, s.config,
		Identity{
			UserID:    identity.UserID,
			SessionID: identity.SessionID,
		}, r.RemoteAddr)
	client.Serve(r.Context())
}

func validateConfig(cfg *config.WebSocketConfig) error {
	if cfg == nil {
		return errors.New("websocket config is nil")
	}
	if cfg.HandshakeTimeout.Duration <= 0 || cfg.TicketTTL.Duration <= 0 {
		return errors.New("websocket handshake config must be positive")
	}
	if cfg.PingInterval.Duration <= 0 || cfg.PongTimeout.Duration <= 0 {
		return errors.New("websocket heartbeat timeout must be positive")
	}
	if cfg.PingInterval.Duration >= cfg.PongTimeout.Duration {
		return errors.New("websocket ping interval must be shorter than pong timeout")
	}
	if cfg.WriteTimeout.Duration <= 0 ||
		cfg.MaxMessageSize <= 0 ||
		cfg.SendQueueSize <= 0 {
		return errors.New("websocket write and capacity config must be positive")
	}
	for _, origin := range cfg.AllowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil ||
			parsed.Scheme == "" ||
			parsed.Host == "" ||
			parsed.Path != "" {
			return fmt.Errorf("invalid websocket origin %q", origin)
		}
	}
	return nil
}

func originChecker(allowedOrigins []string) func(*http.Request) bool {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[strings.TrimSuffix(origin, "/")] = struct{}{}
	}

	return func(r *http.Request) bool {
		origin := strings.TrimSuffix(r.Header.Get("Origin"), "/")
		if origin == "" {
			return true
		}
		if _, ok := allowed[origin]; ok {
			return true
		}
		parsed, err := url.Parse(origin)
		return err == nil && parsed.Host == r.Host
	}
}
