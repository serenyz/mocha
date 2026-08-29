package v1

import (
	"fmt"
	"mmchat/internal/api"
	"mmchat/internal/dto"
	"mmchat/internal/middleware"
	"mmchat/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MediaHandler struct {
	mediaService   service.MediaService
	authentication gin.HandlerFunc
}

func NewMediaHandler(mediaService service.MediaService, authentication gin.HandlerFunc) *MediaHandler {
	return &MediaHandler{mediaService: mediaService, authentication: authentication}
}

func (h *MediaHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.Use(h.authentication)
	group.POST("/uploads", middleware.Wrap(h.createUpload))
	group.POST("/uploads/:uuid/complete", middleware.Wrap(h.completeUpload))
}

func (h *MediaHandler) createUpload(c *gin.Context) error {
	var req dto.CreateMediaUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", api.ErrInvalidArgument, err)
	}

	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}

	cmd := &service.CreateMediaUploadCommand{
		UserUUID: principal.UserUUID,
		Type:     req.Type,
		Filename: req.Filename,
		MIMEType: req.MIMEType,
		Filesize: req.Size,
	}
	res := &service.CreateMediaUploadRes{}
	if err := h.mediaService.CreateMediaUpload(c.Request.Context(), cmd, res); err != nil {
		return err
	}

	api.OK[dto.CreateMediaUploadResponse](c, http.StatusOK, dto.CreateMediaUploadResponse{
		MediaUUID: res.MediaUUID,
		Upload: dto.MediaUpload{
			Method:    res.Method,
			URL:       res.URL,
			Headers:   res.Headers,
			ExpiresAt: res.ExpireAt,
		},
	})
	return nil
}

func (h *MediaHandler) completeUpload(c *gin.Context) error {
	mediaUUID, err := uuid.Parse(c.Param("uuid"))
	if err != nil {
		return api.ErrInvalidArgument
	}

	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}

	cmd := &service.CompleteMediaUploadCommand{
		UserUUID:  principal.UserUUID,
		MediaUUID: mediaUUID.String(),
	}
	res := &service.CompleteMediaUploadRes{}

	if err := h.mediaService.CompleteMediaUpload(c.Request.Context(), cmd, res); err != nil {
		return err
	}

	api.OK[dto.CompleteMediaUploadResponse](c, http.StatusOK, dto.CompleteMediaUploadResponse{
		MediaUUID:    res.MediaUUID,
		Filename:     res.Filename,
		MIMEType:     res.MIMEType,
		FileSize:     res.FileSize,
		MediaURL:     res.MediaURL,
		Status:       res.Status,
		URLExpiredAt: res.URLExpiredAt,
	})
	return nil
}
