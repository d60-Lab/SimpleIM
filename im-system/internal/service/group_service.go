// Package service 提供业务逻辑服务
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/d60-lab/im-system/internal/model"
	"github.com/d60-lab/im-system/pkg/util"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// 群组服务错误定义
var (
	ErrGroupNotFound   = errors.New("group not found")
	ErrNotGroupMember  = errors.New("not a group member")
	ErrNotGroupOwner   = errors.New("not group owner")
	ErrNotGroupAdmin   = errors.New("not group admin or owner")
	ErrGroupFull       = errors.New("group is full")
	ErrAlreadyInGroup  = errors.New("already in group")
	ErrCannotKickOwner = errors.New("cannot kick group owner")
	ErrGroupDismissed  = errors.New("group has been dismissed")
	ErrPermissionDeny  = errors.New("permission denied")
	ErrInvalidRequest  = errors.New("invalid request")
)

// GroupService 群组服务接口
type GroupService interface {
	// 群组操作
	CreateGroup(ctx context.Context, req *model.CreateGroupRequest) (*model.Group, error)
	DismissGroup(ctx context.Context, groupID, operatorID string) error
	GetGroupInfo(ctx context.Context, groupID string) (*model.Group, error)
	UpdateGroupInfo(ctx context.Context, req *model.UpdateGroupRequest) error

	// 成员管理
	JoinGroup(ctx context.Context, groupID, userID, inviterID string) error
	LeaveGroup(ctx context.Context, groupID, userID string) error
	KickMember(ctx context.Context, groupID, operatorID string, targetIDs []string) error
	GetGroupMembers(ctx context.Context, groupID string, page, pageSize int) ([]*model.GroupMember, int64, error)

	// 管理员操作
	SetAdmin(ctx context.Context, groupID, operatorID, targetID string, isAdmin bool) error
	TransferOwner(ctx context.Context, groupID, ownerID, newOwnerID string) error
	MuteMember(ctx context.Context, groupID, operatorID, targetID string, duration time.Duration) error
	SetMuteAll(ctx context.Context, groupID, operatorID string, muteAll bool) error

	// 查询
	GetUserGroups(ctx context.Context, userID string) ([]*model.Group, error)
	IsMember(ctx context.Context, groupID, userID string) (bool, error)
	GetMemberRole(ctx context.Context, groupID, userID string) (model.GroupRole, error)
	GetGroupMemberIDs(ctx context.Context, groupID string) ([]string, error)
}

// MessageDispatcher 消息分发器接口（用于发送群通知）
type MessageDispatcher interface {
	DispatchToUsers(ctx context.Context, userIDs []string, msg *model.Message) error
}

// groupServiceImpl 群组服务实现
type groupServiceImpl struct {
	db            *gorm.DB
	redis         *redis.Client
	msgDispatcher MessageDispatcher
}

// NewGroupService 创建群组服务
func NewGroupService(db *gorm.DB, redisClient *redis.Client, dispatcher MessageDispatcher) GroupService {
	return &groupServiceImpl{
		db:            db,
		redis:         redisClient,
		msgDispatcher: dispatcher,
	}
}

// CreateGroup 创建群组
func (s *groupServiceImpl) CreateGroup(ctx context.Context, req *model.CreateGroupRequest) (*model.Group, error) {
	if req.Name == "" {
		return nil, ErrInvalidRequest
	}

	groupID := util.GenerateGroupID()
	now := time.Now()

	group := &model.Group{
		GroupID:     groupID,
		Name:        req.Name,
		Avatar:      req.Avatar,
		Description: req.Description,
		OwnerID:     req.OwnerID,
		MaxMembers:  500,
		MemberCount: 1,
		JoinMode:    model.JoinModeFree,
		Status:      model.GroupStatusNormal,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 开启事务
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 创建群组
		if err := tx.Create(group).Error; err != nil {
			return fmt.Errorf("create group error: %w", err)
		}

		// 添加群主为成员
		ownerMember := &model.GroupMember{
			GroupID:  groupID,
			UserID:   req.OwnerID,
			Role:     model.RoleOwner,
			JoinedAt: now,
		}
		if err := tx.Create(ownerMember).Error; err != nil {
			return fmt.Errorf("add owner member error: %w", err)
		}

		// 添加初始成员
		if len(req.MemberIDs) > 0 {
			memberIDs := uniqueStrings(req.MemberIDs)
			members := make([]*model.GroupMember, 0, len(memberIDs))

			for _, memberID := range memberIDs {
				if memberID == req.OwnerID {
					continue // 跳过群主
				}
				members = append(members, &model.GroupMember{
					GroupID:   groupID,
					UserID:    memberID,
					Role:      model.RoleMember,
					InviterID: req.OwnerID,
					JoinedAt:  now,
				})
			}

			if len(members) > 0 {
				if err := tx.Create(&members).Error; err != nil {
					return fmt.Errorf("add initial members error: %w", err)
				}

				// 更新成员数
				group.MemberCount = 1 + len(members)
				if err := tx.Model(group).Update("member_count", group.MemberCount).Error; err != nil {
					return fmt.Errorf("update member count error: %w", err)
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 同步群成员到Redis
	if err := s.syncGroupMembersToRedis(ctx, groupID); err != nil {
		// 记录错误但不影响返回
		fmt.Printf("sync group members to redis error: %v\n", err)
	}

	// 发送群创建通知
	s.notifyGroupEvent(ctx, model.MsgGroupCreated, groupID, req.OwnerID, nil, nil)

	return group, nil
}

// DismissGroup 解散群组
func (s *groupServiceImpl) DismissGroup(ctx context.Context, groupID, operatorID string) error {
	// 检查是否为群主
	role, err := s.GetMemberRole(ctx, groupID, operatorID)
	if err != nil {
		return err
	}
	if role != model.RoleOwner {
		return ErrNotGroupOwner
	}

	// 获取所有成员ID（用于发送通知）
	memberIDs, err := s.GetGroupMemberIDs(ctx, groupID)
	if err != nil {
		return err
	}

	// 开启事务
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 更新群组状态
		if err := tx.Model(&model.Group{}).Where("group_id = ?", groupID).
			Update("status", model.GroupStatusDismissed).Error; err != nil {
			return fmt.Errorf("update group status error: %w", err)
		}

		// 删除所有群成员
		if err := tx.Where("group_id = ?", groupID).Delete(&model.GroupMember{}).Error; err != nil {
			return fmt.Errorf("delete group members error: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 清理Redis中的群成员
	groupKey := fmt.Sprintf("group:members:%s", groupID)
	s.redis.Del(ctx, groupKey)

	// 发送群解散通知
	s.notifyGroupEvent(ctx, model.MsgGroupDismissed, groupID, operatorID, memberIDs, nil)

	return nil
}

// GetGroupInfo 获取群信息
func (s *groupServiceImpl) GetGroupInfo(ctx context.Context, groupID string) (*model.Group, error) {
	var group model.Group
	if err := s.db.WithContext(ctx).Where("group_id = ?", groupID).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}

	if group.Status == model.GroupStatusDismissed {
		return nil, ErrGroupDismissed
	}

	return &group, nil
}

// UpdateGroupInfo 更新群信息
func (s *groupServiceImpl) UpdateGroupInfo(ctx context.Context, req *model.UpdateGroupRequest) error {
	// 检查操作权限（需要管理员或群主）
	role, err := s.GetMemberRole(ctx, req.GroupID, req.OperatorID)
	if err != nil {
		return err
	}
	if role < model.RoleAdmin {
		return ErrNotGroupAdmin
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	changes := make([]map[string]string, 0)

	if req.Name != nil {
		updates["name"] = *req.Name
		changes = append(changes, map[string]string{
			"field":     "name",
			"new_value": *req.Name,
		})
	}
	if req.Avatar != nil {
		updates["avatar"] = *req.Avatar
		changes = append(changes, map[string]string{
			"field":     "avatar",
			"new_value": *req.Avatar,
		})
	}
	if req.Announcement != nil {
		updates["announcement"] = *req.Announcement
		changes = append(changes, map[string]string{
			"field":     "announcement",
			"new_value": *req.Announcement,
		})
	}
	if req.Description != nil {
		updates["description"] = *req.Description
		changes = append(changes, map[string]string{
			"field":     "description",
			"new_value": *req.Description,
		})
	}
	if req.JoinMode != nil {
		updates["join_mode"] = *req.JoinMode
		changes = append(changes, map[string]string{
			"field":     "join_mode",
			"new_value": fmt.Sprintf("%d", *req.JoinMode),
		})
	}

	if len(updates) == 0 {
		return nil
	}

	updates["updated_at"] = time.Now()

	// 执行更新
	result := s.db.WithContext(ctx).Model(&model.Group{}).
		Where("group_id = ?", req.GroupID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("update group info error: %w", result.Error)
	}

	// 发送群信息更新通知
	for _, change := range changes {
		extra := map[string]string{
			"field":     change["field"],
			"new_value": change["new_value"],
		}
		s.notifyGroupEvent(ctx, model.MsgGroupInfoUpdate, req.GroupID, req.OperatorID, nil, extra)
	}

	return nil
}

// JoinGroup 加入群组
func (s *groupServiceImpl) JoinGroup(ctx context.Context, groupID, userID, inviterID string) error {
	// 获取群信息
	group, err := s.GetGroupInfo(ctx, groupID)
	if err != nil {
		return err
	}

	// 检查群是否已满
	if group.IsFull() {
		return ErrGroupFull
	}

	// 检查是否已是成员
	isMember, err := s.IsMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if isMember {
		return ErrAlreadyInGroup
	}

	// TODO: 如果需要审批，创建加入申请而不是直接加入
	if group.NeedApproval() {
		// 创建加入申请
		return s.createJoinRequest(ctx, groupID, userID, "")
	}

	// 开启事务
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 添加成员
		member := &model.GroupMember{
			GroupID:   groupID,
			UserID:    userID,
			Role:      model.RoleMember,
			InviterID: inviterID,
			JoinedAt:  time.Now(),
		}
		if err := tx.Create(member).Error; err != nil {
			return fmt.Errorf("create member error: %w", err)
		}

		// 更新成员数
		if err := tx.Model(&model.Group{}).Where("group_id = ?", groupID).
			UpdateColumn("member_count", gorm.Expr("member_count + ?", 1)).Error; err != nil {
			return fmt.Errorf("update member count error: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 更新Redis中的群成员
	groupKey := fmt.Sprintf("group:members:%s", groupID)
	s.redis.SAdd(ctx, groupKey, userID)

	// 发送成员加入通知
	s.notifyGroupEvent(ctx, model.MsgGroupMemberJoin, groupID, userID, []string{userID}, nil)

	return nil
}

// createJoinRequest 创建加入申请
func (s *groupServiceImpl) createJoinRequest(ctx context.Context, groupID, userID, message string) error {
	request := &model.GroupJoinRequest{
		GroupID:   groupID,
		UserID:    userID,
		Message:   message,
		Status:    model.JoinRequestPending,
		CreatedAt: time.Now(),
	}
	return s.db.WithContext(ctx).Create(request).Error
}

// LeaveGroup 离开群组
func (s *groupServiceImpl) LeaveGroup(ctx context.Context, groupID, userID string) error {
	// 检查是否为成员
	role, err := s.GetMemberRole(ctx, groupID, userID)
	if err != nil {
		return err
	}

	// 群主不能直接离开，需要先转让
	if role == model.RoleOwner {
		return errors.New("group owner cannot leave, please transfer ownership first")
	}

	// 开启事务
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除成员
		if err := tx.Where("group_id = ? AND user_id = ?", groupID, userID).
			Delete(&model.GroupMember{}).Error; err != nil {
			return fmt.Errorf("delete member error: %w", err)
		}

		// 更新成员数
		if err := tx.Model(&model.Group{}).Where("group_id = ?", groupID).
			UpdateColumn("member_count", gorm.Expr("member_count - ?", 1)).Error; err != nil {
			return fmt.Errorf("update member count error: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 更新Redis中的群成员
	groupKey := fmt.Sprintf("group:members:%s", groupID)
	s.redis.SRem(ctx, groupKey, userID)

	// 发送成员离开通知
	s.notifyGroupEvent(ctx, model.MsgGroupMemberLeave, groupID, userID, []string{userID}, nil)

	return nil
}

// KickMember 踢出成员
func (s *groupServiceImpl) KickMember(ctx context.Context, groupID, operatorID string, targetIDs []string) error {
	// 检查操作者权限
	operatorRole, err := s.GetMemberRole(ctx, groupID, operatorID)
	if err != nil {
		return err
	}
	if operatorRole < model.RoleAdmin {
		return ErrNotGroupAdmin
	}

	// 检查目标用户
	for _, targetID := range targetIDs {
		targetRole, err := s.GetMemberRole(ctx, groupID, targetID)
		if err != nil {
			continue // 跳过不存在的成员
		}

		// 不能踢群主
		if targetRole == model.RoleOwner {
			return ErrCannotKickOwner
		}

		// 管理员只能踢普通成员，群主可以踢所有人
		if operatorRole == model.RoleAdmin && targetRole >= model.RoleAdmin {
			return ErrPermissionDeny
		}
	}

	// 开启事务
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 批量删除成员
		if err := tx.Where("group_id = ? AND user_id IN ?", groupID, targetIDs).
			Delete(&model.GroupMember{}).Error; err != nil {
			return fmt.Errorf("delete members error: %w", err)
		}

		// 更新成员数
		if err := tx.Model(&model.Group{}).Where("group_id = ?", groupID).
			UpdateColumn("member_count", gorm.Expr("member_count - ?", len(targetIDs))).Error; err != nil {
			return fmt.Errorf("update member count error: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 更新Redis中的群成员
	groupKey := fmt.Sprintf("group:members:%s", groupID)
	for _, targetID := range targetIDs {
		s.redis.SRem(ctx, groupKey, targetID)
	}

	// 发送成员被踢通知
	s.notifyGroupEvent(ctx, model.MsgGroupMemberKicked, groupID, operatorID, targetIDs, nil)

	return nil
}

// GetGroupMembers 获取群成员列表
func (s *groupServiceImpl) GetGroupMembers(ctx context.Context, groupID string, page, pageSize int) ([]*model.GroupMember, int64, error) {
	var members []*model.GroupMember
	var total int64

	// 计算偏移量
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	// 查询总数
	if err := s.db.WithContext(ctx).Model(&model.GroupMember{}).
		Where("group_id = ?", groupID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 查询成员列表（按角色降序，群主在前）
	if err := s.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("role DESC, joined_at ASC").
		Offset(offset).
		Limit(pageSize).
		Find(&members).Error; err != nil {
		return nil, 0, err
	}

	return members, total, nil
}

// SetAdmin 设置/取消管理员
func (s *groupServiceImpl) SetAdmin(ctx context.Context, groupID, operatorID, targetID string, isAdmin bool) error {
	// 只有群主可以设置管理员
	operatorRole, err := s.GetMemberRole(ctx, groupID, operatorID)
	if err != nil {
		return err
	}
	if operatorRole != model.RoleOwner {
		return ErrNotGroupOwner
	}

	// 检查目标用户是否为成员
	targetRole, err := s.GetMemberRole(ctx, groupID, targetID)
	if err != nil {
		return err
	}
	if targetRole == model.RoleOwner {
		return errors.New("cannot change owner's role")
	}

	// 更新角色
	newRole := model.RoleMember
	if isAdmin {
		newRole = model.RoleAdmin
	}

	if err := s.db.WithContext(ctx).Model(&model.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, targetID).
		Update("role", newRole).Error; err != nil {
		return err
	}

	// 发送管理员变更通知
	extra := map[string]string{
		"action": "set_admin",
	}
	if !isAdmin {
		extra["action"] = "remove_admin"
	}
	s.notifyGroupEvent(ctx, model.MsgGroupAdminChange, groupID, operatorID, []string{targetID}, extra)

	return nil
}

// TransferOwner 转让群主
func (s *groupServiceImpl) TransferOwner(ctx context.Context, groupID, ownerID, newOwnerID string) error {
	// 检查是否为群主
	role, err := s.GetMemberRole(ctx, groupID, ownerID)
	if err != nil {
		return err
	}
	if role != model.RoleOwner {
		return ErrNotGroupOwner
	}

	// 检查新群主是否为成员
	isMember, err := s.IsMember(ctx, groupID, newOwnerID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrNotGroupMember
	}

	// 开启事务
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 原群主变为管理员
		if err := tx.Model(&model.GroupMember{}).
			Where("group_id = ? AND user_id = ?", groupID, ownerID).
			Update("role", model.RoleAdmin).Error; err != nil {
			return err
		}

		// 新群主
		if err := tx.Model(&model.GroupMember{}).
			Where("group_id = ? AND user_id = ?", groupID, newOwnerID).
			Update("role", model.RoleOwner).Error; err != nil {
			return err
		}

		// 更新群组的owner_id
		if err := tx.Model(&model.Group{}).
			Where("group_id = ?", groupID).
			Update("owner_id", newOwnerID).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 发送群主转让通知
	s.notifyGroupEvent(ctx, model.MsgGroupTransfer, groupID, ownerID, []string{newOwnerID}, nil)

	return nil
}

// MuteMember 禁言成员
func (s *groupServiceImpl) MuteMember(ctx context.Context, groupID, operatorID, targetID string, duration time.Duration) error {
	// 检查操作者权限
	operatorRole, err := s.GetMemberRole(ctx, groupID, operatorID)
	if err != nil {
		return err
	}
	if operatorRole < model.RoleAdmin {
		return ErrNotGroupAdmin
	}

	// 检查目标用户
	targetRole, err := s.GetMemberRole(ctx, groupID, targetID)
	if err != nil {
		return err
	}

	// 不能禁言群主或同级
	if targetRole >= operatorRole {
		return ErrPermissionDeny
	}

	// 计算禁言截止时间
	muteUntil := time.Now().Add(duration).Unix()
	if duration == 0 {
		muteUntil = 0 // 取消禁言
	}

	if err := s.db.WithContext(ctx).Model(&model.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, targetID).
		Update("mute_until", muteUntil).Error; err != nil {
		return err
	}

	// 发送禁言通知
	extra := map[string]string{
		"duration": fmt.Sprintf("%d", int(duration.Seconds())),
	}
	s.notifyGroupEvent(ctx, model.MsgGroupMute, groupID, operatorID, []string{targetID}, extra)

	return nil
}

// SetMuteAll 设置全员禁言
func (s *groupServiceImpl) SetMuteAll(ctx context.Context, groupID, operatorID string, muteAll bool) error {
	// 检查操作者权限
	role, err := s.GetMemberRole(ctx, groupID, operatorID)
	if err != nil {
		return err
	}
	if role < model.RoleAdmin {
		return ErrNotGroupAdmin
	}

	if err := s.db.WithContext(ctx).Model(&model.Group{}).
		Where("group_id = ?", groupID).
		Update("mute_all", muteAll).Error; err != nil {
		return err
	}

	// 发送全员禁言通知
	extra := map[string]string{
		"mute_all": fmt.Sprintf("%t", muteAll),
	}
	s.notifyGroupEvent(ctx, model.MsgGroupMute, groupID, operatorID, nil, extra)

	return nil
}

// GetUserGroups 获取用户所在的群组列表
func (s *groupServiceImpl) GetUserGroups(ctx context.Context, userID string) ([]*model.Group, error) {
	var groups []*model.Group

	// 子查询获取用户所在的群组ID
	subQuery := s.db.WithContext(ctx).Model(&model.GroupMember{}).
		Select("group_id").
		Where("user_id = ?", userID)

	if err := s.db.WithContext(ctx).
		Where("group_id IN (?) AND status = ?", subQuery, model.GroupStatusNormal).
		Order("updated_at DESC").
		Find(&groups).Error; err != nil {
		return nil, err
	}

	return groups, nil
}

// IsMember 检查用户是否为群成员
func (s *groupServiceImpl) IsMember(ctx context.Context, groupID, userID string) (bool, error) {
	// 先从Redis检查
	groupKey := fmt.Sprintf("group:members:%s", groupID)
	exists, err := s.redis.SIsMember(ctx, groupKey, userID).Result()
	if err == nil && exists {
		return true, nil
	}

	// Redis没有或出错，从数据库检查
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetMemberRole 获取成员角色
func (s *groupServiceImpl) GetMemberRole(ctx context.Context, groupID, userID string) (model.GroupRole, error) {
	var member model.GroupMember
	if err := s.db.WithContext(ctx).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrNotGroupMember
		}
		return 0, err
	}
	return member.Role, nil
}

// GetGroupMemberIDs 获取群所有成员ID
func (s *groupServiceImpl) GetGroupMemberIDs(ctx context.Context, groupID string) ([]string, error) {
	// 先从Redis获取
	groupKey := fmt.Sprintf("group:members:%s", groupID)
	members, err := s.redis.SMembers(ctx, groupKey).Result()
	if err == nil && len(members) > 0 {
		return members, nil
	}

	// 从数据库获取
	var memberIDs []string
	if err := s.db.WithContext(ctx).Model(&model.GroupMember{}).
		Where("group_id = ?", groupID).
		Pluck("user_id", &memberIDs).Error; err != nil {
		return nil, err
	}

	// 同步到Redis
	if len(memberIDs) > 0 {
		s.redis.SAdd(ctx, groupKey, stringsToInterfaces(memberIDs)...)
		s.redis.Expire(ctx, groupKey, 24*time.Hour)
	}

	return memberIDs, nil
}

// syncGroupMembersToRedis 同步群成员到Redis
func (s *groupServiceImpl) syncGroupMembersToRedis(ctx context.Context, groupID string) error {
	memberIDs, err := s.GetGroupMemberIDs(ctx, groupID)
	if err != nil {
		return err
	}

	if len(memberIDs) == 0 {
		return nil
	}

	groupKey := fmt.Sprintf("group:members:%s", groupID)

	// 先删除旧数据
	s.redis.Del(ctx, groupKey)

	// 添加新数据
	s.redis.SAdd(ctx, groupKey, stringsToInterfaces(memberIDs)...)
	s.redis.Expire(ctx, groupKey, 24*time.Hour)

	return nil
}

// notifyGroupEvent 发送群事件通知
func (s *groupServiceImpl) notifyGroupEvent(ctx context.Context, eventType model.MessageType, groupID, operatorID string, targetIDs []string, extra map[string]string) {
	if s.msgDispatcher == nil {
		return
	}

	// 获取群成员
	memberIDs, err := s.GetGroupMemberIDs(ctx, groupID)
	if err != nil {
		fmt.Printf("get group member IDs error: %v\n", err)
		return
	}

	// 构建消息
	msg := model.NewGroupEventMessage(eventType, groupID, operatorID, targetIDs)
	if extra != nil {
		if content, ok := msg.Content.(*model.GroupEventContent); ok {
			content.Extra = extra
		}
	}

	// 分发给所有群成员
	if err := s.msgDispatcher.DispatchToUsers(ctx, memberIDs, msg); err != nil {
		fmt.Printf("dispatch group event error: %v\n", err)
	}
}

// uniqueStrings 字符串去重
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(strs))
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// stringsToInterfaces 字符串切片转接口切片
func stringsToInterfaces(strs []string) []interface{} {
	result := make([]interface{}, len(strs))
	for i, s := range strs {
		result[i] = s
	}
	return result
}
