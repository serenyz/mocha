package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gin-gonic/gin"
)

const RequestIDKey = "request_id"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestId := c.GetHeader("X-Request-ID")
		if requestId == "" || len(requestId) > 64 {
			requestId = newRequestID()
		}
		c.Set(RequestIDKey, requestId)
		c.Header("X-Request-ID", requestId)
		c.Next()
	}
}

func newRequestID() string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err == nil {
		return hex.EncodeToString(data)
	}

	return time.Now().Format("20060102150405.000000000")
}
