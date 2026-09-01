package model

import "time"

type MessageType uint8

const MessageTypeNormal MessageType = 1

type Message struct {
	ID              uint        `gorm:"primaryKey"`
	ConversationID  uint        `gorm:"not null;uniqueIndex:uk_message_conversation_seq,priority:1;comment:会话ID"`
	Seq             uint64      `gorm:"not null;uniqueIndex:uk_message_conversation_seq,priority:2;check:chk_message_seq,seq > 0;comment:会话内消息序号"`
	SenderID        uint        `gorm:"not null;uniqueIndex:uk_message_sender_client,priority:1;comment:发送者ID"`
	ClientMessageID string      `gorm:"type:varbinary(64);not null;uniqueIndex:uk_message_sender_client,priority:2;comment:客户端消息ID"`
	Type            MessageType `gorm:"type:tinyint unsigned;not null;check:chk_message_type,type IN (1);comment:消息类型"`
	TextContent     string      `gorm:"type:text;not null;comment:文本内容"`
	CreatedAt       time.Time   `gorm:"type:datetime;not null"`

	Conversation *Conversation       `gorm:"foreignKey:ConversationID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Sender       *User               `gorm:"foreignKey:SenderID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Attachments  []MessageAttachment `gorm:"foreignKey:MessageID;references:ID"`
}

func (Message) TableName() string { return "message" }

type MessageAttachment struct {
	ID        uint      `gorm:"primaryKey"`
	MessageID uint      `gorm:"not null;uniqueIndex:uk_message_attachment_position,priority:1;uniqueIndex:uk_message_attachment_media,priority:1;comment:消息ID"`
	MediaID   uint      `gorm:"not null;index;uniqueIndex:uk_message_attachment_media,priority:2;comment:媒体ID"`
	Position  uint16    `gorm:"type:smallint unsigned;not null;uniqueIndex:uk_message_attachment_position,priority:2;comment:附件顺序"`
	CreatedAt time.Time `gorm:"type:datetime;not null"`

	Message *Message `gorm:"foreignKey:MessageID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Media   *Media   `gorm:"foreignKey:MediaID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}

func (MessageAttachment) TableName() string { return "message_attachment" }
