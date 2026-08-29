package service

import (
	"context"
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

	"github.com/google/uuid"
)

var directUploadTTL = time.Hour
var maxDirectUploadSize int64 = 32 << 20
var getObjectTTL = time.Minute * 15

type CreateMediaUploadCommand struct {
	UserUUID string
	Type     string
	Filename string
	MIMEType string
	Filesize int64
}

type CreateMediaUploadRes struct {
	MediaUUID string
	Method    string
	URL       string
	Headers   map[string]string
	ExpireAt  time.Time
}

type CompleteMediaUploadCommand struct {
	UserUUID  string
	MediaUUID string
}

type CompleteMediaUploadRes struct {
	MediaUUID    string
	Filename     string
	MIMEType     string
	FileSize     int64
	Status       string
	MediaURL     string
	URLExpiredAt time.Time
}

type MediaService interface {
	CreateMediaUpload(ctx context.Context, cmd *CreateMediaUploadCommand, res *CreateMediaUploadRes) error
	CompleteMediaUpload(ctx context.Context, cmd *CompleteMediaUploadCommand, res *CompleteMediaUploadRes) error
}

type mediaService struct {
	objs      ObjectStorageService
	userRepo  repository.UserRepository
	mediaRepo repository.MediaRepository
}

func NewMediaService(objs ObjectStorageService, userRepo repository.UserRepository, mediaRepo repository.MediaRepository) MediaService {
	return &mediaService{objs: objs, userRepo: userRepo, mediaRepo: mediaRepo}
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

	user, err := s.userRepo.FindByUUID(ctx, cmd.UserUUID)
	if err != nil {
		return err
	}

	if user == nil {
		return api.ErrUserNotFound
	}

	if user.Status != model.UserStatusNormal {
		return api.ErrAccountDisabled
	}

	mediaUUID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate media uuid: %w", err)
	}

	now := time.Now().UTC()
	mediaUUIDString := mediaUUID.String()
	storageKey := buildMediaStorageKey(mediaType, mediaUUIDString, now)
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
		UUID:            mediaUUIDString,
		UserID:          &user.ID,
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

	res.MediaUUID = media.UUID
	res.Method = http.MethodPut
	res.URL = upload.URL
	res.Headers = headers
	res.ExpireAt = upload.ExpiresAt
	return nil
}

func (s *mediaService) CompleteMediaUpload(ctx context.Context, cmd *CompleteMediaUploadCommand, res *CompleteMediaUploadRes) error {
	mediaRecord, err := s.mediaRepo.FindDetailByUUID(ctx, cmd.MediaUUID)
	if err != nil {
		return err
	}
	if mediaRecord == nil || mediaRecord.User.UUID != cmd.UserUUID {
		return api.ErrMediaNotFound
	}

	res.MediaUUID = mediaRecord.UUID
	res.FileSize = mediaRecord.FileSize
	res.Filename = mediaRecord.Filename
	res.MIMEType = mediaRecord.MIMEType
	res.Status = model.MediaStatusUploaded.String()

	if mediaRecord.Status == model.MediaStatusUploaded {
		ccmd := &PresignGetObjectCommand{
			StorageKey: mediaRecord.StorageKey,
			ExpireIn:   getObjectTTL,
		}
		rres := &PresignGetObjectRes{}
		if err := s.objs.PresignGetObject(ctx, ccmd, rres); err != nil {
			return err
		}
		res.MediaURL = rres.URL
		res.URLExpiredAt = rres.ExpiresAt
		return nil
	}

	if mediaRecord.Status != model.MediaStatusPending {
		return api.ErrMediaStatusConflict
	}

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
	if updated {
		ccmd := &PresignGetObjectCommand{
			StorageKey: mediaRecord.StorageKey,
			ExpireIn:   getObjectTTL,
		}
		rres := &PresignGetObjectRes{}
		if err := s.objs.PresignGetObject(ctx, ccmd, rres); err != nil {
			return err
		}
		res.MediaURL = rres.URL
		res.URLExpiredAt = rres.ExpiresAt
		return nil
	}

	return api.ErrMediaStatusConflict
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

func buildMediaStorageKey(mediaType model.MediaType, mediaUUID string, now time.Time) string {
	return path.Join("media", string(mediaType), now.Format("2006/01"), mediaUUID)
}

func sameMIMEType(actual, expected string) bool {
	mediaType, params, err := mime.ParseMediaType(actual)
	return err == nil && len(params) == 0 && strings.EqualFold(mediaType, expected)
}
