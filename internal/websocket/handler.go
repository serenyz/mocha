package websocket

import (
	"context"
	"encoding/json"
	"errors"

	"mmchat/internal/api"
	"mmchat/internal/messaging"
	"mmchat/internal/service"
	"mmchat/internal/zlog"

	"go.uber.org/zap"
)

const (
	EventMessageSend         = "message.send"
	EventMessageAccepted     = "message.accepted"
	ErrorInvalidMessage      = "INVALID_MESSAGE"
	ErrorMessageUnavailable  = "MESSAGE_UNAVAILABLE"
	ErrorInvalidProgress     = "INVALID_MESSAGE_PROGRESS"
	ErrorProgressUnavailable = "MESSAGE_PROGRESS_UNAVAILABLE"
)

type EventHandler interface {
	Handle(ctx context.Context, client *Client, incoming IncomingEvent)
}

type messageEventHandler struct {
	messageService service.MessageService
}

type messageSendData struct {
	ClientMessageID string `json:"client_message_id"`
	ConversationID  uint   `json:"conversation_id"`
	Text            string `json:"text"`
	MediaIDs        []uint `json:"media_ids"`
}

type messageEventReference struct {
	ClientMessageID string `json:"client_message_id,omitempty"`
	ConversationID  uint   `json:"conversation_id,omitempty"`
}

type messageProgressData struct {
	ConversationID uint   `json:"conversation_id"`
	Seq            uint64 `json:"seq"`
}

func NewMessageEventHandler(messageService service.MessageService) EventHandler {
	if messageService == nil {
		return nil
	}
	return &messageEventHandler{messageService: messageService}
}

func (h *messageEventHandler) Handle(
	ctx context.Context,
	client *Client,
	incoming IncomingEvent,
) {
	switch incoming.Type {
	case EventMessageSend:
		h.handleSend(client, incoming.Data)
	case messaging.MessagePushDelivered:
		h.handlePushDelivered(ctx, client, incoming.Data)
	case messaging.MessagePushRead:
		h.handlePushRead(ctx, client, incoming.Data)
	default:
		client.Send(NewErrorEvent(ErrorEventUnsupported, "暂不支持事件 "+incoming.Type))
	}
}

func (h *messageEventHandler) handleSend(client *Client, raw json.RawMessage) {
	var data messageSendData
	if json.Unmarshal(raw, &data) != nil {
		client.Send(NewErrorEvent(ErrorInvalidMessage, "消息参数不正确"))
		return
	}

	reference := messageEventReference{
		ClientMessageID: data.ClientMessageID,
		ConversationID:  data.ConversationID,
	}
	err := h.messageService.Send(&service.SendMessageCommand{
		ClientMessageID: data.ClientMessageID,
		ConversationID:  data.ConversationID,
		SenderID:        client.UserID(),
		Text:            data.Text,
		MediaIDs:        data.MediaIDs,
	}, func(result *service.SendMessageResult, err error) {
		if err != nil {
			zlog.Error(
				"publish message command",
				zap.Uint("user_id", client.UserID()),
				zap.Uint("conversation_id", data.ConversationID),
				zap.String("client_message_id", data.ClientMessageID),
				zap.Error(err),
			)
			client.Send(NewErrorEventWithData(
				ErrorMessageUnavailable,
				"消息暂时无法发送，请重试",
				reference,
			))
			return
		}
		client.Send(Event{Type: EventMessageAccepted, Data: result})
	})
	if err != nil {
		client.Send(NewErrorEventWithData(
			ErrorInvalidMessage,
			err.Error(),
			reference.data(),
		))
	}
}

func (h *messageEventHandler) handlePushDelivered(ctx context.Context, client *Client, raw json.RawMessage) {
	var data messageProgressData
	if json.Unmarshal(raw, &data) != nil {
		client.Send(NewErrorEventWithData(ErrorInvalidProgress, "消息送达参数不正确", data))
		return
	}

	result := &service.MessageProgressResult{}
	err := h.messageService.Delivered(
		ctx,
		&service.MessageProgressCommand{
			ConversationID: data.ConversationID,
			UserID:         client.UserID(),
			Seq:            data.Seq,
		},
		result,
	)
	if err == nil {
		client.Send(Event{
			Type: messaging.MessagePushDelivered,
			Data: result,
		})
		return
	}

	if appErr, ok := errors.AsType[*api.AppError](err); ok {
		client.Send(NewErrorEventWithData(
			appErr.Code(),
			appErr.Message(),
			data,
		))
		return
	}

	zlog.Error(
		"report message delivered",
		zap.Uint("user_id", client.UserID()),
		zap.Uint("conversation_id", data.ConversationID),
		zap.Uint64("seq", data.Seq),
		zap.Error(err),
	)
	client.Send(NewErrorEventWithData(
		ErrorProgressUnavailable,
		"消息送达状态暂时无法更新，请重试",
		data,
	))
}

func (h *messageEventHandler) handlePushRead(ctx context.Context, client *Client, raw json.RawMessage) {
	var data messageProgressData
	if json.Unmarshal(raw, &data) != nil {
		client.Send(NewErrorEventWithData(
			ErrorInvalidProgress,
			"消息已读参数不正确",
			data,
		))
		return
	}

	result := &service.MessageProgressResult{}
	err := h.messageService.Read(
		ctx,
		&service.MessageProgressCommand{
			ConversationID: data.ConversationID,
			UserID:         client.UserID(),
			Seq:            data.Seq,
		},
		result,
	)
	if err == nil {
		client.Send(Event{
			Type: messaging.MessagePushRead,
			Data: result,
		})
		return
	}

	if appErr, ok := errors.AsType[*api.AppError](err); ok {
		client.Send(NewErrorEventWithData(
			appErr.Code(),
			appErr.Message(),
			data,
		))
		return
	}

	zlog.Error(
		"report message read",
		zap.Uint("user_id", client.UserID()),
		zap.Uint("conversation_id", data.ConversationID),
		zap.Uint64("seq", data.Seq),
		zap.Error(err),
	)
	client.Send(NewErrorEventWithData(
		ErrorProgressUnavailable,
		"消息已读状态暂时无法更新，请重试",
		data,
	))
}

func (r messageEventReference) data() any {
	if r.ClientMessageID == "" && r.ConversationID == 0 {
		return nil
	}
	return r
}
