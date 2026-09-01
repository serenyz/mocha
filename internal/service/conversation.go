package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"mmchat/internal/api"
	"mmchat/internal/common"
	"mmchat/internal/model"
	"mmchat/internal/repository"
)

const (
	defaultConversationListLimit = 20
	maxConversationListLimit     = 50
)

type CreateDirectConversationCommand struct {
	CurrentUserID uint
	TargetUserID  uint
}

type CreateDirectConversationRes struct {
	ID   uint
	Type model.ConversationType
}

type CreateGroupConversationCommand struct {
	CurrentUserID uint
	Name          string
	AvatarMediaID *uint
	UserIDs       []uint
}

type CreateGroupConversationRes struct {
	ID   uint
	Type model.ConversationType
}

type ListConversationsCommand struct {
	UserID uint
	Cursor *string
	Limit  int
}

type ConversationListItem struct {
	ID             uint
	Type           model.ConversationType
	Group          *conversationGroupProfile
	Peers          []*publicProfile
	MemberCount    int
	LastMessage    *conversationMessageSummary
	LastMessageSeq uint64
	JoinedSeq      uint64
	LastReadSeq    uint64
	UnreadCount    int64
}

type GetConversationCommand struct {
	UserID         uint
	ConversationID uint
}

type GetConversationRes struct {
	ConversationListItem
}

type ListConversationsRes struct {
	Items      []*ConversationListItem
	NextCursor *string
	HasMore    bool
	Limit      int
}

type conversationListCursor struct {
	SortAt time.Time `json:"t"`
	ID     uint      `json:"i"`
}

type conversationPeerSet struct {
	Peers       []*publicProfile
	MemberCount int
}

type conversationGroupProfile struct {
	Name string
	avatar
}

type conversationMessageSummary struct {
	ID        uint
	Seq       uint64
	SenderID  uint
	Type      model.MessageType
	Text      string
	CreatedAt time.Time
}

type ConversationService interface {
	CreateDirectConversation(
		ctx context.Context,
		cmd *CreateDirectConversationCommand,
		res *CreateDirectConversationRes,
	) error
	CreateGroupConversation(
		ctx context.Context,
		cmd *CreateGroupConversationCommand,
		res *CreateGroupConversationRes,
	) error
	GetConversation(ctx context.Context, cmd *GetConversationCommand, res *GetConversationRes) error
	ListConversations(
		ctx context.Context,
		cmd *ListConversationsCommand,
		res *ListConversationsRes,
	) error
}

type conversationService struct {
	conversationRepo repository.ConversationRepository
	messageRepo      repository.MessageRepository
	userRepo         repository.UserRepository
	mediaRepo        repository.MediaRepository
	objs             ObjectStorageService
}

var _ ConversationService = (*conversationService)(nil)

func NewConversationService(
	conversationRepo repository.ConversationRepository,
	messageRepo repository.MessageRepository,
	userRepo repository.UserRepository,
	mediaRepo repository.MediaRepository,
	objs ObjectStorageService,
) ConversationService {
	return &conversationService{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		userRepo:         userRepo,
		mediaRepo:        mediaRepo,
		objs:             objs,
	}
}

func (s *conversationService) CreateDirectConversation(
	ctx context.Context,
	cmd *CreateDirectConversationCommand,
	res *CreateDirectConversationRes,
) error {
	if cmd.CurrentUserID == 0 || cmd.TargetUserID == 0 {
		return api.ErrInvalidArgument
	}

	targetUser, err := s.userRepo.FindByID(ctx, cmd.TargetUserID)
	if err != nil {
		return err
	}
	if targetUser == nil || targetUser.Status != model.UserStatusNormal {
		return api.ErrUserNotFound
	}

	existing, err := s.conversationRepo.FindDirect(ctx, cmd.CurrentUserID, cmd.TargetUserID)
	if err != nil {
		return err
	}
	if existing != nil {
		res.ID, res.Type = existing.ID, existing.Type
		return nil
	}

	lowID, highID := directUserIDs(cmd.CurrentUserID, cmd.TargetUserID)
	now := time.Now()
	conversation := &model.Conversation{
		Type:          model.ConversationTypeDirect,
		LastMessageAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	direct := &model.ConversationDirect{UserLowID: lowID, UserHighID: highID}
	members := []model.ConversationMember{{UserID: lowID, JoinedAt: now}}
	if highID != lowID {
		members = append(members, model.ConversationMember{UserID: highID, JoinedAt: now})
	}

	err = s.conversationRepo.CreateDirect(ctx, conversation, direct, members)
	if err == nil {
		res.ID, res.Type = conversation.ID, conversation.Type
		return nil
	}
	if !errors.Is(err, repository.ErrDirectConversationDuplicate) {
		return err
	}

	existing, err = s.conversationRepo.FindDirect(ctx, cmd.CurrentUserID, cmd.TargetUserID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("direct conversation duplicated but existing record not found")
	}

	res.ID, res.Type = existing.ID, existing.Type
	return nil
}

func (s *conversationService) CreateGroupConversation(
	ctx context.Context,
	cmd *CreateGroupConversationCommand,
	res *CreateGroupConversationRes,
) error {
	if cmd.CurrentUserID == 0 {
		return api.ErrInvalidArgument
	}

	name, err := common.NormalizeGroupName(cmd.Name)
	if err != nil {
		return err
	}
	if cmd.AvatarMediaID != nil {
		if *cmd.AvatarMediaID == 0 {
			return api.ErrInvalidArgument
		}
		if err := s.validateGroupAvatar(ctx, cmd.CurrentUserID, *cmd.AvatarMediaID); err != nil {
			return err
		}
	}

	memberIDs, err := normalizeUserIDs(append([]uint{cmd.CurrentUserID}, cmd.UserIDs...))
	if err != nil {
		return err
	}
	if err := s.validateUsers(ctx, memberIDs); err != nil {
		return err
	}

	now := time.Now()
	conversation := &model.Conversation{
		Type:          model.ConversationTypeGroup,
		LastMessageAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	group := &model.ConversationGroup{
		Name:          name,
		AvatarMediaID: cmd.AvatarMediaID,
		UpdatedAt:     now,
	}
	members := conversationMembers(memberIDs, now)
	if err := s.conversationRepo.CreateGroup(ctx, conversation, group, members); err != nil {
		return err
	}

	res.ID, res.Type = conversation.ID, conversation.Type
	return nil
}

func (s *conversationService) GetConversation(
	ctx context.Context,
	cmd *GetConversationCommand,
	res *GetConversationRes,
) error {
	if cmd.UserID == 0 || cmd.ConversationID == 0 {
		return api.ErrInvalidArgument
	}

	membership, err := s.conversationRepo.FindMember(ctx, cmd.ConversationID, cmd.UserID)
	if err != nil {
		return err
	}
	if membership == nil {
		return api.ErrConversationNotFound
	}
	if membership.Conversation == nil {
		return fmt.Errorf("conversation %d was not loaded", cmd.ConversationID)
	}
	conversation := membership.Conversation

	peerSets, err := s.loadConversationPeers(ctx, cmd.UserID, []*model.Conversation{conversation})
	if err != nil {
		return err
	}
	peerSet := peerSets[cmd.ConversationID]
	if peerSet == nil {
		return fmt.Errorf("conversation %d has no peer set", cmd.ConversationID)
	}
	groups, err := s.loadConversationGroups(ctx, []*model.Conversation{conversation})
	if err != nil {
		return err
	}
	lastMessages, err := s.loadConversationLastMessages(ctx, []model.ConversationMember{*membership})
	if err != nil {
		return err
	}

	res.ConversationListItem = ConversationListItem{
		ID:             conversation.ID,
		Type:           conversation.Type,
		Group:          groups[conversation.ID],
		Peers:          peerSet.Peers,
		MemberCount:    peerSet.MemberCount,
		LastMessage:    lastMessages[conversation.ID],
		LastMessageSeq: conversation.LastMessageSeq,
		JoinedSeq:      membership.JoinedSeq,
		LastReadSeq:    membership.LastReadSeq,
		UnreadCount:    int64(conversation.LastMessageSeq - max(membership.JoinedSeq, membership.LastReadSeq)),
	}
	return nil
}

func (s *conversationService) ListConversations(
	ctx context.Context,
	cmd *ListConversationsCommand,
	res *ListConversationsRes,
) error {
	if cmd.UserID == 0 {
		return api.ErrInvalidArgument
	}

	res.Limit = cmd.Limit
	if res.Limit == 0 {
		res.Limit = defaultConversationListLimit
	}
	if res.Limit < 1 || res.Limit > maxConversationListLimit {
		return api.ErrInvalidArgument
	}

	params := &repository.ListConversationsParams{
		UserID: cmd.UserID,
		Limit:  res.Limit + 1,
	}

	if cmd.Cursor != nil {
		cursor, err := decodeConversationListCursor(*cmd.Cursor)
		if err != nil {
			return err
		}
		params.Cursor = cursor
	}

	memberships, err := s.conversationRepo.ListByUser(ctx, params)
	if err != nil {
		return err
	}

	res.HasMore = len(memberships) > res.Limit
	if res.HasMore {
		memberships = memberships[:res.Limit]
	}

	conversations := make([]*model.Conversation, len(memberships))
	for i := range memberships {
		membership := &memberships[i]
		if membership.Conversation == nil {
			return fmt.Errorf("conversation %d was not loaded", membership.ConversationID)
		}
		conversations[i] = membership.Conversation
	}

	peerSets, err := s.loadConversationPeers(ctx, cmd.UserID, conversations)
	if err != nil {
		return err
	}
	groups, err := s.loadConversationGroups(ctx, conversations)
	if err != nil {
		return err
	}
	lastMessages, err := s.loadConversationLastMessages(ctx, memberships)
	if err != nil {
		return err
	}

	res.Items = make([]*ConversationListItem, len(memberships))
	for i := range memberships {
		membership := &memberships[i]
		conversation := membership.Conversation
		peerSet := peerSets[conversation.ID]
		if peerSet == nil {
			return fmt.Errorf("conversation %d has no peer set", conversation.ID)
		}

		res.Items[i] = &ConversationListItem{
			ID:             conversation.ID,
			Type:           conversation.Type,
			Group:          groups[conversation.ID],
			Peers:          peerSet.Peers,
			MemberCount:    peerSet.MemberCount,
			LastMessage:    lastMessages[conversation.ID],
			LastMessageSeq: conversation.LastMessageSeq,
			JoinedSeq:      membership.JoinedSeq,
			LastReadSeq:    membership.LastReadSeq,
			UnreadCount:    int64(conversation.LastMessageSeq - max(membership.JoinedSeq, membership.LastReadSeq)),
		}
	}

	if res.HasMore {
		last := memberships[len(memberships)-1].Conversation
		cursor, err := encodeConversationListCursor(last.LastMessageAt, last.ID)
		if err != nil {
			return err
		}
		res.NextCursor = &cursor
	}

	return nil
}

func (s *conversationService) loadConversationLastMessages(
	ctx context.Context,
	memberships []model.ConversationMember,
) (map[uint]*conversationMessageSummary, error) {
	lastMessageIDs := make([]uint, 0, len(memberships))
	conversationByMessageID := make(map[uint]uint, len(memberships))
	for i := range memberships {
		membership := &memberships[i]
		conversation := membership.Conversation
		if conversation == nil {
			return nil, fmt.Errorf("conversation %d was not loaded", membership.ConversationID)
		}
		if conversation.LastMessageID != nil {
			lastMessageIDs = append(lastMessageIDs, *conversation.LastMessageID)
			conversationByMessageID[*conversation.LastMessageID] = conversation.ID
		}
	}

	messages, err := s.messageRepo.ListByIDs(ctx, lastMessageIDs)
	if err != nil {
		return nil, err
	}
	lastMessages := make(map[uint]*conversationMessageSummary, len(messages))
	for i := range messages {
		message := &messages[i]
		conversationID, exists := conversationByMessageID[message.ID]
		if !exists || message.ConversationID != conversationID {
			return nil, fmt.Errorf("last message %d does not belong to its conversation", message.ID)
		}
		lastMessages[conversationID] = &conversationMessageSummary{
			ID:        message.ID,
			Seq:       message.Seq,
			SenderID:  message.SenderID,
			Type:      message.Type,
			Text:      message.TextContent,
			CreatedAt: message.CreatedAt,
		}
	}
	if len(lastMessages) != len(lastMessageIDs) {
		return nil, errors.New("some conversation last messages were not found")
	}

	return lastMessages, nil
}

func (s *conversationService) loadConversationGroups(
	ctx context.Context,
	conversations []*model.Conversation,
) (map[uint]*conversationGroupProfile, error) {
	groupIDs := make([]uint, 0, len(conversations))
	for _, conversation := range conversations {
		if conversation.Type == model.ConversationTypeGroup {
			groupIDs = append(groupIDs, conversation.ID)
		}
	}

	groups, err := s.conversationRepo.ListGroups(ctx, groupIDs)
	if err != nil {
		return nil, err
	}

	profiles := make(map[uint]*conversationGroupProfile, len(groups))
	for i := range groups {
		group := &groups[i]
		groupAvatar, err := s.loadAvatar(ctx, group.AvatarMedia)
		if err != nil {
			return nil, err
		}
		profiles[group.ConversationID] = &conversationGroupProfile{
			Name:   group.Name,
			avatar: groupAvatar,
		}
	}

	for _, groupID := range groupIDs {
		if profiles[groupID] == nil {
			return nil, fmt.Errorf("group conversation %d was not found", groupID)
		}
	}
	return profiles, nil
}

func (s *conversationService) loadConversationPeers(
	ctx context.Context,
	currentUserID uint,
	conversations []*model.Conversation,
) (map[uint]*conversationPeerSet, error) {
	conversationIDs := make([]uint, len(conversations))
	for i, conversation := range conversations {
		conversationIDs[i] = conversation.ID
	}

	members, err := s.conversationRepo.ListMembers(ctx, conversationIDs)
	if err != nil {
		return nil, err
	}

	peerIDsByConversation := make(map[uint][]uint, len(conversationIDs))
	memberCounts := make(map[uint]int, len(conversationIDs))
	for _, member := range members {
		conversationID := member.ConversationID
		memberCounts[conversationID]++
		if member.UserID != currentUserID {
			peerIDsByConversation[conversationID] = append(peerIDsByConversation[conversationID], member.UserID)
		}
	}

	uniquePeerIDs := make([]uint, 0, len(members))
	seenPeerIDs := make(map[uint]struct{}, len(members))
	for _, conversation := range conversations {
		conversationID := conversation.ID
		if memberCounts[conversationID] == 0 {
			return nil, fmt.Errorf("conversation %d has no members", conversationID)
		}
		if conversation.Type == model.ConversationTypeDirect && len(peerIDsByConversation[conversationID]) == 0 {
			peerIDsByConversation[conversationID] = []uint{currentUserID}
		}
		for _, peerID := range peerIDsByConversation[conversationID] {
			if _, exists := seenPeerIDs[peerID]; exists {
				continue
			}
			seenPeerIDs[peerID] = struct{}{}
			uniquePeerIDs = append(uniquePeerIDs, peerID)
		}
	}

	users, err := s.userRepo.FindDetailsByIDs(ctx, uniquePeerIDs)
	if err != nil {
		return nil, err
	}

	peersByUserID := make(map[uint]*publicProfile, len(users))
	for i := range users {
		user := &users[i]
		peer := &publicProfile{
			ID:        user.ID,
			Nickname:  user.Profile.Nickname,
			Gender:    user.Profile.Gender,
			Signature: user.Profile.Signature,
			Birthday:  user.Profile.Birthday,
			Country:   user.Profile.Country,
			Province:  user.Profile.Province,
		}
		peerAvatar, err := s.loadAvatar(ctx, user.Profile.AvatarMedia)
		if err != nil {
			return nil, err
		}
		peer.avatar = peerAvatar
		peersByUserID[user.ID] = peer
	}

	peerSets := make(map[uint]*conversationPeerSet, len(conversationIDs))
	for _, conversation := range conversations {
		conversationID := conversation.ID
		peerIDs := peerIDsByConversation[conversationID]
		peers := make([]*publicProfile, len(peerIDs))
		for i, peerID := range peerIDs {
			peer := peersByUserID[peerID]
			if peer == nil {
				return nil, fmt.Errorf("conversation peer %d was not found", peerID)
			}
			peers[i] = peer
		}
		peerSets[conversationID] = &conversationPeerSet{
			Peers:       peers,
			MemberCount: memberCounts[conversationID],
		}
	}

	return peerSets, nil
}

func (s *conversationService) loadAvatar(
	ctx context.Context,
	avatarMedia *model.Media,
) (avatar, error) {
	if avatarMedia == nil {
		return avatar{}, nil
	}

	getRes := &PresignGetObjectRes{}
	if err := s.objs.PresignGetObject(
		ctx,
		&PresignGetObjectCommand{
			StorageKey: avatarMedia.StorageKey,
			ExpireIn:   getObjectTTL,
		},
		getRes,
	); err != nil {
		return avatar{}, err
	}

	return avatar{
		MediaID:      avatarMedia.ID,
		AvatarURL:    getRes.URL,
		URLExpiredAt: getRes.ExpiresAt,
	}, nil
}

func directUserIDs(firstUserID, secondUserID uint) (uint, uint) {
	if firstUserID <= secondUserID {
		return firstUserID, secondUserID
	}
	return secondUserID, firstUserID
}

func normalizeUserIDs(userIDs []uint) ([]uint, error) {
	result := make([]uint, 0, len(userIDs))
	seen := make(map[uint]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID == 0 {
			return nil, api.ErrInvalidArgument
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		result = append(result, userID)
	}
	return result, nil
}

func conversationMembers(userIDs []uint, joinedAt time.Time) []model.ConversationMember {
	members := make([]model.ConversationMember, len(userIDs))
	for i, userID := range userIDs {
		members[i] = model.ConversationMember{UserID: userID, JoinedAt: joinedAt}
	}
	return members
}

func (s *conversationService) validateUsers(ctx context.Context, userIDs []uint) error {
	users, err := s.userRepo.FindByIDs(ctx, userIDs)
	if err != nil {
		return err
	}
	if len(users) != len(userIDs) {
		return api.ErrUserNotFound
	}
	for _, user := range users {
		if user.Status != model.UserStatusNormal {
			return api.ErrUserNotFound
		}
	}
	return nil
}

func (s *conversationService) validateGroupAvatar(ctx context.Context, userID, mediaID uint) error {
	mediaRecord, err := s.mediaRepo.FindByID(ctx, mediaID)
	if err != nil {
		return err
	}
	if mediaRecord == nil || mediaRecord.UserID == nil ||
		*mediaRecord.UserID != userID ||
		mediaRecord.Status != model.MediaStatusUploaded ||
		mediaRecord.Type != model.MediaTypeImage {
		return api.ErrMediaNotFound
	}
	return nil
}

func encodeConversationListCursor(
	sortAt time.Time,
	id uint,
) (string, error) {
	data, err := json.Marshal(conversationListCursor{
		SortAt: sortAt,
		ID:     id,
	})
	if err != nil {
		return "", fmt.Errorf("encode conversation cursor: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeConversationListCursor(
	value string,
) (*repository.ConversationListCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, api.ErrInvalidArgument
	}

	var cursor conversationListCursor
	if err := json.Unmarshal(data, &cursor); err != nil ||
		cursor.ID == 0 ||
		cursor.SortAt.IsZero() {
		return nil, api.ErrInvalidArgument
	}

	return &repository.ConversationListCursor{
		SortAt: cursor.SortAt,
		ID:     cursor.ID,
	}, nil
}
