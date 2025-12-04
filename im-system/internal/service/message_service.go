// Package service 消息服务
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/d60-lab/im-system/internal/model"
	"github.com/d60-lab/im-system/internal/repository"
)

// MessageService 消息服务接口
type MessageService interface {
	// SaveMessage 保存消息
	SaveMessage(ctx context.Context, msg *model.Message) error

	// GetConversationMessages 获取会话消息历史
	GetConversationMessages(ctx context.Context, userID, conversationID string, lastSeq int64, limit int) ([]*MessageDTO, error)

	// GetGroupMessages 获取群聊消息历史
	GetGroupMessages(ctx context.Context, userID, groupID string, lastSeq int64, limit int) ([]*MessageDTO, error)

	// GetPrivateMessages 获取私聊消息历史
	GetPrivateMessages(ctx context.Context, userID, otherUserID string, lastSeq int64, limit int) ([]*MessageDTO, error)

	// RevokeMessage 撤回消息
	RevokeMessage(ctx context.Context, userID, messageID string) error

	// GetMessageByID 获取单条消息
	GetMessageByID(ctx context.Context, messageID string) (*MessageDTO, error)
}

// MessageDTO 消息数据传输对象
type MessageDTO struct {
	MessageID      string                 `json:"message_id"`
	ConversationID string                 `json:"conversation_id"`
	Type           int                    `json:"type"`
	From           string                 `json:"from"`
	To             string                 `json:"to"`
	GroupID        string                 `json:"group_id,omitempty"`
	Content        map[string]interface{} `json:"content"`
	Seq            int64                  `json:"seq"`
	Status         int                    `json:"status"`
	Revoked        bool                   `json:"revoked"`
	Timestamp      int64                  `json:"timestamp"`
	CreatedAt      time.Time              `json:"created_at"`
}

// messageServiceImpl 消息服务实现
type messageServiceImpl struct {
	messageRepo  repository.MessageRepository
	groupService GroupService
}

// NewMessageService 创建消息服务
func NewMessageService(messageRepo repository.MessageRepository, groupService GroupService) MessageService {
	return &messageServiceImpl{
		messageRepo:  messageRepo,
		groupService: groupService,
	}
}

// SaveMessage 保存消息
func (s *messageServiceImpl) SaveMessage(ctx context.Context, msg *model.Message) error {
	// 转换content为map
	content := s.convertContent(msg.Content)

	// 确定group_id
	groupID := ""
	if msg.Type == model.MsgGroupChat {
		groupID = msg.To
	}

	// 创建文档
	doc := &repository.MessageDocument{
		MessageID:      msg.MessageID,
		ConversationID: msg.ConversationID,
		Type:           int(msg.Type),
		From:           msg.From,
		To:             msg.To,
		GroupID:        groupID,
		Content:        content,
		Seq:            msg.Seq,
		Status:         1, // 已发送
		Revoked:        false,
		CreatedAt:      time.UnixMilli(msg.Timestamp),
	}

	if err := s.messageRepo.Save(ctx, doc); err != nil {
		return fmt.Errorf("save message error: %w", err)
	}

	return nil
}

// convertContent 转换消息内容为map
func (s *messageServiceImpl) convertContent(content interface{}) map[string]interface{} {
	if content == nil {
		return nil
	}

	// 如果已经是map，直接返回
	if m, ok := content.(map[string]interface{}); ok {
		return m
	}

	// 尝试JSON序列化再反序列化
	data, err := json.Marshal(content)
	if err != nil {
		return map[string]interface{}{"raw": fmt.Sprintf("%v", content)}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]interface{}{"raw": string(data)}
	}

	return result
}

// GetConversationMessages 获取会话消息历史
func (s *messageServiceImpl) GetConversationMessages(ctx context.Context, userID, conversationID string, lastSeq int64, limit int) ([]*MessageDTO, error) {
	docs, err := s.messageRepo.FindByConversation(ctx, conversationID, lastSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("get conversation messages error: %w", err)
	}

	return s.documentsToDTO(docs), nil
}

// GetGroupMessages 获取群聊消息历史
func (s *messageServiceImpl) GetGroupMessages(ctx context.Context, userID, groupID string, lastSeq int64, limit int) ([]*MessageDTO, error) {
	// 验证用户是否是群成员
	if s.groupService != nil {
		isMember, err := s.groupService.IsMember(ctx, groupID, userID)
		if err != nil {
			return nil, fmt.Errorf("check membership error: %w", err)
		}
		if !isMember {
			return nil, fmt.Errorf("user is not a member of this group")
		}
	}

	docs, err := s.messageRepo.FindByGroup(ctx, groupID, lastSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("get group messages error: %w", err)
	}

	return s.documentsToDTO(docs), nil
}

// GetPrivateMessages 获取私聊消息历史
func (s *messageServiceImpl) GetPrivateMessages(ctx context.Context, userID, otherUserID string, lastSeq int64, limit int) ([]*MessageDTO, error) {
	docs, err := s.messageRepo.FindByPrivateChat(ctx, userID, otherUserID, lastSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("get private messages error: %w", err)
	}

	return s.documentsToDTO(docs), nil
}

// RevokeMessage 撤回消息
func (s *messageServiceImpl) RevokeMessage(ctx context.Context, userID, messageID string) error {
	// 查询消息
	doc, err := s.messageRepo.FindByMessageID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("find message error: %w", err)
	}
	if doc == nil {
		return fmt.Errorf("message not found")
	}

	// 验证是否是发送者
	if doc.From != userID {
		return fmt.Errorf("only sender can revoke message")
	}

	// 检查是否超过撤回时限（2分钟）
	if time.Since(doc.CreatedAt) > 2*time.Minute {
		return fmt.Errorf("message revoke time exceeded")
	}

	// 执行撤回
	if err := s.messageRepo.Revoke(ctx, messageID); err != nil {
		return fmt.Errorf("revoke message error: %w", err)
	}

	return nil
}

// GetMessageByID 获取单条消息
func (s *messageServiceImpl) GetMessageByID(ctx context.Context, messageID string) (*MessageDTO, error) {
	doc, err := s.messageRepo.FindByMessageID(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("find message error: %w", err)
	}
	if doc == nil {
		return nil, nil
	}

	return s.documentToDTO(doc), nil
}

// documentsToDTO 将文档列表转换为DTO列表
func (s *messageServiceImpl) documentsToDTO(docs []*repository.MessageDocument) []*MessageDTO {
	result := make([]*MessageDTO, 0, len(docs))
	for _, doc := range docs {
		result = append(result, s.documentToDTO(doc))
	}
	return result
}

// documentToDTO 将文档转换为DTO
func (s *messageServiceImpl) documentToDTO(doc *repository.MessageDocument) *MessageDTO {
	return &MessageDTO{
		MessageID:      doc.MessageID,
		ConversationID: doc.ConversationID,
		Type:           doc.Type,
		From:           doc.From,
		To:             doc.To,
		GroupID:        doc.GroupID,
		Content:        doc.Content,
		Seq:            doc.Seq,
		Status:         doc.Status,
		Revoked:        doc.Revoked,
		Timestamp:      doc.CreatedAt.UnixMilli(),
		CreatedAt:      doc.CreatedAt,
	}
}
