package dto

import "time"

type CreateMediaUploadRequest struct {
	Type     string `json:"type" binding:"required,max=16"`
	Filename string `json:"filename" binding:"required,max=255"`
	MIMEType string `json:"mime_type" binding:"required,max=127"`
	Size     int64  `json:"size" binding:"required,gt=0"`
}

type CreateMediaUploadResponse struct {
	MediaID uint        `json:"media_id"`
	Upload  MediaUpload `json:"upload"`
}

type MediaUpload struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

type CompleteMediaUploadResponse struct {
	MediaID      uint      `json:"media_id"`
	Filename     string    `json:"filename"`
	MIMEType     string    `json:"mime_type"`
	FileSize     int64     `json:"filesize"`
	MediaURL     string    `json:"media_url"`
	URLExpiredAt time.Time `json:"url_expired_at"`
	Status       string    `json:"status"`
}
