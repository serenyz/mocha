package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"mmchat/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrMessageConversationMissing = errors.New("message conversation missing")
	ErrMessageSequenceExhausted   = errors.New("message sequence exhausted")
)

type MessageRepository interface {
	FindByID(ctx context.Context, id uint) (*model.Message, error)
	ListByIDs(ctx context.Context, ids []uint) ([]model.Message, error)
	ListByClientIDs(ctx context.Context, keys []MessageClientKey) ([]model.Message, error)
	ListLatest(ctx context.Context, conversationID uint, minSeq uint64, limit int) ([]model.Message, error)
	ListBeforeSeq(
		ctx context.Context,
		conversationID uint,
		minSeq uint64,
		beforeSeq uint64,
		limit int,
	) ([]model.Message, error)
	ListAfterSeq(ctx context.Context, conversationID uint, afterSeq uint64, limit int) ([]model.Message, error)
	ListAttachmentsByMessageIDs(ctx context.Context, messageIDs []uint) ([]model.MessageAttachment, error)
	CreateBatch(ctx context.Context, conversationID uint, messages []model.Message) ([]model.Message, error)
}

type MessageClientKey struct {
	SenderID        uint
	ClientMessageID string
}

type messageRepository struct {
	db *gorm.DB
}

var _ MessageRepository = (*messageRepository)(nil)

func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &messageRepository{db: db}
}

func (r *messageRepository) FindByID(ctx context.Context, id uint) (*model.Message, error) {
	message, err := gorm.G[model.Message](r.db).Where("id = ?", id).Take(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find message by id: %w", err)
	}
	return &message, nil
}

func (r *messageRepository) ListByIDs(
	ctx context.Context,
	ids []uint,
) ([]model.Message, error) {
	if len(ids) == 0 {
		return []model.Message{}, nil
	}

	messages, err := gorm.G[model.Message](r.db).Where("id IN ?", ids).Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("list messages by ids: %w", err)
	}
	return messages, nil
}

func (r *messageRepository) ListByClientIDs(
	ctx context.Context,
	keys []MessageClientKey,
) ([]model.Message, error) {
	if len(keys) == 0 {
		return []model.Message{}, nil
	}

	messages, err := gorm.G[model.Message](r.db).
		Where(messageClientKeysClause(keys)).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("list messages by client ids: %w", err)
	}
	return messages, nil
}

func (r *messageRepository) ListLatest(
	ctx context.Context,
	conversationID uint,
	minSeq uint64,
	limit int,
) ([]model.Message, error) {
	if limit <= 0 {
		return []model.Message{}, nil
	}

	messages, err := gorm.G[model.Message](r.db).
		Where("conversation_id = ?", conversationID).
		Where("seq > ?", minSeq).
		Order("seq desc").
		Limit(limit).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("list latest messages: %w", err)
	}

	slices.Reverse(messages)
	return messages, nil
}

func (r *messageRepository) ListBeforeSeq(
	ctx context.Context,
	conversationID uint,
	minSeq uint64,
	beforeSeq uint64,
	limit int,
) ([]model.Message, error) {
	if limit <= 0 {
		return []model.Message{}, nil
	}

	messages, err := gorm.G[model.Message](r.db).
		Where("conversation_id = ?", conversationID).
		Where("seq > ?", minSeq).
		Where("seq < ?", beforeSeq).
		Order("seq desc").
		Limit(limit).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("list messages before seq: %w", err)
	}

	slices.Reverse(messages)
	return messages, nil
}

func (r *messageRepository) ListAfterSeq(
	ctx context.Context,
	conversationID uint,
	afterSeq uint64,
	limit int,
) ([]model.Message, error) {
	if limit <= 0 {
		return []model.Message{}, nil
	}

	messages, err := gorm.G[model.Message](r.db).
		Where("conversation_id = ?", conversationID).
		Where("seq > ?", afterSeq).
		Order("seq").
		Limit(limit).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("list messages after seq: %w", err)
	}
	return messages, nil
}

func (r *messageRepository) ListAttachmentsByMessageIDs(
	ctx context.Context,
	messageIDs []uint,
) ([]model.MessageAttachment, error) {
	attachments, err := listMessageAttachments(ctx, r.db, messageIDs)
	if err != nil {
		return nil, fmt.Errorf("list message attachments: %w", err)
	}
	return attachments, nil
}

func (r *messageRepository) CreateBatch(
	ctx context.Context,
	conversationID uint,
	messages []model.Message,
) ([]model.Message, error) {
	if len(messages) == 0 {
		return []model.Message{}, nil
	}

	// 消息、附件、seq、会话摘要和发送者已读进度必须在同一事务内提交。
	err := r.db.Transaction(func(tx *gorm.DB) error {
		return r.createBatch(ctx, tx, conversationID, messages)
	})
	if err != nil {
		return nil, fmt.Errorf("create message batch: %w", err)
	}
	return messages, nil
}

func (r *messageRepository) createBatch(
	ctx context.Context,
	tx *gorm.DB,
	conversationID uint,
	messages []model.Message,
) error {
	// 分配 seq 前锁住会话，让同一会话的并发写入串行执行。
	// todo: sql并发抢锁，慢
	conversation, err := gorm.G[model.Conversation](tx, clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", conversationID).
		Take(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrMessageConversationMissing
	}
	if err != nil {
		return fmt.Errorf("lock message conversation: %w", err)
	}

	if uint64(len(messages)) > math.MaxUint64-conversation.LastMessageSeq {
		return ErrMessageSequenceExhausted
	}
	// 锁定后的 last_message_seq 是当前批次分配 seq 的唯一基准。
	for i := range messages {
		messages[i].ConversationID = conversationID
		messages[i].Seq = conversation.LastMessageSeq + uint64(i) + 1
	}

	if err := createMessages(ctx, tx, messages); err != nil {
		return err
	}

	// 会话列表使用的最后消息摘要必须和消息一起提交。
	lastMessage := messages[len(messages)-1]
	conversation.LastMessageID = &lastMessage.ID
	conversation.LastMessageSeq = lastMessage.Seq
	conversation.LastMessageAt = lastMessage.CreatedAt
	conversation.UpdatedAt = time.Now()

	// TODO:慢sql
	rowsAffected, err := gorm.G[model.Conversation](tx).
		Where("id = ?", conversationID).
		Select("last_message_id", "last_message_seq", "last_message_at", "updated_at").
		Updates(ctx, conversation)
	if err != nil {
		return fmt.Errorf("update conversation last message: %w", err)
	}
	if rowsAffected != 1 {
		return ErrMessageConversationMissing
	}
	if err := advanceSenderReadSeqs(ctx, tx, conversationID, messages); err != nil {
		return err
	}
	return loadMessageAttachments(ctx, tx, messages)
}

func advanceSenderReadSeqs(
	ctx context.Context,
	tx *gorm.DB,
	conversationID uint,
	messages []model.Message,
) error {
	lastSeqs := make(map[uint]uint64)
	for i := range messages {
		lastSeqs[messages[i].SenderID] = messages[i].Seq
	}

	// TODO: 慢sql
	for senderID, seq := range lastSeqs {
		_, err := gorm.G[model.ConversationMember](tx).
			Where("conversation_id = ?", conversationID).
			Where("user_id = ?", senderID).
			Where("last_read_seq < ?", seq).
			Update(ctx, "last_read_seq", seq)
		if err != nil {
			return fmt.Errorf("advance message sender read seq: %w", err)
		}
	}
	return nil
}

func createMessages(ctx context.Context, tx *gorm.DB, messages []model.Message) error {
	if len(messages) == 0 {
		return nil
	}

	// TODO: 慢SQL
	if err := gorm.G[model.Message](tx).Omit(clause.Associations).CreateInBatches(ctx, &messages, len(messages)); err != nil {
		return fmt.Errorf("create message batch: %w", err)
	}

	attachments := make([]model.MessageAttachment, 0)
	for i := range messages {
		for _, source := range messages[i].Attachments {
			attachment := source
			attachment.MessageID = messages[i].ID
			attachments = append(attachments, attachment)
		}
	}
	if len(attachments) == 0 {
		return nil
	}
	if err := gorm.G[model.MessageAttachment](tx).Omit(clause.Associations).
		CreateInBatches(ctx, &attachments, len(attachments)); err != nil {
		return fmt.Errorf("create message attachments: %w", err)
	}
	return nil
}

func loadMessageAttachments(ctx context.Context, db *gorm.DB, messages []model.Message) error {
	if len(messages) == 0 {
		return nil
	}

	messageIDs := make([]uint, len(messages))
	messageIndexes := make(map[uint]int, len(messages))
	for i := range messages {
		messageIDs[i] = messages[i].ID
		messageIndexes[messages[i].ID] = i
		messages[i].Attachments = nil
	}

	attachments, err := listMessageAttachments(ctx, db, messageIDs)
	if err != nil {
		return fmt.Errorf("load message attachments: %w", err)
	}
	for i := range attachments {
		index, ok := messageIndexes[attachments[i].MessageID]
		if !ok {
			return errors.New("attachment message was not found")
		}
		messages[index].Attachments = append(messages[index].Attachments, attachments[i])
	}
	return nil
}

func listMessageAttachments(
	ctx context.Context,
	db *gorm.DB,
	messageIDs []uint,
) ([]model.MessageAttachment, error) {
	if len(messageIDs) == 0 {
		return []model.MessageAttachment{}, nil
	}

	attachments, err := gorm.G[model.MessageAttachment](db).
		Where("message_id IN ?", messageIDs).
		Order("message_id, position").
		Find(ctx)
	if err != nil {
		return nil, err
	}
	return attachments, nil
}

func messageClientKeysClause(keys []MessageClientKey) clause.IN {
	values := make([]interface{}, len(keys))
	for i := range keys {
		values[i] = []interface{}{keys[i].SenderID, keys[i].ClientMessageID}
	}
	return clause.IN{
		Column: []clause.Column{{Name: "sender_id"}, {Name: "client_message_id"}},
		Values: values,
	}
}
