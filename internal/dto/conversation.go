package dto

import "time"

type CreateDirectConversationRequest struct {
	UserID uint `json:"user_id" binding:"required"`
}

type CreateDirectConversationResponse struct {
	ID   uint  `json:"id"`
	Type uint8 `json:"type"`
}

type CreateGroupConversationRequest struct {
	Name          string `json:"name" binding:"required"`
	AvatarMediaID *uint  `json:"avatar_media_id" binding:"omitempty,gt=0"`
	UserIDs       []uint `json:"user_ids" binding:"omitempty,dive,gt=0"`
}

type CreateGroupConversationResponse = CreateDirectConversationResponse

type ListConversationsRequest struct {
	Cursor *string `form:"cursor"`
	Limit  *int    `form:"limit" binding:"omitempty,min=1,max=50"`
}

type ListConversationsResponse struct {
	Items      []*ConversationListItemResponse `json:"items"`
	NextCursor *string                         `json:"next_cursor"`
	HasMore    bool                            `json:"has_more"`
	Limit      int                             `json:"limit"`
}

type ConversationListItemResponse struct {
	ID             uint                        `json:"id"`
	Type           uint8                       `json:"type"`
	Group          *ConversationGroupResponse  `json:"group,omitempty"`
	Peers          []*PublicProfile            `json:"peers"`
	MemberCount    int                         `json:"member_count"`
	LastMessage    *ConversationMessageSummary `json:"last_message"`
	LastMessageSeq uint64                      `json:"last_message_seq"`
	JoinedSeq      uint64                      `json:"joined_seq"`
	LastReadSeq    uint64                      `json:"last_read_seq"`
	UnreadCount    int64                       `json:"unread_count"`
}

type ConversationGroupResponse struct {
	Name         string    `json:"name"`
	AvatarURL    string    `json:"avatar_url"`
	URLExpiredAt time.Time `json:"url_expired_at"`
}

type ConversationMessageSummary struct {
	ID        uint      `json:"id"`
	Seq       uint64    `json:"seq"`
	SenderID  uint      `json:"sender_id"`
	Type      uint8     `json:"type"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type ListMessagesRequest struct {
	BeforeSeq *uint64 `form:"before_seq"`
	AfterSeq  *uint64 `form:"after_seq"`
	Limit     *int    `form:"limit" binding:"omitempty,min=1,max=100"`
}

type ListMessagesResponse struct {
	Items         []MessageResponse `json:"items"`
	NextBeforeSeq *uint64           `json:"next_before_seq"`
	NextAfterSeq  *uint64           `json:"next_after_seq"`
	HasMore       bool              `json:"has_more"`
	Limit         int               `json:"limit"`
}

type MessageResponse struct {
	ID              uint                        `json:"id"`
	ClientMessageID string                      `json:"client_message_id"`
	ConversationID  uint                        `json:"conversation_id"`
	Seq             uint64                      `json:"seq"`
	SenderID        uint                        `json:"sender_id"`
	Type            uint8                       `json:"type"`
	Text            string                      `json:"text"`
	Attachments     []MessageAttachmentResponse `json:"attachments"`
	CreatedAt       time.Time                   `json:"created_at"`
}

type MessageAttachmentResponse struct {
	ID           uint      `json:"id"`
	MediaID      uint      `json:"media_id"`
	Position     uint16    `json:"position"`
	Type         string    `json:"type"`
	Filename     string    `json:"filename"`
	MIMEType     string    `json:"mime_type"`
	Size         int64     `json:"size"`
	URL          string    `json:"url"`
	URLExpiredAt time.Time `json:"url_expired_at"`
}
