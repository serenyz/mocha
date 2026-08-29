package middleware

import (
	"errors"
	"mmchat/internal/api"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}

		err := c.Errors.Last().Err
		if appErr, ok := errors.AsType[*api.AppError](err); ok {
			api.Fail(
				c,
				appErr.Status(),
				appErr.Code(),
				appErr.Message(),
			)
			return
		}

		api.Fail(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"服务器内部错误",
		)
	}
}
