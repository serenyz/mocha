package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"mmchat/internal/api"
	"mmchat/internal/messaging"
	"mmchat/internal/model"
	"mmchat/internal/repository"
)

const (
	defaultMessageListLimit = 50
	maxMessageListLimit     = 100
)

var (
	ErrInvalidMessage      = errors.New("消息参数不正确")
	ErrEmptyMessage        = errors.New("消息内容不能为空")
	ErrInvalidAttachment   = errors.New("附件参数不正确")
	ErrDuplicateAttachment = errors.New("附件不能重复")
	ErrTooManyAttachments  = errors.New("附件数量过多")
)

type SendMessageCommand struct {
	ClientMessageID string
	ConversationID  uint
	SenderID        uint
	Text            string
	MediaIDs        []uint
}

type SendMessageResult struct {
	ClientMessageID string    `json:"client_message_id"`
	ConversationID  uint      `json:"conversation_id"`
	AcceptedAt      time.Time `json:"accepted_at"`
}

type MessageProgressCommand struct {
	ConversationID uint
	UserID         uint
	Seq            uint64
}

type MessageProgressResult struct {
	ConversationID uint      `json:"conversation_id"`
	UserID         uint      `json:"user_id"`
	Seq            uint64    `json:"seq"`
	ReportedAt     time.Time `json:"reported_at"`
}

type ListMessagesCommand struct {
	UserID         uint
	ConversationID uint
	BeforeSeq      *uint64
	AfterSeq       *uint64
	Limit          int
}

type ListMessagesResult struct {
	Items         []MessageItem
	NextBeforeSeq *uint64
	NextAfterSeq  *uint64
	HasMore       bool
	Limit         int
}

type MessageItem struct {
	ID              uint
	ClientMessageID string
	ConversationID  uint
	Seq             uint64
	SenderID        uint
	Type            model.MessageType
	Text            string
	Attachments     []MessageAttachmentItem
	CreatedAt       time.Time
}

type MessageAttachmentItem struct {
	ID           uint
	MediaID      uint
	Position     uint16
	Type         model.MediaType
	Filename     string
	MIMEType     string
	Size         int64
	URL          string
	URLExpiredAt time.Time
}

type MessageService interface {
	Send(cmd *SendMessageCommand, callback func(result *SendMessageResult, err error)) error
	ListMessages(ctx context.Context, cmd *ListMessagesCommand, res *ListMessagesResult) error
	Delivered(ctx context.Context, cmd *MessageProgressCommand, res *MessageProgressResult) error
	Read(ctx context.Context, cmd *MessageProgressCommand, res *MessageProgressResult) error
	Process(ctx context.Context, commands []messaging.MessageCommand) ([]messaging.MessageOutcome, error)
	Build(ctx context.Context, outcomes []messaging.MessageOutcome) ([]messaging.MessagePush, error)
}

type messageService struct {
	commandPublisher messaging.MessageCommandPublisher
	pushPublisher    messaging.MessagePushPublisher
	messageRepo      repository.MessageRepository
	conversationRepo repository.ConversationRepository
	mediaRepo        repository.MediaRepository
	objs             ObjectStorageService
}

var _ MessageService = (*messageService)(nil)

func NewMessageService(
	commandPublisher messaging.MessageCommandPublisher,
	pushPublisher messaging.MessagePushPublisher,
	messageRepo repository.MessageRepository,
	conversationRepo repository.ConversationRepository,
	mediaRepo repository.MediaRepository,
	objs ObjectStorageService,
) MessageService {
	return &messageService{
		commandPublisher: commandPublisher,
		pushPublisher:    pushPublisher,
		messageRepo:      messageRepo,
		conversationRepo: conversationRepo,
		mediaRepo:        mediaRepo,
		objs:             objs,
	}
}

func (s *messageService) Send(cmd *SendMessageCommand, callback func(result *SendMessageResult, err error)) error {
	command, err := newMessageCommand(cmd)
	if err != nil {
		return err
	}
	if callback == nil {
		callback = func(*SendMessageResult, error) {}
	}

	s.commandPublisher.Publish(command, func(err error) {
		if err != nil {
			callback(nil, err)
			return
		}
		callback(&SendMessageResult{
			ClientMessageID: command.ClientMessageID,
			ConversationID:  command.ConversationID,
			AcceptedAt:      command.AcceptedAt,
		}, nil)
	})
	return nil
}

func (s *messageService) Delivered(
	ctx context.Context,
	cmd *MessageProgressCommand,
	res *MessageProgressResult,
) error {
	if res == nil {
		return api.ErrInvalidArgument
	}
	if _, err := s.validateMessageProgress(ctx, cmd); err != nil {
		return err
	}

	*res = MessageProgressResult{
		ConversationID: cmd.ConversationID,
		UserID:         cmd.UserID,
		Seq:            cmd.Seq,
		ReportedAt:     time.Now().UTC(),
	}
	return s.publishMessageProgress(ctx, messaging.MessagePushDelivered, res)
}

func (s *messageService) Read(
	ctx context.Context,
	cmd *MessageProgressCommand,
	res *MessageProgressResult,
) error {
	if res == nil {
		return api.ErrInvalidArgument
	}
	membership, err := s.validateMessageProgress(ctx, cmd)
	if err != nil {
		return err
	}

	readSeq := max(cmd.Seq, membership.LastReadSeq)
	if cmd.Seq > membership.LastReadSeq {
		advanced, err := s.conversationRepo.AdvanceLastReadSeq(ctx, cmd.ConversationID, cmd.UserID, cmd.Seq)
		if err != nil {
			return err
		}
		if !advanced {
			membership, err = s.conversationRepo.FindMember(ctx, cmd.ConversationID, cmd.UserID)
			if err != nil {
				return err
			}
			if membership == nil {
				return api.ErrConversationNotFound
			}
			readSeq = membership.LastReadSeq
		}
	}

	*res = MessageProgressResult{
		ConversationID: cmd.ConversationID,
		UserID:         cmd.UserID,
		Seq:            readSeq,
		ReportedAt:     time.Now().UTC(),
	}
	return s.publishMessageProgress(ctx, messaging.MessagePushRead, res)
}

func (s *messageService) validateMessageProgress(
	ctx context.Context,
	cmd *MessageProgressCommand,
) (*model.ConversationMember, error) {
	if cmd == nil || cmd.ConversationID == 0 || cmd.UserID == 0 || cmd.Seq == 0 {
		return nil, api.ErrInvalidArgument
	}

	membership, err := s.conversationRepo.FindMember(ctx, cmd.ConversationID, cmd.UserID)
	if err != nil {
		return nil, err
	}
	if membership == nil {
		return nil, api.ErrConversationNotFound
	}
	if membership.Conversation == nil {
		return nil, fmt.Errorf("conversation %d was not loaded", cmd.ConversationID)
	}
	if cmd.Seq <= membership.JoinedSeq || cmd.Seq > membership.Conversation.LastMessageSeq {
		return nil, api.ErrMessageNotFound
	}
	return membership, nil
}

func (s *messageService) publishMessageProgress(
	ctx context.Context,
	eventType string,
	result *MessageProgressResult,
) error {
	members, err := s.conversationRepo.ListMembers(ctx, []uint{result.ConversationID})
	if err != nil {
		return err
	}

	userIDs := make([]uint, 0, len(members))
	found := false
	for _, member := range members {
		if member.UserID == result.UserID {
			found = true
			continue
		}
		if result.Seq > member.JoinedSeq {
			userIDs = append(userIDs, member.UserID)
		}
	}
	if !found {
		return api.ErrConversationNotFound
	}
	if len(userIDs) == 0 {
		return nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal message progress: %w", err)
	}
	return s.pushPublisher.Publish(ctx, []messaging.MessagePush{{
		UserIDs: userIDs,
		Event: messaging.MessagePushEvent{
			Type: eventType,
			Data: data,
		},
	}})
}

func (s *messageService) ListMessages(
	ctx context.Context,
	cmd *ListMessagesCommand,
	res *ListMessagesResult,
) error {
	if cmd == nil || res == nil || cmd.UserID == 0 || cmd.ConversationID == 0 {
		return api.ErrInvalidArgument
	}
	if cmd.BeforeSeq != nil && cmd.AfterSeq != nil {
		return api.ErrInvalidArgument
	}

	res.Limit = cmd.Limit
	if res.Limit == 0 {
		res.Limit = defaultMessageListLimit
	}
	if res.Limit < 1 || res.Limit > maxMessageListLimit {
		return api.ErrInvalidArgument
	}

	membership, err := s.conversationRepo.FindMember(ctx, cmd.ConversationID, cmd.UserID)
	if err != nil {
		return err
	}
	if membership == nil {
		return api.ErrConversationNotFound
	}

	messages, err := s.listMessages(ctx, cmd, membership.JoinedSeq, res.Limit+1)
	if err != nil {
		return err
	}
	res.HasMore = len(messages) > res.Limit
	if res.HasMore && cmd.AfterSeq != nil {
		messages = messages[:res.Limit]
	}
	if res.HasMore && cmd.AfterSeq == nil {
		messages = messages[1:]
	}
	mediaItems, err := s.loadMessageAttachments(ctx, messages)
	if err != nil {
		return err
	}

	res.Items, err = messageItems(messages, mediaItems)
	if err != nil {
		return err
	}
	if res.HasMore && len(messages) > 0 && cmd.AfterSeq != nil {
		res.NextAfterSeq = new(messages[len(messages)-1].Seq)
	}
	if res.HasMore && len(messages) > 0 && cmd.AfterSeq == nil {
		res.NextBeforeSeq = new(messages[0].Seq)
	}
	return nil
}

func (s *messageService) listMessages(
	ctx context.Context,
	cmd *ListMessagesCommand,
	joinedSeq uint64,
	limit int,
) ([]model.Message, error) {
	if cmd.AfterSeq != nil {
		afterSeq := max(*cmd.AfterSeq, joinedSeq)
		return s.messageRepo.ListAfterSeq(ctx, cmd.ConversationID, afterSeq, limit)
	}
	if cmd.BeforeSeq != nil {
		return s.messageRepo.ListBeforeSeq(ctx, cmd.ConversationID, joinedSeq, *cmd.BeforeSeq, limit)
	}
	return s.messageRepo.ListLatest(ctx, cmd.ConversationID, joinedSeq, limit)
}

func (s *messageService) loadMessageAttachments(
	ctx context.Context,
	messages []model.Message,
) (map[uint]MessageAttachmentItem, error) {
	messageIDs := make([]uint, len(messages))
	messagesByID := make(map[uint]*model.Message, len(messages))
	for i := range messages {
		messageIDs[i] = messages[i].ID
		messagesByID[messages[i].ID] = &messages[i]
	}

	attachments, err := s.messageRepo.ListAttachmentsByMessageIDs(ctx, messageIDs)
	if err != nil {
		return nil, err
	}
	mediaIDs := make([]uint, 0, len(attachments))
	seenMedia := make(map[uint]struct{}, len(attachments))
	for i := range attachments {
		message := messagesByID[attachments[i].MessageID]
		if message == nil {
			return nil, fmt.Errorf("attachment message %d was not found", attachments[i].MessageID)
		}
		message.Attachments = append(message.Attachments, attachments[i])
		if _, exists := seenMedia[attachments[i].MediaID]; !exists {
			seenMedia[attachments[i].MediaID] = struct{}{}
			mediaIDs = append(mediaIDs, attachments[i].MediaID)
		}
	}
	return s.loadMessageMedia(ctx, mediaIDs)
}

func (s *messageService) loadMessageMedia(
	ctx context.Context,
	mediaIDs []uint,
) (map[uint]MessageAttachmentItem, error) {
	media, err := s.mediaRepo.ListByIDs(ctx, mediaIDs)
	if err != nil {
		return nil, err
	}
	items := make(map[uint]MessageAttachmentItem, len(media))
	for i := range media {
		if media[i].Status != model.MediaStatusUploaded {
			continue
		}
		presigned := &PresignGetObjectRes{}
		if err := s.objs.PresignGetObject(ctx, &PresignGetObjectCommand{
			StorageKey: media[i].StorageKey,
			ExpireIn:   getObjectTTL,
		}, presigned); err != nil {
			return nil, err
		}
		items[media[i].ID] = MessageAttachmentItem{
			MediaID:      media[i].ID,
			Type:         media[i].Type,
			Filename:     media[i].Filename,
			MIMEType:     media[i].MIMEType,
			Size:         media[i].FileSize,
			URL:          presigned.URL,
			URLExpiredAt: presigned.ExpiresAt,
		}
	}
	for _, mediaID := range mediaIDs {
		if _, exists := items[mediaID]; !exists {
			return nil, fmt.Errorf("message attachment media %d was not found", mediaID)
		}
	}
	return items, nil
}

func messageItems(
	messages []model.Message,
	mediaItems map[uint]MessageAttachmentItem,
) ([]MessageItem, error) {
	items := make([]MessageItem, len(messages))
	for i := range messages {
		message := &messages[i]
		attachments := make([]MessageAttachmentItem, len(message.Attachments))
		for j := range message.Attachments {
			attachment := &message.Attachments[j]
			mediaItem, exists := mediaItems[attachment.MediaID]
			if !exists {
				return nil, fmt.Errorf("message attachment media %d was not loaded", attachment.MediaID)
			}
			mediaItem.ID = attachment.ID
			mediaItem.Position = attachment.Position
			attachments[j] = mediaItem
		}
		items[i] = MessageItem{
			ID:              message.ID,
			ClientMessageID: message.ClientMessageID,
			ConversationID:  message.ConversationID,
			Seq:             message.Seq,
			SenderID:        message.SenderID,
			Type:            message.Type,
			Text:            message.TextContent,
			Attachments:     attachments,
			CreatedAt:       message.CreatedAt,
		}
	}
	return items, nil
}

func newMessageCommand(cmd *SendMessageCommand) (*messaging.MessageCommand, error) {
	if cmd == nil || cmd.SenderID == 0 || cmd.ConversationID == 0 {
		return nil, ErrInvalidMessage
	}
	if !validClientMessageID(cmd.ClientMessageID) {
		return nil, ErrInvalidMessage
	}
	if strings.TrimSpace(cmd.Text) == "" && len(cmd.MediaIDs) == 0 {
		return nil, ErrEmptyMessage
	}
	if len(cmd.MediaIDs) > math.MaxUint16+1 {
		return nil, ErrTooManyAttachments
	}

	attachments := make([]messaging.MessageCommandAttachment, len(cmd.MediaIDs))
	mediaIDs := make(map[uint]struct{}, len(cmd.MediaIDs))
	for i, mediaID := range cmd.MediaIDs {
		if mediaID == 0 {
			return nil, ErrInvalidAttachment
		}
		if _, exists := mediaIDs[mediaID]; exists {
			return nil, ErrDuplicateAttachment
		}
		mediaIDs[mediaID] = struct{}{}
		attachments[i] = messaging.MessageCommandAttachment{
			MediaID:  mediaID,
			Position: uint16(i),
		}
	}

	return &messaging.MessageCommand{
		ClientMessageID: cmd.ClientMessageID,
		ConversationID:  cmd.ConversationID,
		SenderID:        cmd.SenderID,
		Type:            model.MessageTypeNormal,
		TextContent:     cmd.Text,
		Attachments:     attachments,
		AcceptedAt:      time.Now().UTC(),
	}, nil
}

func validClientMessageID(value string) bool {
	return value != "" && len(value) <= 64 && strings.TrimSpace(value) == value
}

const (
	messageRejectInvalid      = "INVALID_MESSAGE"
	messageRejectConversation = "CONVERSATION_NOT_FOUND"
	messageRejectMedia        = "MEDIA_NOT_FOUND"
	messageRejectConflict     = "CLIENT_MESSAGE_ID_CONFLICT"
)

type messageWriterKey struct {
	senderID        uint
	clientMessageID string
}

// Process 将每条消息命令转换为 committed 或 rejected 结果。
func (s *messageService) Process(ctx context.Context, commands []messaging.MessageCommand) ([]messaging.MessageOutcome, error) {
	outcomes := make([]messaging.MessageOutcome, len(commands))
	// 校验命令，并合并当前批次内 client_message_id 相同的重复消息。
	canonical, duplicates := classifyMessageCommands(commands, outcomes)

	// Kafka 可能重复投递；数据库已有相同消息时直接复用，不能再次插入。
	existing, err := s.loadExistingMessages(ctx, commands, canonical)
	if err != nil {
		return nil, err
	}

	pending := make([]int, 0, len(canonical))
	for _, index := range canonical {
		command := &commands[index]
		stored := existing[writerKey(command)]
		if stored == nil {
			pending = append(pending, index)
			continue
		}
		if !sameStoredMessage(stored, command) {
			outcomes[index] = rejectedOutcome(command, messageRejectConflict, "client_message_id 已用于其他消息")
			continue
		}
		outcomes[index] = committedOutcome(stored)
	}

	// 只有尚未落库的消息才需要继续检查会话成员和附件。
	pending, err = s.validateMemberships(ctx, commands, pending, outcomes)
	if err != nil {
		return nil, err
	}
	pending, err = s.validateMedia(ctx, commands, pending, outcomes)
	if err != nil {
		return nil, err
	}
	if err := s.createMessages(ctx, commands, pending, outcomes); err != nil {
		return nil, err
	}

	// 内容相同的重复消息复用第一条消息的处理结果。
	for index, canonicalIndex := range duplicates {
		if canonicalIndex >= 0 {
			outcomes[index] = outcomes[canonicalIndex]
		}
	}
	for i := range outcomes {
		if outcomes[i].Type == "" {
			return nil, fmt.Errorf("message outcome %d was not created", i)
		}
	}
	return outcomes, nil
}

func classifyMessageCommands(
	commands []messaging.MessageCommand,
	outcomes []messaging.MessageOutcome,
) ([]int, []int) {
	canonicalByKey := make(map[messageWriterKey]int, len(commands))
	canonical := make([]int, 0, len(commands))
	duplicates := make([]int, len(commands))
	for i := range duplicates {
		duplicates[i] = -1
	}

	for i := range commands {
		command := &commands[i]
		if err := validateMessageCommand(command); err != nil {
			outcomes[i] = rejectedOutcome(command, messageRejectInvalid, err.Error())
			continue
		}

		key := writerKey(command)
		canonicalIndex, exists := canonicalByKey[key]
		if !exists {
			canonicalByKey[key] = i
			canonical = append(canonical, i)
			continue
		}
		if sameCommandPayload(&commands[canonicalIndex], command) {
			duplicates[i] = canonicalIndex
			continue
		}
		outcomes[i] = rejectedOutcome(command, messageRejectConflict, "client_message_id 已用于其他消息")
	}
	return canonical, duplicates
}

func (s *messageService) loadExistingMessages(
	ctx context.Context,
	commands []messaging.MessageCommand,
	indices []int,
) (map[messageWriterKey]*model.Message, error) {
	keys := make([]repository.MessageClientKey, len(indices))
	for i, index := range indices {
		keys[i] = repository.MessageClientKey{
			SenderID:        commands[index].SenderID,
			ClientMessageID: commands[index].ClientMessageID,
		}
	}

	messages, err := s.messageRepo.ListByClientIDs(ctx, keys)
	if err != nil {
		return nil, err
	}
	messageIDs := make([]uint, len(messages))
	for i := range messages {
		messageIDs[i] = messages[i].ID
	}
	attachments, err := s.messageRepo.ListAttachmentsByMessageIDs(ctx, messageIDs)
	if err != nil {
		return nil, err
	}

	messageByID := make(map[uint]*model.Message, len(messages))
	existing := make(map[messageWriterKey]*model.Message, len(messages))
	for i := range messages {
		message := &messages[i]
		messageByID[message.ID] = message
		existing[messageWriterKey{
			senderID:        message.SenderID,
			clientMessageID: message.ClientMessageID,
		}] = message
	}
	for i := range attachments {
		message := messageByID[attachments[i].MessageID]
		if message == nil {
			return nil, errors.New("attachment message was not found")
		}
		message.Attachments = append(message.Attachments, attachments[i])
	}
	return existing, nil
}

func (s *messageService) validateMemberships(
	ctx context.Context,
	commands []messaging.MessageCommand,
	indices []int,
	outcomes []messaging.MessageOutcome,
) ([]int, error) {
	keys := make([]repository.ConversationMemberKey, 0, len(indices))
	seen := make(map[repository.ConversationMemberKey]struct{}, len(indices))
	for _, index := range indices {
		key := repository.ConversationMemberKey{
			ConversationID: commands[index].ConversationID,
			UserID:         commands[index].SenderID,
		}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}

	members, err := s.conversationRepo.ListMembersByKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	found := make(map[repository.ConversationMemberKey]struct{}, len(members))
	for i := range members {
		found[repository.ConversationMemberKey{
			ConversationID: members[i].ConversationID,
			UserID:         members[i].UserID,
		}] = struct{}{}
	}

	valid := make([]int, 0, len(indices))
	for _, index := range indices {
		command := &commands[index]
		key := repository.ConversationMemberKey{
			ConversationID: command.ConversationID,
			UserID:         command.SenderID,
		}
		if _, exists := found[key]; !exists {
			outcomes[index] = rejectedOutcome(command, messageRejectConversation, "会话不存在")
			continue
		}
		valid = append(valid, index)
	}
	return valid, nil
}

func (s *messageService) validateMedia(
	ctx context.Context,
	commands []messaging.MessageCommand,
	indices []int,
	outcomes []messaging.MessageOutcome,
) ([]int, error) {
	mediaIDs := make([]uint, 0)
	seen := make(map[uint]struct{})
	for _, index := range indices {
		for _, attachment := range commands[index].Attachments {
			if _, exists := seen[attachment.MediaID]; !exists {
				seen[attachment.MediaID] = struct{}{}
				mediaIDs = append(mediaIDs, attachment.MediaID)
			}
		}
	}

	media, err := s.mediaRepo.ListByIDs(ctx, mediaIDs)
	if err != nil {
		return nil, err
	}
	mediaByID := make(map[uint]*model.Media, len(media))
	for i := range media {
		mediaByID[media[i].ID] = &media[i]
	}

	valid := make([]int, 0, len(indices))
	for _, index := range indices {
		command := &commands[index]
		if !validMessageMedia(command, mediaByID) {
			outcomes[index] = rejectedOutcome(command, messageRejectMedia, "附件不存在")
			continue
		}
		valid = append(valid, index)
	}
	return valid, nil
}

func (s *messageService) createMessages(
	ctx context.Context,
	commands []messaging.MessageCommand,
	indices []int,
	outcomes []messaging.MessageOutcome,
) error {
	// 每个会话单独开启事务并分配 seq，避免一个会话失败影响其他会话。
	conversationOrder := make([]uint, 0)
	indicesByConversation := make(map[uint][]int)
	for _, index := range indices {
		conversationID := commands[index].ConversationID
		if _, exists := indicesByConversation[conversationID]; !exists {
			conversationOrder = append(conversationOrder, conversationID)
		}
		indicesByConversation[conversationID] = append(indicesByConversation[conversationID], index)
	}

	for _, conversationID := range conversationOrder {
		messageIndices := indicesByConversation[conversationID]
		messages := make([]model.Message, len(messageIndices))
		for i, index := range messageIndices {
			messages[i] = messageFromCommand(&commands[index])
		}

		created, err := s.messageRepo.CreateBatch(ctx, conversationID, messages)
		if err != nil {
			return err
		}
		for i, message := range created {
			outcomes[messageIndices[i]] = committedOutcome(&message)
		}
	}
	return nil
}

func validateMessageCommand(command *messaging.MessageCommand) error {
	if command.SenderID == 0 || command.ConversationID == 0 || command.Type != model.MessageTypeNormal {
		return ErrInvalidMessage
	}
	if !validClientMessageID(command.ClientMessageID) || command.AcceptedAt.IsZero() {
		return ErrInvalidMessage
	}
	if strings.TrimSpace(command.TextContent) == "" && len(command.Attachments) == 0 {
		return ErrEmptyMessage
	}
	if len(command.Attachments) > math.MaxUint16+1 {
		return ErrTooManyAttachments
	}

	mediaIDs := make(map[uint]struct{}, len(command.Attachments))
	for i, attachment := range command.Attachments {
		if attachment.MediaID == 0 || attachment.Position != uint16(i) {
			return ErrInvalidAttachment
		}
		if _, exists := mediaIDs[attachment.MediaID]; exists {
			return ErrDuplicateAttachment
		}
		mediaIDs[attachment.MediaID] = struct{}{}
	}
	return nil
}

func validMessageMedia(command *messaging.MessageCommand, media map[uint]*model.Media) bool {
	for _, attachment := range command.Attachments {
		record := media[attachment.MediaID]
		if record == nil || record.UserID == nil || *record.UserID != command.SenderID {
			return false
		}
		if record.Status != model.MediaStatusUploaded {
			return false
		}
	}
	return true
}

func sameCommandPayload(first, second *messaging.MessageCommand) bool {
	if first.ConversationID != second.ConversationID || first.SenderID != second.SenderID {
		return false
	}
	if first.Type != second.Type || first.TextContent != second.TextContent {
		return false
	}
	if len(first.Attachments) != len(second.Attachments) {
		return false
	}
	for i := range first.Attachments {
		if first.Attachments[i] != second.Attachments[i] {
			return false
		}
	}
	return true
}

func sameStoredMessage(message *model.Message, command *messaging.MessageCommand) bool {
	if message.ConversationID != command.ConversationID || message.SenderID != command.SenderID {
		return false
	}
	if message.Type != command.Type || message.TextContent != command.TextContent {
		return false
	}
	if len(message.Attachments) != len(command.Attachments) {
		return false
	}
	for i := range message.Attachments {
		stored, incoming := message.Attachments[i], command.Attachments[i]
		if stored.MediaID != incoming.MediaID || stored.Position != incoming.Position {
			return false
		}
	}
	return true
}

func messageFromCommand(command *messaging.MessageCommand) model.Message {
	attachments := make([]model.MessageAttachment, len(command.Attachments))
	for i, attachment := range command.Attachments {
		attachments[i] = model.MessageAttachment{
			MediaID:   attachment.MediaID,
			Position:  attachment.Position,
			CreatedAt: command.AcceptedAt,
		}
	}
	return model.Message{
		ConversationID:  command.ConversationID,
		SenderID:        command.SenderID,
		ClientMessageID: command.ClientMessageID,
		Type:            command.Type,
		TextContent:     command.TextContent,
		CreatedAt:       command.AcceptedAt,
		Attachments:     attachments,
	}
}

func committedOutcome(message *model.Message) messaging.MessageOutcome {
	attachments := make([]messaging.MessageCommittedAttachment, len(message.Attachments))
	for i, attachment := range message.Attachments {
		attachments[i] = messaging.MessageCommittedAttachment{
			ID:       attachment.ID,
			MediaID:  attachment.MediaID,
			Position: attachment.Position,
		}
	}
	return messaging.MessageOutcome{
		Type: messaging.MessageOutcomeCommitted,
		Committed: &messaging.MessageCommitted{
			ID:              message.ID,
			ClientMessageID: message.ClientMessageID,
			ConversationID:  message.ConversationID,
			Seq:             message.Seq,
			SenderID:        message.SenderID,
			Type:            message.Type,
			TextContent:     message.TextContent,
			Attachments:     attachments,
			CreatedAt:       message.CreatedAt,
		},
	}
}

func rejectedOutcome(
	command *messaging.MessageCommand,
	code string,
	message string,
) messaging.MessageOutcome {
	return messaging.MessageOutcome{
		Type: messaging.MessageOutcomeRejected,
		Rejected: &messaging.MessageRejected{
			ClientMessageID: command.ClientMessageID,
			ConversationID:  command.ConversationID,
			SenderID:        command.SenderID,
			Code:            code,
			Message:         message,
			RejectedAt:      time.Now().UTC(),
		},
	}
}

func writerKey(command *messaging.MessageCommand) messageWriterKey {
	return messageWriterKey{
		senderID:        command.SenderID,
		clientMessageID: command.ClientMessageID,
	}
}

func (s *messageService) Build(
	ctx context.Context,
	outcomes []messaging.MessageOutcome,
) ([]messaging.MessagePush, error) {
	mediaIDs := make([]uint, 0)
	seenMedia := make(map[uint]struct{})
	for i := range outcomes {
		if outcomes[i].Committed == nil {
			continue
		}
		for _, attachment := range outcomes[i].Committed.Attachments {
			if _, exists := seenMedia[attachment.MediaID]; !exists {
				seenMedia[attachment.MediaID] = struct{}{}
				mediaIDs = append(mediaIDs, attachment.MediaID)
			}
		}
	}
	mediaItems, err := s.loadMessageMedia(ctx, mediaIDs)
	if err != nil {
		return nil, err
	}
	members, err := s.conversationRepo.ListMembers(ctx, committedConversationIDs(outcomes))
	if err != nil {
		return nil, err
	}
	membersByConversation := groupMessageMembers(members)

	pushes := make([]messaging.MessagePush, 0, len(outcomes))
	for i := range outcomes {
		if outcomes[i].Committed != nil {
			for j := range outcomes[i].Committed.Attachments {
				attachment := &outcomes[i].Committed.Attachments[j]
				mediaItem, exists := mediaItems[attachment.MediaID]
				if !exists {
					return nil, fmt.Errorf("message attachment media %d was not loaded", attachment.MediaID)
				}
				attachment.Type = mediaItem.Type
				attachment.Filename = mediaItem.Filename
				attachment.MIMEType = mediaItem.MIMEType
				attachment.Size = mediaItem.Size
				attachment.URL = mediaItem.URL
				attachment.URLExpiredAt = &mediaItem.URLExpiredAt
			}
		}
		push, err := buildMessagePush(&outcomes[i], membersByConversation)
		if err != nil {
			return nil, err
		}
		if len(push.UserIDs) > 0 {
			pushes = append(pushes, push)
		}
	}
	return pushes, nil
}

func committedConversationIDs(outcomes []messaging.MessageOutcome) []uint {
	ids := make([]uint, 0, len(outcomes))
	seen := make(map[uint]struct{}, len(outcomes))
	for i := range outcomes {
		committed := outcomes[i].Committed
		if outcomes[i].Type != messaging.MessageOutcomeCommitted || committed == nil {
			continue
		}
		if _, exists := seen[committed.ConversationID]; exists {
			continue
		}
		seen[committed.ConversationID] = struct{}{}
		ids = append(ids, committed.ConversationID)
	}
	return ids
}

func groupMessageMembers(
	members []model.ConversationMember,
) map[uint][]model.ConversationMember {
	result := make(map[uint][]model.ConversationMember)
	for i := range members {
		conversationID := members[i].ConversationID
		result[conversationID] = append(result[conversationID], members[i])
	}
	return result
}

func buildMessagePush(
	outcome *messaging.MessageOutcome,
	members map[uint][]model.ConversationMember,
) (messaging.MessagePush, error) {
	switch {
	case outcome.Type == messaging.MessageOutcomeCommitted && outcome.Committed != nil && outcome.Rejected == nil:
		return buildMessageCreatedPush(outcome.Committed, members)
	case outcome.Type == messaging.MessageOutcomeRejected && outcome.Rejected != nil && outcome.Committed == nil:
		return buildMessageRejectedPush(outcome.Rejected)
	default:
		return messaging.MessagePush{}, errors.New("invalid message outcome")
	}
}

func buildMessageCreatedPush(
	message *messaging.MessageCommitted,
	members map[uint][]model.ConversationMember,
) (messaging.MessagePush, error) {
	userIDs := make([]uint, 0, len(members[message.ConversationID]))
	for _, member := range members[message.ConversationID] {
		if message.Seq > member.JoinedSeq {
			userIDs = append(userIDs, member.UserID)
		}
	}
	if len(userIDs) == 0 {
		return messaging.MessagePush{}, nil
	}

	data, err := json.Marshal(message)
	if err != nil {
		return messaging.MessagePush{}, fmt.Errorf("marshal committed message: %w", err)
	}
	return messaging.MessagePush{
		UserIDs: userIDs,
		Event: messaging.MessagePushEvent{
			Type: messaging.MessagePushCreated,
			Data: data,
		},
	}, nil
}

func buildMessageRejectedPush(message *messaging.MessageRejected) (messaging.MessagePush, error) {
	data, err := json.Marshal(message)
	if err != nil {
		return messaging.MessagePush{}, fmt.Errorf("marshal rejected message: %w", err)
	}
	return messaging.MessagePush{
		UserIDs: []uint{message.SenderID},
		Event: messaging.MessagePushEvent{
			Type: messaging.MessagePushRejected,
			Data: data,
		},
	}, nil
}
