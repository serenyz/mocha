package model

import (
	"time"

	"gorm.io/gorm"
)

type Media struct {
	gorm.Model

	UserID *uint `gorm:"index;comment:所属用户ID"`
	User   *User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	Type       MediaType   `gorm:"type:varchar(16);not null;index;comment:媒体类型"`
	Filename   string      `gorm:"type:varchar(255);not null;comment:原始文件名"`
	MIMEType   string      `gorm:"type:varchar(127);not null;comment:MIME类型"`
	FileSize   int64       `gorm:"not null;comment:声明文件大小"`
	Status     MediaStatus `gorm:"type:tinyint unsigned;not null;default:0;index;comment:媒体状态"`
	StorageKey string      `gorm:"type:varchar(512);not null;uniqueIndex;comment:存储键"`
	ETag       string      `gorm:"column:etag;type:varchar(128);not null;default:'';comment:对象存储ETag"`

	UploadExpiresAt time.Time  `gorm:"not null;index;comment:上传申请过期时间"`
	UploadedAt      *time.Time `gorm:"comment:上传完成时间"`
}

type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
	MediaTypeAudio MediaType = "audio"
	MediaTypeFile  MediaType = "file"
)

type MediaStatus uint8

const (
	MediaStatusPending MediaStatus = iota
	MediaStatusUploaded
)

func (s MediaStatus) String() string {
	switch s {
	case MediaStatusPending:
		return "pending"
	case MediaStatusUploaded:
		return "uploaded"
	default:
		return "unknown"
	}
}

func (Media) TableName() string {
	return "media"
}
