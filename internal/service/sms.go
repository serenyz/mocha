package service

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"mmchat/internal/api"
	"mmchat/internal/common"
	"mmchat/internal/zlog"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Message struct {
	Phone string
	Text  string
}

type Sender interface {
	Send(ctx context.Context, message Message) error
}

type localSms struct {
}

var _ Sender = (*localSms)(nil)

func NewSender() Sender {
	return &localSms{}
}

func (s *localSms) Send(ctx context.Context, message Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	zlog.Info(
		"本地SMS",
		zap.String("phone", message.Phone),
		zap.String("text", message.Text),
	)

	return nil
}

type SmsService interface {
	Allow(ctx context.Context, phone string) error
	SendCode(ctx context.Context, phone string, code string) error
	VerifyCode(ctx context.Context, phone, code string) error
}

type smsService struct {
	client                       *redis.Client
	smsSender                    Sender
	registerCodeTTL              time.Duration
	registerCodeResendInterval   time.Duration
	registerCodePhoneHourlyLimit int
}

var _ SmsService = (*smsService)(nil)

func NewSmsService(
	client *redis.Client,
	smsSender Sender,
	registerCodeTTL time.Duration,
	registerCodeResendInterval time.Duration,
	registerCodePhoneHourlyLimit int) SmsService {
	return &smsService{client: client, smsSender: smsSender, registerCodeTTL: registerCodeTTL, registerCodeResendInterval: registerCodeResendInterval, registerCodePhoneHourlyLimit: registerCodePhoneHourlyLimit}
}

func (s *smsService) Allow(ctx context.Context, phone string) error {
	// 验证60秒冷却
	cooldownKey := common.RedisKeys.RegisterCooldownKey(phone)
	success, err := s.client.SetNX(ctx, cooldownKey, 1, s.registerCodeResendInterval).Result()
	if err != nil {
		return err
	}
	if !success {
		return api.ErrRegisterCodeCooldown
	}

	// 验证一小时限流
	limitKey := common.RedisKeys.RegisterCodeHourlyLimitKey(phone)
	success, err = s.allowSlidingWindow(ctx, limitKey, time.Hour, s.registerCodePhoneHourlyLimit)
	if err != nil {
		return err
	}
	if !success {
		return api.ErrRegisterCodeHourlyLimit
	}

	return nil
}

func (s *smsService) SendCode(ctx context.Context, phone string, code string) error {
	codeKey := common.RedisKeys.RegisterCodeKey(phone)
	if err := s.client.Set(ctx, codeKey, code, s.registerCodeTTL).Err(); err != nil {
		return err
	}
	if err := s.smsSender.Send(ctx, Message{Phone: phone, Text: code}); err != nil {
		return fmt.Errorf("send verification code: %w", err)
	}
	return nil
}

func (s *smsService) VerifyCode(ctx context.Context, phone, code string) error {
	codeKey := common.RedisKeys.RegisterCodeKey(phone)
	expectedCode, err := s.client.Get(ctx, codeKey).Result()
	if errors.Is(err, redis.Nil) {
		return api.ErrRegisterCodeExpired
	}

	if err != nil {
		return fmt.Errorf("get register code: %w", err)
	}

	if subtle.ConstantTimeCompare([]byte(code), []byte(expectedCode)) == 0 {
		return api.ErrRegisterCodeInvalid
	}

	return nil
}

var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]

local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

local cutoff = now - window

redis.call("ZREMRANGEBYSCORE", key, "-inf", cutoff)
local count = redis.call("ZCARD", key)
if count >= limit then 
	return 0
end
redis.call("ZADD", key, now, member)
redis.call("PEXPIRE", key, window)
return 1
`)

func (s *smsService) allowSlidingWindow(ctx context.Context, key string, window time.Duration, limit int) (bool, error) {
	memberBytes := make([]byte, 16)
	if _, err := cryptorand.Read(memberBytes); err != nil {
		return false, err
	}

	member := hex.EncodeToString(memberBytes)
	now := time.Now().UnixMilli()

	result, err := slidingWindowScript.Run(ctx, s.client, []string{key}, now, window.Milliseconds(), limit, member).Int()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}
