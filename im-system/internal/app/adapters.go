// Package app 应用初始化
package app

import (
	"context"

	"github.com/d60-lab/im-system/internal/gateway"
	"github.com/d60-lab/im-system/internal/model"
	"github.com/d60-lab/im-system/internal/service"
)

// groupMemberGetterAdapter 群成员获取器适配器
type groupMemberGetterAdapter struct {
	groupService service.GroupService
}

// GetGroupMemberIDs 获取群成员ID列表
func (a *groupMemberGetterAdapter) GetGroupMemberIDs(ctx context.Context, groupID string) ([]string, error) {
	if a.groupService == nil {
		return nil, nil
	}
	return a.groupService.GetGroupMemberIDs(ctx, groupID)
}

// messageDispatcherAdapter 消息分发器适配器
type messageDispatcherAdapter struct {
	dispatcher gateway.MessageDispatcher
}

// DispatchToUsers 分发消息给指定用户
func (a *messageDispatcherAdapter) DispatchToUsers(ctx context.Context, userIDs []string, msg *model.Message) error {
	return a.dispatcher.DispatchToUsers(ctx, userIDs, msg)
}

// messageSaverAdapter 消息保存适配器
type messageSaverAdapter struct {
	messageService service.MessageService
}

// SaveMessage 保存消息
func (a *messageSaverAdapter) SaveMessage(ctx context.Context, msg *model.Message) error {
	return a.messageService.SaveMessage(ctx, msg)
}
