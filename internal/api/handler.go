package api

import "github.com/gin-gonic/gin"

type Handler interface {
	RegisterRoutes(group *gin.RouterGroup)
}
