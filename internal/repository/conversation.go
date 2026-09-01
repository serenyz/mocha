package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mmchat/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrDirectConversationDuplicate = errors.New("direct conversation duplicate")

type ConversationListCursor struct {
	SortAt time.Time
	ID     uint
}

type ListConversationsParams struct {
	UserID uint
	Cursor *ConversationListCursor
	Limit  int
}

type ConversationMemberKey struct {
	ConversationID uint
	UserID         uint
}

type ConversationRepository interface {
	FindDirect(ctx context.Context, firstUserID, secondUserID uint) (*model.Conversation, error)
	FindMember(ctx context.Context, conversationID, userID uint) (*model.ConversationMember, error)
	CreateDirect(
		ctx context.Context,
		conversation *model.Conversation,
		direct *model.ConversationDirect,
		members []model.ConversationMember,
	) error
	CreateGroup(
		ctx context.Context,
		conversation *model.Conversation,
		group *model.ConversationGroup,
		members []model.ConversationMember,
	) error
	ListByUser(ctx context.Context, params *ListConversationsParams) ([]model.ConversationMember, error)
	ListMembers(ctx context.Context, conversationIDs []uint) ([]model.ConversationMember, error)
	ListMembersByKeys(ctx context.Context, keys []ConversationMemberKey) ([]model.ConversationMember, error)
	ListGroups(ctx context.Context, conversationIDs []uint) ([]model.ConversationGroup, error)
	AdvanceLastReadSeq(ctx context.Context, conversationID, userID uint, seq uint64) (bool, error)
}

type conversationRepository struct {
	db *gorm.DB
}

var _ ConversationRepository = (*conversationRepository)(nil)

func NewConversationRepository(db *gorm.DB) ConversationRepository {
	return &conversationRepository{db: db}
}

func (r *conversationRepository) FindDirect(
	ctx context.Context,
	firstUserID, secondUserID uint,
) (*model.Conversation, error) {
	if firstUserID > secondUserID {
		firstUserID, secondUserID = secondUserID, firstUserID
	}
	direct, err := gorm.G[model.ConversationDirect](r.db).
		Joins(clause.InnerJoin.Association("Conversation"), nil).
		Where("`conversation_direct`.`user_low_id` = ?", firstUserID).
		Where("`conversation_direct`.`user_high_id` = ?", secondUserID).
		Take(ctx)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find direct conversation: %w", err)
	}

	if direct.Conversation == nil {
		return nil, fmt.Errorf("direct conversation %d was not loaded", direct.ConversationID)
	}
	return direct.Conversation, nil
}

func (r *conversationRepository) FindMember(
	ctx context.Context,
	conversationID, userID uint,
) (*model.ConversationMember, error) {
	member, err := gorm.G[model.ConversationMember](r.db).
		Joins(clause.InnerJoin.Association("Conversation"), nil).
		Where("`conversation_member`.`conversation_id` = ?", conversationID).
		Where("`conversation_member`.`user_id` = ?", userID).
		Take(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find conversation member: %w", err)
	}

	return &member, nil
}

func (r *conversationRepository) CreateDirect(
	ctx context.Context,
	conversation *model.Conversation,
	direct *model.ConversationDirect,
	members []model.ConversationMember,
) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := gorm.G[model.Conversation](tx).Create(ctx, conversation); err != nil {
			return fmt.Errorf("create direct conversation: %w", err)
		}

		direct.ConversationID = conversation.ID
		if err := gorm.G[model.ConversationDirect](tx).Create(ctx, direct); err != nil {
			return fmt.Errorf("create direct conversation detail: %w", err)
		}

		for i := range members {
			members[i].ConversationID = conversation.ID
		}
		if err := gorm.G[model.ConversationMember](tx).CreateInBatches(ctx, &members, len(members)); err != nil {
			return fmt.Errorf("create direct conversation members: %w", err)
		}

		return nil
	})

	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDirectConversationDuplicate
	}
	return fmt.Errorf("create direct conversation with members: %w", err)
}

func (r *conversationRepository) CreateGroup(
	ctx context.Context,
	conversation *model.Conversation,
	group *model.ConversationGroup,
	members []model.ConversationMember,
) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := gorm.G[model.Conversation](tx).Create(ctx, conversation); err != nil {
			return fmt.Errorf("create group conversation: %w", err)
		}

		group.ConversationID = conversation.ID
		if err := gorm.G[model.ConversationGroup](tx).Create(ctx, group); err != nil {
			return fmt.Errorf("create group conversation detail: %w", err)
		}

		for i := range members {
			members[i].ConversationID = conversation.ID
		}
		if err := gorm.G[model.ConversationMember](tx).CreateInBatches(ctx, &members, len(members)); err != nil {
			return fmt.Errorf("create group members: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("create group conversation with members: %w", err)
	}

	return nil
}

func (r *conversationRepository) ListByUser(
	ctx context.Context,
	params *ListConversationsParams,
) ([]model.ConversationMember, error) {
	query := gorm.G[model.ConversationMember](r.db).
		Joins(clause.InnerJoin.Association("Conversation"), nil).
		Where("`conversation_member`.`user_id` = ?", params.UserID)

	if params.Cursor != nil {
		cursor := params.Cursor
		query = query.Where(
			`Conversation.last_message_at < ? OR
				(Conversation.last_message_at = ? AND Conversation.id < ?)`,
			cursor.SortAt, cursor.SortAt, cursor.ID,
		)
	}

	memberships, err := query.
		Order("Conversation.last_message_at DESC").
		Order("Conversation.id DESC").
		Limit(params.Limit).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("list conversations by user: %w", err)
	}

	return memberships, nil
}

func (r *conversationRepository) ListMembers(
	ctx context.Context,
	conversationIDs []uint,
) ([]model.ConversationMember, error) {
	if len(conversationIDs) == 0 {
		return []model.ConversationMember{}, nil
	}

	members, err := gorm.G[model.ConversationMember](r.db).
		Select("conversation_id", "user_id", "joined_seq").
		Where("conversation_id IN ?", conversationIDs).
		Order("conversation_id ASC").
		Order("user_id ASC").
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("list conversation members: %w", err)
	}

	return members, nil
}

func (r *conversationRepository) ListMembersByKeys(
	ctx context.Context,
	keys []ConversationMemberKey,
) ([]model.ConversationMember, error) {
	if len(keys) == 0 {
		return []model.ConversationMember{}, nil
	}

	// 慢sql
	members, err := gorm.G[model.ConversationMember](r.db).
		Where(conversationMemberKeysClause(keys)).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("list conversation members by keys: %w", err)
	}
	return members, nil
}

func (r *conversationRepository) ListGroups(
	ctx context.Context,
	conversationIDs []uint,
) ([]model.ConversationGroup, error) {
	if len(conversationIDs) == 0 {
		return []model.ConversationGroup{}, nil
	}

	groups, err := gorm.G[model.ConversationGroup](r.db).
		Joins(clause.LeftJoin.Association("AvatarMedia"), nil).
		Where("`conversation_group`.`conversation_id` IN ?", conversationIDs).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("list conversation groups: %w", err)
	}

	return groups, nil
}

func (r *conversationRepository) AdvanceLastReadSeq(
	ctx context.Context,
	conversationID, userID uint,
	seq uint64,
) (bool, error) {
	rowsAffected, err := gorm.G[model.ConversationMember](r.db).
		Where("conversation_id = ?", conversationID).
		Where("user_id = ?", userID).
		Where("last_read_seq < ?", seq).
		Update(ctx, "last_read_seq", seq)
	if err != nil {
		return false, fmt.Errorf("advance conversation last read seq: %w", err)
	}
	return rowsAffected == 1, nil
}

func conversationMemberKeysClause(keys []ConversationMemberKey) clause.IN {
	values := make([]interface{}, len(keys))
	for i := range keys {
		values[i] = []interface{}{keys[i].ConversationID, keys[i].UserID}
	}
	return clause.IN{
		Column: []clause.Column{{Name: "conversation_id"}, {Name: "user_id"}},
		Values: values,
	}
}
