package model

import (
	"time"

	"gorm.io/gorm"
)

type UserProfile struct {
	gorm.Model

	UserID uint  `gorm:"not null;uniqueIndex;comment:用户ID"`
	User   *User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Nickname  string    `gorm:"type:varchar(50);not null;comment:昵称"`
	Gender    uint8     `gorm:"type:tinyint unsigned;not null;default:0;comment:0未知 1男 2女"`
	Signature string    `gorm:"type:varchar(200);not null;default:'';comment:个性签名"`
	Birthday  time.Time `gorm:"type:date;not null;default:(CURRENT_DATE);comment:生日"`
	Country   string    `gorm:"type:char(2);not null;default:'';comment:ISO国家或地区代码"`
	Province  string    `gorm:"type:varchar(100);not null;default:'';comment:一级行政区"`

	AvatarMediaID *uint  `gorm:"index;comment:头像媒体ID"`
	AvatarMedia   *Media `gorm:"foreignKey:AvatarMediaID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

func (UserProfile) TableName() string { return "user_profile" }
