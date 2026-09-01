package common

import (
	"crypto/sha256"
	"fmt"
)

type redisKey struct{}

var RedisKeys = redisKey{}

const registerVerificationPrefix = "mocha:verification:register"

func (redisKey) RegisterCodeKey(phone string) string {
	return fmt.Sprintf("%s:{%s}:%s", registerVerificationPrefix, phone, "code")
}

func (redisKey) RegisterCooldownKey(phone string) string {
	return fmt.Sprintf("%s:{%s}:%s", registerVerificationPrefix, phone, "cooldown")
}

func (redisKey) RegisterCodeHourlyLimitKey(phone string) string {
	return fmt.Sprintf("%s:{%s}:%s", registerVerificationPrefix, phone, "hour")
}

const authSessionPrefix = "mocha:auth:session"

func (redisKey) AuthSessionKey(userID uint) string {
	return fmt.Sprintf("%s:%d", authSessionPrefix, userID)
}

const webSocketTicketPrefix = "mocha:ws:ticket"

func (redisKey) WebSocketTicketKey(hash string) string {
	return webSocketTicketPrefix + ":" + hash
}

const presignGetObjectPrefix = "mocha:object:presign:get"

func (redisKey) PresignGetObjectKey(bucket, storageKey string) string {
	value := bucket + "\x00" + storageKey
	return fmt.Sprintf("%s:%x", presignGetObjectPrefix, sha256.Sum256([]byte(value)))
}
