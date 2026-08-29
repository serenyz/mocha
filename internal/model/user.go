package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model

	Phone        *string    `gorm:"type:varchar(20);uniqueIndex;comment:手机号"`
	Email        *string    `gorm:"type:varchar(255);uniqueIndex;comment:邮箱"`
	PasswordHash string     `gorm:"type:varchar(255);not null;comment:密码哈希" json:"-"`
	Role         UserRole   `gorm:"type:tinyint unsigned;not null;default:0;comment:用户角色"`
	Status       UserStatus `gorm:"type:tinyint unsigned;not null;default:0;index;comment:用户角色"`
	LastLoginAt  *time.Time `gorm:"type:datetime;comment:最后登陆时间"`

	Profile UserProfile `gorm:"foreignKey:UserID;references:ID"`
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

func (User) TableName() string { return "user" }
