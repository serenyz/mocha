package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"mmchat/internal/messaging"
	"mmchat/internal/zlog"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type MessageSubscriber struct {
	client  *redis.Client
	channel string
	hub     *Hub
}

func NewMessageSubscriber(
	client *redis.Client,
	channel string,
	hub *Hub,
) (*MessageSubscriber, error) {
	if client == nil || hub == nil {
		return nil, errors.New("message subscriber dependency is nil")
	}
	if strings.TrimSpace(channel) == "" {
		return nil, errors.New("message subscriber channel is empty")
	}
	return &MessageSubscriber{client: client, channel: channel, hub: hub}, nil
}

func (s *MessageSubscriber) Run(ctx context.Context) error {
	subscription := s.client.Subscribe(ctx, s.channel)
	defer subscription.Close()
	messages := subscription.Channel(redis.WithChannelSize(256))

	for {
		select {
		case <-ctx.Done():
			return nil
		case message, open := <-messages:
			if !open {
				return nil
			}
			s.dispatch(message)
		}
	}
}

func (s *MessageSubscriber) dispatch(message *redis.Message) {
	var push messaging.MessagePush
	if message == nil || json.Unmarshal([]byte(message.Payload), &push) != nil || !validMessagePush(&push) {
		zlog.Warn("discard invalid Redis message push", zap.String("channel", s.channel))
		return
	}

	payload, err := json.Marshal(push.Event)
	if err != nil {
		zlog.Warn("marshal WebSocket message push", zap.Error(err))
		return
	}
	for _, userID := range push.UserIDs {
		s.hub.sendPayloadToUser(userID, payload)
	}
}

func validMessagePush(push *messaging.MessagePush) bool {
	if push == nil || len(push.UserIDs) == 0 || !json.Valid(push.Event.Data) {
		return false
	}
	if push.Event.Type != messaging.MessagePushCreated &&
		push.Event.Type != messaging.MessagePushRejected &&
		push.Event.Type != messaging.MessagePushDelivered &&
		push.Event.Type != messaging.MessagePushRead {
		return false
	}
	for _, userID := range push.UserIDs {
		if userID == 0 {
			return false
		}
	}
	return true
}
