package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"mmchat/internal/messaging"
	"mmchat/internal/service"
	"mmchat/internal/zlog"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

type MessagePush struct {
	consumer     *kgo.Client
	publisher    messaging.MessagePushPublisher
	service      service.MessageService
	batchSize    int
	timeout      time.Duration
	retryBackoff time.Duration
}

func NewMessagePush(
	consumer *kgo.Client,
	publisher messaging.MessagePushPublisher,
	service service.MessageService,
	batchSize int,
	timeout time.Duration,
	retryBackoff time.Duration,
) (*MessagePush, error) {
	if consumer == nil || publisher == nil || service == nil {
		return nil, errors.New("message push dependency is nil")
	}
	if batchSize <= 0 || timeout <= 0 || retryBackoff <= 0 {
		return nil, errors.New("message push config must be positive")
	}
	return &MessagePush{
		consumer:     consumer,
		publisher:    publisher,
		service:      service,
		batchSize:    batchSize,
		timeout:      timeout,
		retryBackoff: retryBackoff,
	}, nil
}

func (w *MessagePush) Run(ctx context.Context) error {
	for {
		fetches := w.consumer.PollRecords(ctx, w.batchSize)
		if stop := logMessagePushFetchErrors(ctx, fetches.Errors()); stop {
			w.consumer.AllowRebalance()
			return nil
		}

		records := fetches.Records()
		if len(records) == 0 {
			w.consumer.AllowRebalance()
			continue
		}
		attemptContext, cancel := context.WithTimeout(ctx, w.timeout)
		err := w.processBatch(attemptContext, records)
		cancel()
		if err == nil {
			w.consumer.AllowRebalance()
			continue
		}

		zlog.Error("process message pushes", zap.Int("records", len(records)), zap.Error(err))
		w.consumer.SetOffsets(messageBatchStartOffsets(records))
		w.consumer.AllowRebalance()
		timer := time.NewTimer(w.retryBackoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func (w *MessagePush) processBatch(
	ctx context.Context,
	records []*kgo.Record,
) error {
	outcomes := make([]messaging.MessageOutcome, 0, len(records))
	for _, record := range records {
		var outcome messaging.MessageOutcome
		if err := json.Unmarshal(record.Value, &outcome); err != nil {
			logInvalidMessageOutcome(record, err)
			continue
		}
		conversationID, valid := messageOutcomeConversationID(&outcome)
		if !valid || string(record.Key) != strconv.FormatUint(uint64(conversationID), 10) {
			logInvalidMessageOutcome(record, errors.New("invalid outcome or record key"))
			continue
		}
		outcomes = append(outcomes, outcome)
	}

	pushes, err := w.service.Build(ctx, outcomes)
	if err != nil {
		return fmt.Errorf("build message pushes: %w", err)
	}
	if err := w.publisher.Publish(ctx, pushes); err != nil {
		return err
	}
	if err := w.consumer.CommitRecords(ctx, records...); err != nil {
		return fmt.Errorf("commit message outcome offsets: %w", err)
	}
	return nil
}

func messageOutcomeConversationID(outcome *messaging.MessageOutcome) (uint, bool) {
	if outcome == nil {
		return 0, false
	}
	if outcome.Type == messaging.MessageOutcomeCommitted && outcome.Committed != nil && outcome.Rejected == nil {
		message := outcome.Committed
		return message.ConversationID, message.ID > 0 && message.ConversationID > 0 && message.Seq > 0 &&
			message.SenderID > 0 && message.ClientMessageID != ""
	}
	if outcome.Type == messaging.MessageOutcomeRejected && outcome.Rejected != nil && outcome.Committed == nil {
		message := outcome.Rejected
		return message.ConversationID, message.ConversationID > 0 && message.SenderID > 0 &&
			message.ClientMessageID != "" && message.Code != ""
	}
	return 0, false
}

func logInvalidMessageOutcome(record *kgo.Record, err error) {
	zlog.Warn(
		"discard invalid message outcome",
		zap.String("topic", record.Topic),
		zap.Int32("partition", record.Partition),
		zap.Int64("offset", record.Offset),
		zap.Error(err),
	)
}

func logMessagePushFetchErrors(ctx context.Context, fetchErrors []kgo.FetchError) bool {
	for _, fetchError := range fetchErrors {
		if errors.Is(fetchError.Err, kgo.ErrClientClosed) || errors.Is(fetchError.Err, context.Canceled) {
			return true
		}
		if ctx.Err() != nil {
			return true
		}
		zlog.Error(
			"fetch message outcomes",
			zap.String("topic", fetchError.Topic),
			zap.Int32("partition", fetchError.Partition),
			zap.Error(fetchError.Err),
		)
	}
	return false
}
