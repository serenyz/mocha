package messaging

import (
	"encoding/json"
	"time"

	"mmchat/internal/model"
)

const (
	MessageOutcomeCommitted = "message.committed"
	MessageOutcomeRejected  = "message.rejected"
	MessagePushCreated      = "message.created"
	MessagePushRejected     = "message.rejected"
	MessagePushDelivered    = "conversation.delivered"
	MessagePushRead         = "conversation.read"
)

type MessageCommand struct {
	ClientMessageID string                     `json:"client_message_id"`
	ConversationID  uint                       `json:"conversation_id"`
	SenderID        uint                       `json:"sender_id"`
	Type            model.MessageType          `json:"type"`
	TextContent     string                     `json:"text"`
	Attachments     []MessageCommandAttachment `json:"attachments"`
	AcceptedAt      time.Time                  `json:"accepted_at"`
}

type MessageCommandAttachment struct {
	MediaID  uint   `json:"media_id"`
	Position uint16 `json:"position"`
}

type MessageCommitted struct {
	ID              uint                         `json:"id"`
	ClientMessageID string                       `json:"client_message_id"`
	ConversationID  uint                         `json:"conversation_id"`
	Seq             uint64                       `json:"seq"`
	SenderID        uint                         `json:"sender_id"`
	Type            model.MessageType            `json:"type"`
	TextContent     string                       `json:"text"`
	Attachments     []MessageCommittedAttachment `json:"attachments"`
	CreatedAt       time.Time                    `json:"created_at"`
}

type MessageCommittedAttachment struct {
	ID           uint            `json:"id"`
	MediaID      uint            `json:"media_id"`
	Position     uint16          `json:"position"`
	Type         model.MediaType `json:"type,omitempty"`
	Filename     string          `json:"filename,omitempty"`
	MIMEType     string          `json:"mime_type,omitempty"`
	Size         int64           `json:"size,omitempty"`
	URL          string          `json:"url,omitempty"`
	URLExpiredAt *time.Time      `json:"url_expired_at,omitempty"`
}

type MessageRejected struct {
	ClientMessageID string    `json:"client_message_id"`
	ConversationID  uint      `json:"conversation_id"`
	SenderID        uint      `json:"sender_id"`
	Code            string    `json:"code"`
	Message         string    `json:"message"`
	RejectedAt      time.Time `json:"rejected_at"`
}

type MessageOutcome struct {
	Type      string            `json:"type"`
	Committed *MessageCommitted `json:"committed,omitempty"`
	Rejected  *MessageRejected  `json:"rejected,omitempty"`
}

type MessagePush struct {
	UserIDs []uint           `json:"user_ids"`
	Event   MessagePushEvent `json:"event"`
}

type MessagePushEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}
