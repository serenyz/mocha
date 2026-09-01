package model

import "time"

type ConversationType uint8

const (
	ConversationTypeDirect ConversationType = 1
	ConversationTypeGroup  ConversationType = 2
)

type Conversation struct {
	ID             uint             `gorm:"primaryKey"`
	Type           ConversationType `gorm:"type:tinyint unsigned;not null;check:chk_conversation_type,type IN (1,2);comment:会话类型"`
	LastMessageID  *uint            `gorm:"comment:最后一条消息ID"`
	LastMessageSeq uint64           `gorm:"not null;default:0;comment:最后一条消息序号"`
	LastMessageAt  time.Time        `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;comment:最后一条消息时间"`
	CreatedAt      time.Time        `gorm:"type:datetime;not null"`
	UpdatedAt      time.Time        `gorm:"type:datetime;not null"`
}

func (Conversation) TableName() string { return "conversation" }

type ConversationDirect struct {
	ConversationID uint `gorm:"primaryKey;autoIncrement:false;comment:会话ID"`
	UserLowID      uint `gorm:"not null;uniqueIndex:uk_conversation_direct_users,priority:1;comment:较小用户ID"`
	UserHighID     uint `gorm:"not null;uniqueIndex:uk_conversation_direct_users,priority:2;check:chk_conversation_direct_users,user_low_id <= user_high_id;comment:较大用户ID"`

	Conversation *Conversation `gorm:"foreignKey:ConversationID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

func (ConversationDirect) TableName() string { return "conversation_direct" }

type ConversationGroup struct {
	ConversationID uint      `gorm:"primaryKey;autoIncrement:false;comment:会话ID"`
	Name           string    `gorm:"type:varchar(50);not null;check:chk_conversation_group_name,CHAR_LENGTH(name) BETWEEN 1 AND 50;comment:群名称"`
	AvatarMediaID  *uint     `gorm:"index;comment:群头像媒体ID"`
	UpdatedAt      time.Time `gorm:"type:datetime;not null"`

	Conversation *Conversation `gorm:"foreignKey:ConversationID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	AvatarMedia  *Media        `gorm:"foreignKey:AvatarMediaID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

func (ConversationGroup) TableName() string { return "conversation_group" }

type ConversationMember struct {
	ConversationID uint          `gorm:"primaryKey;autoIncrement:false;index:idx_conversation_member_user,priority:2;comment:会话ID"`
	UserID         uint          `gorm:"primaryKey;autoIncrement:false;index:idx_conversation_member_user,priority:1;comment:用户ID"`
	JoinedSeq      uint64        `gorm:"not null;default:0;comment:加入会话时的消息序号"`
	LastReadSeq    uint64        `gorm:"not null;default:0;comment:最后连续已读消息序号"`
	JoinedAt       time.Time     `gorm:"type:datetime;not null;comment:加入时间"`
	Conversation   *Conversation `gorm:"foreignKey:ConversationID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	User           *User         `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

func (ConversationMember) TableName() string { return "conversation_member" }
