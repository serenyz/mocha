package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

type MessageCommandPublisher interface {
	Publish(command *MessageCommand, callback func(error))
}

type MessageOutcomePublisher interface {
	Publish(ctx context.Context, outcomes []MessageOutcome) error
}

type messageCommandPublisher struct {
	producer *kgo.Client
	topic    string
	timeout  time.Duration
}

func NewMessageCommandPublisher(
	producer *kgo.Client,
	topic string,
	timeout time.Duration,
) (MessageCommandPublisher, error) {
	if producer == nil {
		return nil, errors.New("message command producer is nil")
	}
	if strings.TrimSpace(topic) == "" {
		return nil, errors.New("message command topic is empty")
	}
	if timeout <= 0 {
		return nil, errors.New("message command timeout must be positive")
	}
	return &messageCommandPublisher{producer: producer, topic: topic, timeout: timeout}, nil
}

func (p *messageCommandPublisher) Publish(command *MessageCommand, callback func(error)) {
	if callback == nil {
		callback = func(error) {}
	}
	if command == nil {
		callback(errors.New("message command is nil"))
		return
	}

	value, err := json.Marshal(command)
	if err != nil {
		callback(fmt.Errorf("marshal message command: %w", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	p.producer.Produce(ctx, &kgo.Record{
		Topic:     p.topic,
		Key:       []byte(strconv.FormatUint(uint64(command.ConversationID), 10)),
		Value:     value,
		Timestamp: command.AcceptedAt,
	}, func(_ *kgo.Record, err error) {
		cancel()
		if err != nil {
			callback(fmt.Errorf("publish message command: %w", err))
			return
		}
		callback(nil)
	})
}

type messageOutcomePublisher struct {
	producer *kgo.Client
	topic    string
}

func NewMessageOutcomePublisher(
	producer *kgo.Client,
	topic string,
) (MessageOutcomePublisher, error) {
	if producer == nil {
		return nil, errors.New("message outcome producer is nil")
	}
	if strings.TrimSpace(topic) == "" {
		return nil, errors.New("message outcome topic is empty")
	}
	return &messageOutcomePublisher{producer: producer, topic: topic}, nil
}

func (p *messageOutcomePublisher) Publish(
	ctx context.Context,
	outcomes []MessageOutcome,
) error {
	if len(outcomes) == 0 {
		return nil
	}

	records := make([]*kgo.Record, len(outcomes))
	for i := range outcomes {
		value, err := json.Marshal(outcomes[i])
		if err != nil {
			return fmt.Errorf("marshal message outcome: %w", err)
		}
		conversationID, timestamp, err := messageOutcomeMetadata(&outcomes[i])
		if err != nil {
			return err
		}
		records[i] = &kgo.Record{
			Topic:     p.topic,
			Key:       []byte(strconv.FormatUint(uint64(conversationID), 10)),
			Value:     value,
			Timestamp: timestamp,
		}
	}

	results := p.producer.ProduceSync(ctx, records...)
	for i := range results {
		if results[i].Err != nil {
			return fmt.Errorf("publish message outcome: %w", results[i].Err)
		}
	}
	return nil
}

func messageOutcomeMetadata(outcome *MessageOutcome) (uint, time.Time, error) {
	if outcome.Type == MessageOutcomeCommitted && outcome.Committed != nil && outcome.Rejected == nil {
		return outcome.Committed.ConversationID, outcome.Committed.CreatedAt, nil
	}
	if outcome.Type == MessageOutcomeRejected && outcome.Rejected != nil && outcome.Committed == nil {
		return outcome.Rejected.ConversationID, outcome.Rejected.RejectedAt, nil
	}
	return 0, time.Time{}, errors.New("invalid message outcome")
}
