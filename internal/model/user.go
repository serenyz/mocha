package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model

	Phone        string     `gorm:"type:varchar(20);comment:手机号"`
	ActivePhone  *string    `gorm:"->;type:varchar(20) GENERATED ALWAYS AS (IF(deleted_at IS NULL, phone, NULL)) VIRTUAL;uniqueIndex" json:"-"`
	Email        *string    `gorm:"type:varchar(255);index;comment:邮箱"`
	PasswordHash string     `gorm:"type:varchar(255);not null;comment:密码哈希" json:"-"`
	Role         UserRole   `gorm:"type:tinyint unsigned;not null;default:0;comment:用户角色"`
	Status       UserStatus `gorm:"type:tinyint unsigned;not null;default:0;index;comment:用户角色"`
	LastLoginAt  *time.Time `gorm:"type:datetime;comment:最后登陆时间"`

	Profile UserProfile `gorm:"foreignKey:UserID;references:ID"`
}

type UserProfile struct {
	gorm.Model

	UserID uint  `gorm:"not null;uniqueIndex;comment:用户ID"`
	User   *User `gorm:"foreignKey:UserID;references:ID"`

	Nickname  string    `gorm:"type:varchar(50);not null;comment:昵称"`
	Gender    uint8     `gorm:"type:tinyint unsigned;not null;default:0;comment:0未知 1男 2女"`
	Signature string    `gorm:"type:varchar(200);not null;default:'';comment:个性签名"`
	Birthday  time.Time `gorm:"type:date;not null;default:(CURRENT_DATE);comment:生日"`
	Country   string    `gorm:"type:char(2);not null;default:'';comment:ISO国家或地区代码"`
	Province  string    `gorm:"type:varchar(100);not null;default:'';comment:一级行政区"`

	AvatarMediaID *uint  `gorm:"index;comment:头像媒体ID"`
	AvatarMedia   *Media `gorm:"foreignKey:AvatarMediaID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

type UserRole int8

const (
	UserRoleNormal UserRole = iota
	UserRoleAdmin
)

type UserStatus int8

const (
	UserStatusNormal UserStatus = iota
	UserStatusDisabled
)

func (User) TableName() string        { return "user" }
func (UserProfile) TableName() string { return "user_profile" }
