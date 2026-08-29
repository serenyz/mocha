package middleware

import "github.com/gin-gonic/gin"

type HandlerFunc func(c *gin.Context) error

func Wrap(handler HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := handler(c); err != nil {
			_ = c.Error(err)
			c.Abort()
		}
	}
}
