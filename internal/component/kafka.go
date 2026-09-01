package component

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"mmchat/internal/config"

	"github.com/twmb/franz-go/pkg/kgo"
)

type KafkaClients struct {
	CommandProducer *kgo.Client
	OutcomeProducer *kgo.Client
	MessageWriter   *kgo.Client
	MessagePush     *kgo.Client
}

func InitKafka(cfg *config.KafkaConfig) (*KafkaClients, error) {
	if err := validateKafkaConfig(cfg); err != nil {
		return nil, err
	}

	// 命令和结果使用独立 Producer，避免入口流量占满缓冲区后阻塞 MessageWriter。
	commandProducer, err := newKafkaProducer(
		cfg,
		"command-producer",
		cfg.ProduceTimeout.Duration,
	)
	if err != nil {
		return nil, fmt.Errorf("create message command producer: %w", err)
	}
	// 内部结果由 MessageWriter 的 context 控制超时，不使用面向客户端的交付期限。
	outcomeProducer, err := newKafkaProducer(cfg, "outcome-producer", 0)
	if err != nil {
		commandProducer.Close()
		return nil, fmt.Errorf("create message outcome producer: %w", err)
	}
	writer, err := newKafkaConsumer(cfg, cfg.WriterGroup, cfg.CommandTopic, "writer")
	if err != nil {
		outcomeProducer.Close()
		commandProducer.Close()
		return nil, fmt.Errorf("create message writer consumer: %w", err)
	}
	push, err := newKafkaConsumer(cfg, cfg.PushGroup, cfg.CommittedTopic, "push")
	if err != nil {
		writer.CloseAllowingRebalance()
		outcomeProducer.Close()
		commandProducer.Close()
		return nil, fmt.Errorf("create message push consumer: %w", err)
	}

	clients := &KafkaClients{
		CommandProducer: commandProducer,
		OutcomeProducer: outcomeProducer,
		MessageWriter:   writer,
		MessagePush:     push,
	}
	for name, producer := range map[string]*kgo.Client{
		"message command producer": commandProducer,
		"message outcome producer": outcomeProducer,
	} {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout.Duration)
		err := producer.Ping(ctx)
		cancel()
		if err != nil {
			clients.Close()
			return nil, fmt.Errorf("ping %s: %w", name, err)
		}
	}
	return clients, nil
}

func (c *KafkaClients) Close() {
	if c == nil {
		return
	}
	if c.MessagePush != nil {
		c.MessagePush.CloseAllowingRebalance()
	}
	if c.MessageWriter != nil {
		c.MessageWriter.CloseAllowingRebalance()
	}
	if c.OutcomeProducer != nil {
		c.OutcomeProducer.Close()
	}
	if c.CommandProducer != nil {
		c.CommandProducer.Close()
	}
}

func newKafkaProducer(
	cfg *config.KafkaConfig,
	suffix string,
	deliveryTimeout time.Duration,
) (*kgo.Client, error) {
	options := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID + "-" + suffix),
		kgo.DialTimeout(cfg.DialTimeout.Duration),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)),
	}
	if deliveryTimeout > 0 {
		options = append(options, kgo.RecordDeliveryTimeout(deliveryTimeout))
	}
	return kgo.NewClient(options...)
}

func newKafkaConsumer(
	cfg *config.KafkaConfig,
	group string,
	topic string,
	suffix string,
) (*kgo.Client, error) {
	return kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ClientID(cfg.ClientID+"-"+suffix),
		kgo.DialTimeout(cfg.DialTimeout.Duration),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.FetchMaxWait(cfg.BatchWait.Duration),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
	)
}

func validateKafkaConfig(cfg *config.KafkaConfig) error {
	if cfg == nil {
		return errors.New("kafka config is nil")
	}
	if len(cfg.Brokers) == 0 {
		return errors.New("kafka brokers are empty")
	}
	for _, broker := range cfg.Brokers {
		if strings.TrimSpace(broker) == "" {
			return errors.New("kafka broker is empty")
		}
	}
	if cfg.ClientID == "" || cfg.CommandTopic == "" || cfg.CommittedTopic == "" {
		return errors.New("kafka client id or topic is empty")
	}
	if cfg.WriterGroup == "" || cfg.PushGroup == "" {
		return errors.New("kafka consumer group is empty")
	}
	if cfg.CommandTopic == cfg.CommittedTopic {
		return errors.New("kafka topics must be different")
	}
	if cfg.WriterGroup == cfg.PushGroup {
		return errors.New("kafka consumer groups must be different")
	}
	if cfg.BatchSize <= 0 || cfg.BatchWait.Duration <= 0 {
		return errors.New("kafka batch config must be positive")
	}
	if cfg.DialTimeout.Duration <= 0 || cfg.ProduceTimeout.Duration <= 0 {
		return errors.New("kafka timeout must be positive")
	}
	if cfg.WriterTimeout.Duration <= 0 || cfg.WriterRetry.Duration <= 0 {
		return errors.New("message writer timeout must be positive")
	}
	if cfg.PushTimeout.Duration <= 0 || cfg.PushRetry.Duration <= 0 {
		return errors.New("message push timeout must be positive")
	}
	return nil
}
