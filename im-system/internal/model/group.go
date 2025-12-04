// Package model 定义IM系统的数据模型
package model

import "time"

// GroupRole 群成员角色
type GroupRole int

const (
	RoleMember GroupRole = 0 // 普通成员
	RoleAdmin  GroupRole = 1 // 管理员
	RoleOwner  GroupRole = 2 // 群主
)

// GroupJoinMode 群加入模式
type GroupJoinMode int

const (
	JoinModeFree     GroupJoinMode = 0 // 自由加入
	JoinModeApproval GroupJoinMode = 1 // 需审批
	JoinModeForbid   GroupJoinMode = 2 // 禁止加入
)

// GroupStatus 群状态
type GroupStatus int

const (
	GroupStatusNormal    GroupStatus = 1 // 正常
	GroupStatusDismissed GroupStatus = 0 // 已解散
)

// Group 群组信息
type Group struct {
	GroupID      string        `json:"group_id" gorm:"primaryKey;type:varchar(64)"`
	Name         string        `json:"name" gorm:"type:varchar(128);not null"`
	Avatar       string        `json:"avatar" gorm:"type:varchar(512)"`
	Announcement string        `json:"announcement" gorm:"type:text"`
	Description  string        `json:"description" gorm:"type:varchar(512)"`
	OwnerID      string        `json:"owner_id" gorm:"type:varchar(64);index;not null"`
	MaxMembers   int           `json:"max_members" gorm:"default:500"`
	MemberCount  int           `json:"member_count" gorm:"default:0"`
	MuteAll      bool          `json:"mute_all" gorm:"default:false"` // 全员禁言
	JoinMode     GroupJoinMode `json:"join_mode" gorm:"default:0"`    // 加入模式
	Status       GroupStatus   `json:"status" gorm:"default:1"`       // 状态
	CreatedAt    time.Time     `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time     `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Group) TableName() string {
	return "groups"
}

// IsActive 判断群是否正常
func (g *Group) IsActive() bool {
	return g.Status == GroupStatusNormal
}

// IsFull 判断群是否已满
func (g *Group) IsFull() bool {
	return g.MemberCount >= g.MaxMembers
}

// CanJoinFreely 判断是否可以自由加入
func (g *Group) CanJoinFreely() bool {
	return g.JoinMode == JoinModeFree
}

// NeedApproval 判断是否需要审批
func (g *Group) NeedApproval() bool {
	return g.JoinMode == JoinModeApproval
}

// GroupMember 群成员
type GroupMember struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	GroupID   string    `json:"group_id" gorm:"type:varchar(64);uniqueIndex:idx_group_user;not null"`
	UserID    string    `json:"user_id" gorm:"type:varchar(64);uniqueIndex:idx_group_user;index;not null"`
	Role      GroupRole `json:"role" gorm:"default:0"`
	Nickname  string    `json:"nickname" gorm:"type:varchar(64)"` // 群昵称
	MuteUntil int64     `json:"mute_until" gorm:"default:0"`      // 禁言截止时间戳
	JoinedAt  time.Time `json:"joined_at" gorm:"autoCreateTime"`
	InviterID string    `json:"inviter_id" gorm:"type:varchar(64)"` // 邀请人
}

// TableName 指定表名
func (GroupMember) TableName() string {
	return "group_members"
}

// IsOwner 判断是否为群主
func (m *GroupMember) IsOwner() bool {
	return m.Role == RoleOwner
}

// IsAdmin 判断是否为管理员（包括群主）
func (m *GroupMember) IsAdmin() bool {
	return m.Role >= RoleAdmin
}

// IsMuted 判断是否被禁言
func (m *GroupMember) IsMuted() bool {
	return m.MuteUntil > time.Now().Unix()
}

// GroupWithMembers 群组及成员信息（用于查询返回）
type GroupWithMembers struct {
	Group   *Group         `json:"group"`
	Members []*GroupMember `json:"members"`
}

// GroupMemberInfo 群成员详细信息（包含用户基本信息）
type GroupMemberInfo struct {
	GroupMember
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Nickname string `json:"nickname"` // 用户昵称（非群昵称）
}

// GroupJoinRequest 入群申请
type GroupJoinRequest struct {
	ID        uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	GroupID   string     `json:"group_id" gorm:"type:varchar(64);index:idx_group_status;not null"`
	UserID    string     `json:"user_id" gorm:"type:varchar(64);index;not null"`
	Message   string     `json:"message" gorm:"type:varchar(256)"` // 申请留言
	Status    int        `json:"status" gorm:"default:0"`          // 0-待处理 1-已同意 2-已拒绝
	HandlerID string     `json:"handler_id" gorm:"type:varchar(64)"`
	HandledAt *time.Time `json:"handled_at"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定表名
func (GroupJoinRequest) TableName() string {
	return "group_join_requests"
}

// GroupJoinRequestStatus 入群申请状态
const (
	JoinRequestPending  = 0 // 待处理
	JoinRequestApproved = 1 // 已同意
	JoinRequestRejected = 2 // 已拒绝
)

// CreateGroupRequest 创建群组请求
type CreateGroupRequest struct {
	OwnerID     string   `json:"owner_id" binding:"required"`
	Name        string   `json:"name" binding:"required,max=128"`
	Avatar      string   `json:"avatar"`
	Description string   `json:"description" binding:"max=512"`
	MemberIDs   []string `json:"member_ids"` // 初始成员
}

// UpdateGroupRequest 更新群组请求
type UpdateGroupRequest struct {
	GroupID      string  `json:"group_id" binding:"required"`
	OperatorID   string  `json:"operator_id" binding:"required"`
	Name         *string `json:"name,omitempty"`
	Avatar       *string `json:"avatar,omitempty"`
	Announcement *string `json:"announcement,omitempty"`
	Description  *string `json:"description,omitempty"`
	JoinMode     *int    `json:"join_mode,omitempty"`
}

// GroupMemberListResponse 群成员列表响应
type GroupMemberListResponse struct {
	Total   int                `json:"total"`
	Members []*GroupMemberInfo `json:"members"`
}

// UserGroupsResponse 用户所在群组列表响应
type UserGroupsResponse struct {
	Total  int      `json:"total"`
	Groups []*Group `json:"groups"`
}
