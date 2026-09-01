package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"mmchat/internal/messaging"
	"mmchat/internal/service"
	"mmchat/internal/zlog"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

// MessageWriter 消费消息命令，完成落库后发布最终处理结果。
type MessageWriter struct {
	consumer     *kgo.Client
	publisher    messaging.MessageOutcomePublisher
	service      service.MessageService
	batchSize    int
	timeout      time.Duration
	retryBackoff time.Duration
}

func NewMessageWriter(
	consumer *kgo.Client,
	publisher messaging.MessageOutcomePublisher,
	service service.MessageService,
	batchSize int,
	timeout time.Duration,
	retryBackoff time.Duration,
) (*MessageWriter, error) {
	if consumer == nil || publisher == nil || service == nil {
		return nil, errors.New("message writer dependency is nil")
	}
	if batchSize <= 0 || timeout <= 0 || retryBackoff <= 0 {
		return nil, errors.New("message writer config must be positive")
	}
	return &MessageWriter{
		consumer:     consumer,
		publisher:    publisher,
		service:      service,
		batchSize:    batchSize,
		timeout:      timeout,
		retryBackoff: retryBackoff,
	}, nil
}

func (w *MessageWriter) Run(ctx context.Context) error {
	for {
		// 当前批次处理完成前禁止 Rebalance，避免分区转移导致同一批消息并发处理。
		fetches := w.consumer.PollRecords(ctx, w.batchSize)
		if stop := logMessageWriterFetchErrors(ctx, fetches.Errors()); stop {
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

		zlog.Error("process message batch", zap.Int("records", len(records)), zap.Error(err))
		// 将涉及的分区退回本批起点，保证失败批次能够完整重试。
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

func (w *MessageWriter) processBatch(
	ctx context.Context,
	records []*kgo.Record,
) error {
	commands := make([]messaging.MessageCommand, 0, len(records))
	for _, record := range records {
		var command messaging.MessageCommand
		if err := json.Unmarshal(record.Value, &command); err != nil {
			zlog.Warn(
				"discard invalid message command",
				zap.String("topic", record.Topic),
				zap.Int32("partition", record.Partition),
				zap.Int64("offset", record.Offset),
				zap.Error(err),
			)
			continue
		}
		commands = append(commands, command)
	}

	// 业务校验失败会生成 rejected；只有基础设施异常才中断并重试整批。
	outcomes, err := w.service.Process(ctx, commands)
	if err != nil {
		return fmt.Errorf("write message batch: %w", err)
	}
	for i := range outcomes {
		if outcomes[i].Rejected != nil {
			zlog.Warn(
				"reject message command",
				zap.Uint("sender_id", outcomes[i].Rejected.SenderID),
				zap.Uint("conversation_id", outcomes[i].Rejected.ConversationID),
				zap.String("client_message_id", outcomes[i].Rejected.ClientMessageID),
				zap.String("code", outcomes[i].Rejected.Code),
			)
		}
	}
	// 消息成功提交 MySQL 后才能发布最终结果。
	if err := w.publisher.Publish(ctx, outcomes); err != nil {
		return err
	}
	// 最终结果成功写入 Kafka 后，才能确认消费消息命令。
	if err := w.consumer.CommitRecords(ctx, records...); err != nil {
		return fmt.Errorf("commit message command offsets: %w", err)
	}
	return nil
}

func logMessageWriterFetchErrors(ctx context.Context, fetchErrors []kgo.FetchError) bool {
	for _, fetchError := range fetchErrors {
		if errors.Is(fetchError.Err, kgo.ErrClientClosed) || errors.Is(fetchError.Err, context.Canceled) {
			return true
		}
		if ctx.Err() != nil {
			return true
		}
		zlog.Error(
			"fetch message commands",
			zap.String("topic", fetchError.Topic),
			zap.Int32("partition", fetchError.Partition),
			zap.Error(fetchError.Err),
		)
	}
	return false
}

// messageBatchStartOffsets 返回本批消息在每个分区中的最早 offset。
func messageBatchStartOffsets(records []*kgo.Record) map[string]map[int32]kgo.EpochOffset {
	offsets := make(map[string]map[int32]kgo.EpochOffset)
	for _, record := range records {
		partitions := offsets[record.Topic]
		if partitions == nil {
			partitions = make(map[int32]kgo.EpochOffset)
			offsets[record.Topic] = partitions
		}
		current, exists := partitions[record.Partition]
		if !exists || record.Offset < current.Offset {
			partitions[record.Partition] = kgo.EpochOffset{
				Epoch:  record.LeaderEpoch,
				Offset: record.Offset,
			}
		}
	}
	return offsets
}
