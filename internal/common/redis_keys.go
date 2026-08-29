package common

import "fmt"

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

func (redisKey) AuthSessionKey(sessionID string) string {
	return fmt.Sprintf("%s:%s", authSessionPrefix, sessionID)
}
