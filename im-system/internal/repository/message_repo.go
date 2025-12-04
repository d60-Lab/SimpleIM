// Package repository 数据访问层
package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/d60-lab/im-system/pkg/database"
)

// 集合名称
const (
	CollectionMessages = "messages"
)

// MessageDocument MongoDB消息文档
type MessageDocument struct {
	ID             primitive.ObjectID     `bson:"_id,omitempty"`
	MessageID      string                 `bson:"message_id"`
	ConversationID string                 `bson:"conversation_id"`
	Type           int                    `bson:"type"`
	From           string                 `bson:"from"`
	To             string                 `bson:"to"`
	GroupID        string                 `bson:"group_id,omitempty"`
	Content        map[string]interface{} `bson:"content"`
	Seq            int64                  `bson:"seq"`
	Status         int                    `bson:"status"`
	Revoked        bool                   `bson:"revoked"`
	CreatedAt      time.Time              `bson:"created_at"`
	UpdatedAt      time.Time              `bson:"updated_at"`
	ExpireAt       *time.Time             `bson:"expire_at,omitempty"` // TTL索引字段
}

// MessageRepository 消息仓库接口
type MessageRepository interface {
	// Save 保存消息
	Save(ctx context.Context, msg *MessageDocument) error

	// SaveBatch 批量保存消息
	SaveBatch(ctx context.Context, msgs []*MessageDocument) error

	// FindByConversation 按会话查询消息
	FindByConversation(ctx context.Context, conversationID string, lastSeq int64, limit int) ([]*MessageDocument, error)

	// FindByGroup 按群组查询消息
	FindByGroup(ctx context.Context, groupID string, lastSeq int64, limit int) ([]*MessageDocument, error)

	// FindByPrivateChat 按私聊查询消息
	FindByPrivateChat(ctx context.Context, userID1, userID2 string, lastSeq int64, limit int) ([]*MessageDocument, error)

	// FindByMessageID 按消息ID查询
	FindByMessageID(ctx context.Context, messageID string) (*MessageDocument, error)

	// UpdateStatus 更新消息状态
	UpdateStatus(ctx context.Context, messageID string, status int) error

	// Revoke 撤回消息
	Revoke(ctx context.Context, messageID string) error

	// Delete 删除消息
	Delete(ctx context.Context, messageID string) error

	// CountByConversation 统计会话消息数
	CountByConversation(ctx context.Context, conversationID string) (int64, error)

	// EnsureIndexes 确保索引存在
	EnsureIndexes(ctx context.Context) error
}

// messageRepository 消息仓库实现
type messageRepository struct {
	mongo      *database.MongoClient
	collection *mongo.Collection
}

// NewMessageRepository 创建消息仓库
func NewMessageRepository(mongoClient *database.MongoClient) MessageRepository {
	return &messageRepository{
		mongo:      mongoClient,
		collection: mongoClient.Collection(CollectionMessages),
	}
}

// Save 保存消息
func (r *messageRepository) Save(ctx context.Context, msg *MessageDocument) error {
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	msg.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}
	return nil
}

// SaveBatch 批量保存消息
func (r *messageRepository) SaveBatch(ctx context.Context, msgs []*MessageDocument) error {
	if len(msgs) == 0 {
		return nil
	}

	documents := make([]interface{}, len(msgs))
	now := time.Now()
	for i, msg := range msgs {
		if msg.CreatedAt.IsZero() {
			msg.CreatedAt = now
		}
		msg.UpdatedAt = now
		documents[i] = msg
	}

	_, err := r.collection.InsertMany(ctx, documents)
	if err != nil {
		return fmt.Errorf("failed to batch save messages: %w", err)
	}
	return nil
}

// FindByConversation 按会话查询消息
func (r *messageRepository) FindByConversation(ctx context.Context, conversationID string, lastSeq int64, limit int) ([]*MessageDocument, error) {
	filter := bson.M{
		"conversation_id": conversationID,
		"revoked":         false,
	}

	if lastSeq > 0 {
		filter["seq"] = bson.M{"$lt": lastSeq}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "seq", Value: -1}, {Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	return r.findMessages(ctx, filter, opts)
}

// FindByGroup 按群组查询消息
func (r *messageRepository) FindByGroup(ctx context.Context, groupID string, lastSeq int64, limit int) ([]*MessageDocument, error) {
	filter := bson.M{
		"group_id": groupID,
		"revoked":  false,
	}

	if lastSeq > 0 {
		filter["seq"] = bson.M{"$lt": lastSeq}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "seq", Value: -1}, {Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	return r.findMessages(ctx, filter, opts)
}

// FindByPrivateChat 按私聊查询消息
func (r *messageRepository) FindByPrivateChat(ctx context.Context, userID1, userID2 string, lastSeq int64, limit int) ([]*MessageDocument, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"from": userID1, "to": userID2},
			{"from": userID2, "to": userID1},
		},
		"group_id": bson.M{"$in": []interface{}{"", nil}},
		"revoked":  false,
	}

	if lastSeq > 0 {
		filter["seq"] = bson.M{"$lt": lastSeq}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "seq", Value: -1}, {Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	return r.findMessages(ctx, filter, opts)
}

// findMessages 通用查询方法
func (r *messageRepository) findMessages(ctx context.Context, filter bson.M, opts *options.FindOptions) ([]*MessageDocument, error) {
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find messages: %w", err)
	}
	defer cursor.Close(ctx)

	var messages []*MessageDocument
	if err := cursor.All(ctx, &messages); err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}

	return messages, nil
}

// FindByMessageID 按消息ID查询
func (r *messageRepository) FindByMessageID(ctx context.Context, messageID string) (*MessageDocument, error) {
	var msg MessageDocument
	err := r.collection.FindOne(ctx, bson.M{"message_id": messageID}).Decode(&msg)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find message: %w", err)
	}
	return &msg, nil
}

// UpdateStatus 更新消息状态
func (r *messageRepository) UpdateStatus(ctx context.Context, messageID string, status int) error {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"message_id": messageID}, update)
	if err != nil {
		return fmt.Errorf("failed to update message status: %w", err)
	}
	return nil
}

// Revoke 撤回消息
func (r *messageRepository) Revoke(ctx context.Context, messageID string) error {
	update := bson.M{
		"$set": bson.M{
			"revoked":    true,
			"updated_at": time.Now(),
		},
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"message_id": messageID}, update)
	if err != nil {
		return fmt.Errorf("failed to revoke message: %w", err)
	}
	return nil
}

// Delete 删除消息
func (r *messageRepository) Delete(ctx context.Context, messageID string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"message_id": messageID})
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return nil
}

// CountByConversation 统计会话消息数
func (r *messageRepository) CountByConversation(ctx context.Context, conversationID string) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{
		"conversation_id": conversationID,
		"revoked":         false,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}
	return count, nil
}

// EnsureIndexes 确保索引存在
func (r *messageRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		// 消息ID唯一索引
		{
			Keys:    bson.D{{Key: "message_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		// 会话ID + 序号复合索引（用于分页查询）
		{
			Keys: bson.D{
				{Key: "conversation_id", Value: 1},
				{Key: "seq", Value: -1},
			},
		},
		// 群组ID + 创建时间索引
		{
			Keys: bson.D{
				{Key: "group_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		// 发送者索引
		{
			Keys: bson.D{{Key: "from", Value: 1}},
		},
		// 接收者索引
		{
			Keys: bson.D{{Key: "to", Value: 1}},
		},
		// TTL索引（自动清理过期消息）
		{
			Keys:    bson.D{{Key: "expire_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
		// 创建时间索引
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}
	return nil
}
