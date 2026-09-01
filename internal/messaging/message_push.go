package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

type MessagePushPublisher interface {
	Publish(ctx context.Context, pushes []MessagePush) error
}

type messagePushPublisher struct {
	client  *redis.Client
	channel string
}

var _ MessagePushPublisher = (*messagePushPublisher)(nil)

func NewMessagePushPublisher(
	client *redis.Client,
	channel string,
) (MessagePushPublisher, error) {
	if client == nil {
		return nil, errors.New("message push redis client is nil")
	}
	if strings.TrimSpace(channel) == "" {
		return nil, errors.New("message push channel is empty")
	}
	return &messagePushPublisher{client: client, channel: channel}, nil
}

func (p *messagePushPublisher) Publish(
	ctx context.Context,
	pushes []MessagePush,
) error {
	if len(pushes) == 0 {
		return nil
	}

	payloads := make([][]byte, len(pushes))
	for i := range pushes {
		payload, err := json.Marshal(pushes[i])
		if err != nil {
			return fmt.Errorf("marshal message push: %w", err)
		}
		payloads[i] = payload
	}

	_, err := p.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, payload := range payloads {
			pipe.Publish(ctx, p.channel, payload)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("publish message pushes: %w", err)
	}
	return nil
}
