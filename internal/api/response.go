package api

import (
	"github.com/gin-gonic/gin"
)

type Response[T any] struct {
	Success bool        `json:"success"`
	Data    *T          `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
	Meta    *CursorMata `json:"meta,omitempty"`
}

func OK[T any](c *gin.Context, status int, data T) {
	c.JSON(status, Response[T]{
		Success: true,
		Data:    &data,
	})
}

func OKWithPage[T any](c *gin.Context, status int, data T, meta CursorMata) {
	c.JSON(status, Response[T]{
		Success: true,
		Data:    &data,
		Meta:    &meta,
	})
}

func Success(c *gin.Context, status int) {
	c.JSON(status, Response[any]{Success: true})
}

func Fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, Response[any]{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	})
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CursorMata struct {
	NextCursor *uint `json:"next_cursor"`
	HasMore    bool  `json:"has_more"`
	Limit      int   `json:"limit"`
}
