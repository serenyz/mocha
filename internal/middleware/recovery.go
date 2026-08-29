package middleware

import (
	"fmt"
	"mmchat/internal/api"
	"mmchat/internal/zlog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			c.Abort()

			zlog.Error(
				"panic recovered",
				zap.String("request_id", c.GetString(RequestIDKey)),
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.String("panic", fmt.Sprint(recovered)),
				zap.ByteString("stack", debug.Stack()),
			)

			if c.Writer.Written() {
				return
			}

			api.Fail(
				c,
				http.StatusInternalServerError,
				"INTERNAL_ERROR",
				"服务器内部错误",
			)
		}()

		c.Next()
	}
}
