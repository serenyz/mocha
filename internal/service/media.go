package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mmchat/internal/api"
	"mmchat/internal/model"
	"mmchat/internal/repository"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var directUploadTTL = time.Hour
var maxDirectUploadSize int64 = 32 << 20
var getObjectTTL = time.Minute * 15

type CreateMediaUploadCommand struct {
	UserID   uint
	Type     string
	Filename string
	MIMEType string
	Filesize int64
}

type CreateMediaUploadRes struct {
	MediaID  uint
	Method   string
	URL      string
	Headers  map[string]string
	ExpireAt time.Time
}

type CompleteMediaUploadCommand struct {
	UserID  uint
	MediaID uint
}

type CompleteMediaUploadRes struct {
	MediaID      uint
	Type         model.MediaType
	Filename     string
	MIMEType     string
	Size         int64
	Status       string
	URL          string
	URLExpiredAt time.Time
}

type MediaService interface {
	CreateMediaUpload(ctx context.Context, cmd *CreateMediaUploadCommand, res *CreateMediaUploadRes) error
	CompleteMediaUpload(ctx context.Context, cmd *CompleteMediaUploadCommand, res *CompleteMediaUploadRes) error
}

type mediaService struct {
	objs      ObjectStorageService
	mediaRepo repository.MediaRepository
}

func NewMediaService(objs ObjectStorageService, mediaRepo repository.MediaRepository) MediaService {
	return &mediaService{objs: objs, mediaRepo: mediaRepo}
}

var _ MediaService = (*mediaService)(nil)

func (s *mediaService) CreateMediaUpload(ctx context.Context, cmd *CreateMediaUploadCommand, res *CreateMediaUploadRes) error {
	mediaType, err := parseMediaType(cmd.Type)
	if err != nil {
		return err
	}

	mimeType, err := parseMIMEType(cmd.MIMEType)
	if err != nil {
		return err
	}

	if classifyMIMEType(mimeType) != mediaType {
		return api.ErrUnsupportedMediaFormat
	}

	if cmd.Filesize <= 0 {
		return api.ErrInvalidArgument
	}
	if cmd.Filesize > maxDirectUploadSize {
		return api.ErrMediaTooLarge
	}

	filename, err := normalizeMediaFilename(cmd.Filename)
	if err != nil {
		return err
	}

	storageToken, err := newStorageToken()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	storageKey := buildMediaStorageKey(mediaType, storageToken, now)
	upload := &PresignPutObjectRes{}
	if err := s.objs.PresignPutObject(ctx, &PresignPutObjectCommand{
		StorageKey:    storageKey,
		ContentType:   mimeType,
		ContentLength: cmd.Filesize,
		ExpiresIn:     directUploadTTL,
	}, upload); err != nil {
		return err
	}

	headers := upload.Headers
	if headers == nil {
		headers = make(map[string]string)
	}
	media := &model.Media{
		UserID:          &cmd.UserID,
		Type:            mediaType,
		Filename:        filename,
		MIMEType:        mimeType,
		FileSize:        cmd.Filesize,
		Status:          model.MediaStatusPending,
		StorageKey:      storageKey,
		UploadExpiresAt: upload.ExpiresAt,
	}

	if err := s.mediaRepo.Create(ctx, media); err != nil {
		return err
	}

	res.MediaID = media.ID
	res.Method = http.MethodPut
	res.URL = upload.URL
	res.Headers = headers
	res.ExpireAt = upload.ExpiresAt
	return nil
}

func (s *mediaService) CompleteMediaUpload(ctx context.Context, cmd *CompleteMediaUploadCommand, res *CompleteMediaUploadRes) error {
	mediaRecord, err := s.mediaRepo.FindByID(ctx, cmd.MediaID)
	if err != nil {
		return err
	}
	if mediaRecord == nil || mediaRecord.UserID == nil || *mediaRecord.UserID != cmd.UserID {
		return api.ErrMediaNotFound
	}

	res.MediaID = mediaRecord.ID
	res.Type = mediaRecord.Type
	res.Size = mediaRecord.FileSize
	res.Filename = mediaRecord.Filename
	res.MIMEType = mediaRecord.MIMEType
	res.Status = model.MediaStatusUploaded.String()

	if mediaRecord.Status != model.MediaStatusUploaded && mediaRecord.Status != model.MediaStatusPending {
		return api.ErrMediaStatusConflict
	}
	if mediaRecord.Status == model.MediaStatusPending {
		stat := &StatObjectRes{}
		if err := s.objs.StatObject(ctx, mediaRecord.StorageKey, stat); err != nil {
			if errors.Is(err, ErrObjectNotFound) {
				return api.ErrMediaUploadIncomplete
			}
			return err
		}
		if stat.Size != mediaRecord.FileSize {
			return api.ErrMediaSizeMismatch
		}
		if !sameMIMEType(stat.ContentType, mediaRecord.MIMEType) {
			return api.ErrMediaMIMETypeMismatch
		}
		if stat.LastModified.After(mediaRecord.UploadExpiresAt) {
			return api.ErrMediaUploadExpired
		}

		updated, err := s.mediaRepo.MarkUploaded(ctx, mediaRecord.ID, stat.ETag, stat.LastModified)
		if err != nil {
			return err
		}
		if !updated {
			return api.ErrMediaStatusConflict
		}
	}

	presigned := &PresignGetObjectRes{}
	if err := s.objs.PresignGetObject(ctx, &PresignGetObjectCommand{
		StorageKey: mediaRecord.StorageKey,
		ExpireIn:   getObjectTTL,
	}, presigned); err != nil {
		return err
	}
	res.URL = presigned.URL
	res.URLExpiredAt = presigned.ExpiresAt
	return nil
}

func parseMediaType(raw string) (model.MediaType, error) {
	mediaType := model.MediaType(
		strings.ToLower(strings.TrimSpace(raw)),
	)

	switch mediaType {
	case model.MediaTypeImage,
		model.MediaTypeVideo,
		model.MediaTypeAudio,
		model.MediaTypeFile:
		return mediaType, nil
	default:
		return "", api.ErrInvalidMediaType
	}
}

func parseMIMEType(raw string) (string, error) {
	mimeType, params, err := mime.ParseMediaType(
		strings.TrimSpace(raw),
	)
	if err != nil || len(params) != 0 {
		return "", api.ErrUnsupportedMediaFormat
	}

	return strings.ToLower(mimeType), nil
}

func classifyMIMEType(mimeType string) model.MediaType {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return model.MediaTypeImage
	case strings.HasPrefix(mimeType, "video/"):
		return model.MediaTypeVideo
	case strings.HasPrefix(mimeType, "audio/"):
		return model.MediaTypeAudio
	default:
		return model.MediaTypeFile
	}
}

func normalizeMediaFilename(raw string) (string, error) {
	filename := strings.TrimSpace(raw)

	if filename == "" || !utf8.ValidString(filename) || len([]byte(filename)) > 255 || strings.ContainsAny(filename, "/\\") {
		return "", api.ErrInvalidArgument
	}

	for _, value := range filename {
		if unicode.IsControl(value) || unicode.Is(unicode.Cf, value) {
			return "", api.ErrInvalidArgument
		}
	}

	return filename, nil
}

func newStorageToken() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate media storage token: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func buildMediaStorageKey(mediaType model.MediaType, storageToken string, now time.Time) string {
	return path.Join("media", string(mediaType), now.Format("2006/01"), storageToken)
}

func sameMIMEType(actual, expected string) bool {
	mediaType, params, err := mime.ParseMediaType(actual)
	return err == nil && len(params) == 0 && strings.EqualFold(mediaType, expected)
}
