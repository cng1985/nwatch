// Package access 提供用户、角色与登录会话管理业务。
package access

import (
	"github.com/cng1985/nwatch/internal/platform/auth"
)

// LoginRequest 是登录参数。IP 由服务端从可信连接地址填充。
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IP       string `json:"-"`
}

// SessionResponse 是登录会话与当前用户信息。
type SessionResponse struct {
	Token                string           `json:"token,omitempty"`
	Username             string           `json:"username"`
	DisplayName          string           `json:"displayName"`
	Roles                []auth.RoleBrief `json:"roles"`
	Permissions          []string         `json:"permissions"`
	UsingDefaultPassword bool             `json:"usingDefaultPassword"`
	Version              string           `json:"version,omitempty"`
}

// ChangePasswordRequest 是修改当前账号密码的参数。
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// PasswordOKResponse 是修改密码结果。
type PasswordOKResponse struct {
	OK bool `json:"ok"`
}

// UserRequest 是创建或修改用户的参数。
type UserRequest struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Enabled     *bool  `json:"enabled"`
	RoleIDs     []uint `json:"roleIds"`
}

// UserListResponse 是用户列表结果。
type UserListResponse struct {
	Items []auth.UserView `json:"items"`
}

// UserResponse 是用户业务结果。
type UserResponse = auth.UserView

// RoleRequest 是创建或修改角色的参数。
type RoleRequest struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// RoleListResponse 是角色列表结果。
type RoleListResponse struct {
	Items []auth.RoleView `json:"items"`
}

// RoleResponse 是角色业务结果。
type RoleResponse = auth.RoleView

// PermissionListResponse 是权限目录结果。
type PermissionListResponse struct {
	Items []auth.PermGroup `json:"items"`
}

// OKResponse 是删除等动作回执。
type OKResponse struct {
	OK bool `json:"ok"`
}
