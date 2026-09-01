package v1

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"mmchat/internal/api"
	"mmchat/internal/dto"
	"mmchat/internal/middleware"
	"mmchat/internal/service"

	"github.com/gin-gonic/gin"
)

type ConversationHandler struct {
	conversationService service.ConversationService
	messageService      service.MessageService
	authentication      gin.HandlerFunc
}

func NewConversationHandler(
	conversationService service.ConversationService,
	messageService service.MessageService,
	authentication gin.HandlerFunc,
) *ConversationHandler {
	return &ConversationHandler{
		conversationService: conversationService,
		messageService:      messageService,
		authentication:      authentication,
	}
}

func (h *ConversationHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.Use(h.authentication)
	group.POST("/direct", middleware.Wrap(h.createDirectConversation))
	group.POST("/group", middleware.Wrap(h.createGroupConversation))
	group.GET("", middleware.Wrap(h.listConversations))
	group.GET("/:id/messages", middleware.Wrap(h.listMessages))
	group.GET("/:id", middleware.Wrap(h.getConversation))
}

func (h *ConversationHandler) createDirectConversation(c *gin.Context) error {
	var req dto.CreateDirectConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", api.ErrInvalidArgument, err)
	}

	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}

	cmd := &service.CreateDirectConversationCommand{
		CurrentUserID: principal.UserID,
		TargetUserID:  req.UserID,
	}
	res := &service.CreateDirectConversationRes{}

	if err := h.conversationService.CreateDirectConversation(c.Request.Context(), cmd, res); err != nil {
		return err
	}

	api.OK(c, http.StatusOK, dto.CreateDirectConversationResponse{
		ID:   res.ID,
		Type: uint8(res.Type),
	})
	return nil
}

func (h *ConversationHandler) createGroupConversation(c *gin.Context) error {
	var req dto.CreateGroupConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return fmt.Errorf("%w: %v", api.ErrInvalidArgument, err)
	}

	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}

	cmd := &service.CreateGroupConversationCommand{
		CurrentUserID: principal.UserID,
		Name:          req.Name,
		AvatarMediaID: req.AvatarMediaID,
		UserIDs:       req.UserIDs,
	}
	res := &service.CreateGroupConversationRes{}
	if err := h.conversationService.CreateGroupConversation(c.Request.Context(), cmd, res); err != nil {
		return err
	}

	api.OK(c, http.StatusCreated, dto.CreateGroupConversationResponse{
		ID:   res.ID,
		Type: uint8(res.Type),
	})
	return nil
}

func (h *ConversationHandler) getConversation(c *gin.Context) error {
	conversationID, err := parseConversationID(c)
	if err != nil {
		return err
	}

	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}

	cmd := &service.GetConversationCommand{
		UserID:         principal.UserID,
		ConversationID: conversationID,
	}
	res := &service.GetConversationRes{}
	if err := h.conversationService.GetConversation(c.Request.Context(), cmd, res); err != nil {
		return err
	}

	api.OK(c, http.StatusOK, *conversationResponse(&res.ConversationListItem))
	return nil
}

func parseConversationID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 0)
	if err != nil || id == 0 {
		return 0, api.ErrInvalidArgument
	}
	return uint(id), nil
}

func (h *ConversationHandler) listConversations(c *gin.Context) error {
	var req dto.ListConversationsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		return fmt.Errorf("%w: %v", api.ErrInvalidArgument, err)
	}

	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}

	cmd := &service.ListConversationsCommand{
		UserID: principal.UserID,
		Cursor: req.Cursor,
	}
	if req.Limit != nil {
		cmd.Limit = *req.Limit
	}

	res := &service.ListConversationsRes{}
	if err := h.conversationService.ListConversations(c.Request.Context(), cmd, res); err != nil {
		return err
	}

	items := make([]*dto.ConversationListItemResponse, len(res.Items))
	for i, item := range res.Items {
		items[i] = conversationResponse(item)
	}

	api.OK(c, http.StatusOK, dto.ListConversationsResponse{
		Items:      items,
		NextCursor: res.NextCursor,
		HasMore:    res.HasMore,
		Limit:      res.Limit,
	})
	return nil
}

func (h *ConversationHandler) listMessages(c *gin.Context) error {
	conversationID, err := parseConversationID(c)
	if err != nil {
		return err
	}

	var req dto.ListMessagesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		return fmt.Errorf("%w: %v", api.ErrInvalidArgument, err)
	}
	principal, err := middleware.Principal(c)
	if err != nil {
		return err
	}

	cmd := &service.ListMessagesCommand{
		UserID:         principal.UserID,
		ConversationID: conversationID,
		BeforeSeq:      req.BeforeSeq,
		AfterSeq:       req.AfterSeq,
	}
	if req.Limit != nil {
		cmd.Limit = *req.Limit
	}
	res := &service.ListMessagesResult{}
	if err := h.messageService.ListMessages(c.Request.Context(), cmd, res); err != nil {
		return err
	}

	items := make([]dto.MessageResponse, len(res.Items))
	for i := range res.Items {
		items[i] = messageResponse(&res.Items[i])
	}
	api.OK(c, http.StatusOK, dto.ListMessagesResponse{
		Items:         items,
		NextBeforeSeq: res.NextBeforeSeq,
		NextAfterSeq:  res.NextAfterSeq,
		HasMore:       res.HasMore,
		Limit:         res.Limit,
	})
	return nil
}

func messageResponse(item *service.MessageItem) dto.MessageResponse {
	attachments := make([]dto.MessageAttachmentResponse, len(item.Attachments))
	for i := range item.Attachments {
		attachments[i] = dto.MessageAttachmentResponse{
			ID:           item.Attachments[i].ID,
			MediaID:      item.Attachments[i].MediaID,
			Position:     item.Attachments[i].Position,
			Type:         string(item.Attachments[i].Type),
			Filename:     item.Attachments[i].Filename,
			MIMEType:     item.Attachments[i].MIMEType,
			Size:         item.Attachments[i].Size,
			URL:          item.Attachments[i].URL,
			URLExpiredAt: item.Attachments[i].URLExpiredAt,
		}
	}
	return dto.MessageResponse{
		ID:              item.ID,
		ClientMessageID: item.ClientMessageID,
		ConversationID:  item.ConversationID,
		Seq:             item.Seq,
		SenderID:        item.SenderID,
		Type:            uint8(item.Type),
		Text:            item.Text,
		Attachments:     attachments,
		CreatedAt:       item.CreatedAt,
	}
}

func conversationResponse(item *service.ConversationListItem) *dto.ConversationListItemResponse {
	var lastMessage *dto.ConversationMessageSummary
	if item.LastMessage != nil {
		lastMessage = &dto.ConversationMessageSummary{
			ID:        item.LastMessage.ID,
			Seq:       item.LastMessage.Seq,
			SenderID:  item.LastMessage.SenderID,
			Type:      uint8(item.LastMessage.Type),
			Text:      item.LastMessage.Text,
			CreatedAt: item.LastMessage.CreatedAt,
		}
	}

	var group *dto.ConversationGroupResponse
	if item.Group != nil {
		group = &dto.ConversationGroupResponse{
			Name:         item.Group.Name,
			AvatarURL:    item.Group.AvatarURL,
			URLExpiredAt: item.Group.URLExpiredAt,
		}
	}

	peers := make([]*dto.PublicProfile, len(item.Peers))
	for i, peer := range item.Peers {
		peers[i] = &dto.PublicProfile{
			ID:           peer.ID,
			Nickname:     peer.Nickname,
			AvatarURL:    peer.AvatarURL,
			URLExpiredAt: peer.URLExpiredAt,
			Gender:       peer.Gender,
			Signature:    peer.Signature,
			Birthday:     peer.Birthday.Format(time.DateOnly),
			Country:      peer.Country,
			Province:     peer.Province,
		}
	}

	return &dto.ConversationListItemResponse{
		ID:             item.ID,
		Type:           uint8(item.Type),
		Group:          group,
		Peers:          peers,
		MemberCount:    item.MemberCount,
		LastMessage:    lastMessage,
		LastMessageSeq: item.LastMessageSeq,
		JoinedSeq:      item.JoinedSeq,
		LastReadSeq:    item.LastReadSeq,
		UnreadCount:    item.UnreadCount,
	}
}
