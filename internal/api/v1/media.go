package v1

import (
	"fmt"
	"mmchat/internal/api"
	"mmchat/internal/dto"
	"mmchat/internal/middleware"
	"mmchat/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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
	group.POST("/uploads/:id/complete", middleware.Wrap(h.completeUpload))
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
		UserID:   principal.UserID,
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
		MediaID: res.MediaID,
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
	mediaID, err := strconv.ParseUint(c.Param("id"), 10, 0)
	if err != nil || mediaID == 0 {
		return api.ErrInvalidArgument
	}

	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}

	cmd := &service.CompleteMediaUploadCommand{
		UserID:  principal.UserID,
		MediaID: uint(mediaID),
	}
	res := &service.CompleteMediaUploadRes{}

	if err := h.mediaService.CompleteMediaUpload(c.Request.Context(), cmd, res); err != nil {
		return err
	}

	api.OK[dto.CompleteMediaUploadResponse](c, http.StatusOK, dto.CompleteMediaUploadResponse{
		MediaID:      res.MediaID,
		Filename:     res.Filename,
		MIMEType:     res.MIMEType,
		FileSize:     res.FileSize,
		MediaURL:     res.MediaURL,
		Status:       res.Status,
		URLExpiredAt: res.URLExpiredAt,
	})
	return nil
}
