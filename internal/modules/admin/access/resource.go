package access

import (
	"context"
	stderrors "errors"
	"log/slog"

	"github.com/cng1985/nwatch/internal/platform/auth"
	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/domain"
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/cng1985/nwatch/internal/platform/settings"
	"github.com/cng1985/nwatch/internal/platform/version"
)

// Resource 编排登录会话与用户角色管理业务。
type Resource struct {
	cfg      *config.Config
	tokens   *auth.Service
	users    *auth.Store
	settings *settings.Store
}

func NewResource(cfg *config.Config, tokens *auth.Service, store *settings.Store) *Resource {
	return &Resource{cfg: cfg, tokens: tokens, users: tokens.Users(), settings: store}
}

// Login 校验账号密码并签发会话令牌。
func (r *Resource) Login(ctx context.Context, request *LoginRequest) (response.Response[SessionResponse], error) {
	if request == nil {
		return response.Response[SessionResponse]{}, errors.BadRequest("请求格式不正确")
	}
	token, principal, err := r.tokens.Login(request.IP, request.Username, request.Password)
	if err != nil {
		if stderrors.Is(err, auth.ErrRateLimited) {
			return response.Response[SessionResponse]{}, errors.RateLimited(err.Error())
		}
		return response.Response[SessionResponse]{}, errors.Unauthorized(err.Error())
	}
	body := SessionResponse{
		Token:                token,
		Username:             principal.Username,
		DisplayName:          principal.DisplayName,
		Roles:                principal.Roles,
		Permissions:          principal.PermissionList(),
		UsingDefaultPassword: r.usingDefaultPassword(principal.Username),
	}
	return response.OK(body), nil
}

// Me 返回当前登录用户信息。
func (r *Resource) Me(ctx context.Context, principal *auth.Principal) (response.Response[SessionResponse], error) {
	if principal == nil {
		return response.Response[SessionResponse]{}, errors.Unauthorized("未登录")
	}
	body := SessionResponse{
		Username:             principal.Username,
		DisplayName:          principal.DisplayName,
		Roles:                principal.Roles,
		Permissions:          principal.PermissionList(),
		UsingDefaultPassword: r.usingDefaultPassword(principal.Username),
		Version:              version.Version,
	}
	return response.OK(body), nil
}

// ChangePassword 修改当前登录账号的密码。
func (r *Resource) ChangePassword(ctx context.Context, request *ChangePasswordRequest) (response.Response[PasswordOKResponse], error) {
	scope := domain.FromContext(ctx)
	if scope == nil || scope.ActorID == 0 {
		return response.Response[PasswordOKResponse]{}, errors.Unauthorized("未登录")
	}
	if request == nil {
		return response.Response[PasswordOKResponse]{}, errors.BadRequest("请求格式不正确")
	}
	if !r.users.CheckPassword(scope.ActorID, request.OldPassword) {
		return response.Response[PasswordOKResponse]{}, errors.BadRequest("原密码不正确")
	}
	if err := r.users.SetPassword(scope.ActorID, request.NewPassword); err != nil {
		return response.Response[PasswordOKResponse]{}, accessError(err)
	}
	r.syncBootstrapPassword(scope.Username, request.NewPassword)
	return response.OK(PasswordOKResponse{OK: true}), nil
}

// ListPermissions 返回权限目录。
func (r *Resource) ListPermissions(ctx context.Context) (response.Response[PermissionListResponse], error) {
	return response.OK(PermissionListResponse{Items: auth.Catalog()}), nil
}

// ListRoles 列出全部角色。
func (r *Resource) ListRoles(ctx context.Context) (response.Response[RoleListResponse], error) {
	roles, err := r.users.ListRoles()
	if err != nil {
		return response.Response[RoleListResponse]{}, errors.Internal("查询角色失败")
	}
	return response.OK(RoleListResponse{Items: roles}), nil
}

// CreateRole 创建角色。
func (r *Resource) CreateRole(ctx context.Context, request *RoleRequest) (response.Response[RoleResponse], error) {
	role, err := r.users.CreateRole(request.input())
	if err != nil {
		return response.Response[RoleResponse]{}, accessError(err)
	}
	r.audit(ctx, "已创建角色", role.Name)
	return response.OK(role), nil
}

// UpdateRole 修改角色。
func (r *Resource) UpdateRole(ctx context.Context, request *RoleRequest) (response.Response[RoleResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[RoleResponse]{}, errors.BadRequest("无效的编号")
	}
	role, err := r.users.UpdateRole(request.ID, request.input())
	if err != nil {
		return response.Response[RoleResponse]{}, accessError(err)
	}
	r.audit(ctx, "已更新角色", role.Name)
	return response.OK(role), nil
}

// DeleteRole 删除角色。
func (r *Resource) DeleteRole(ctx context.Context, id uint) (response.Response[OKResponse], error) {
	if id == 0 {
		return response.Response[OKResponse]{}, errors.BadRequest("无效的编号")
	}
	if err := r.users.DeleteRole(id); err != nil {
		return response.Response[OKResponse]{}, accessError(err)
	}
	r.audit(ctx, "已删除角色", "")
	return response.OK(OKResponse{OK: true}), nil
}

// ListUsers 列出全部用户。
func (r *Resource) ListUsers(ctx context.Context) (response.Response[UserListResponse], error) {
	users, err := r.users.ListUsers()
	if err != nil {
		return response.Response[UserListResponse]{}, errors.Internal("查询用户失败")
	}
	return response.OK(UserListResponse{Items: users}), nil
}

// CreateUser 创建用户。
func (r *Resource) CreateUser(ctx context.Context, request *UserRequest) (response.Response[UserResponse], error) {
	user, err := r.users.CreateUser(request.input())
	if err != nil {
		return response.Response[UserResponse]{}, accessError(err)
	}
	r.audit(ctx, "已创建用户", user.Username)
	return response.OK(user), nil
}

// UpdateUser 修改用户。修改管理员本人密码时同步引导密码记录。
func (r *Resource) UpdateUser(ctx context.Context, request *UserRequest) (response.Response[UserResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[UserResponse]{}, errors.BadRequest("无效的编号")
	}
	scope := domain.FromContext(ctx)
	actorID := uint(0)
	if scope != nil {
		actorID = scope.ActorID
	}
	user, err := r.users.UpdateUser(actorID, request.ID, request.input())
	if err != nil {
		return response.Response[UserResponse]{}, accessError(err)
	}
	if request.Password != "" {
		r.syncBootstrapPassword(user.Username, request.Password)
	}
	r.audit(ctx, "已更新用户", user.Username)
	return response.OK(user), nil
}

// DeleteUser 删除用户。
func (r *Resource) DeleteUser(ctx context.Context, id uint) (response.Response[OKResponse], error) {
	if id == 0 {
		return response.Response[OKResponse]{}, errors.BadRequest("无效的编号")
	}
	scope := domain.FromContext(ctx)
	actorID := uint(0)
	if scope != nil {
		actorID = scope.ActorID
	}
	if err := r.users.DeleteUser(actorID, id); err != nil {
		return response.Response[OKResponse]{}, accessError(err)
	}
	r.audit(ctx, "已删除用户", "")
	return response.OK(OKResponse{OK: true}), nil
}

func (r *Resource) usingDefaultPassword(username string) bool {
	return username == auth.ConfigUsername(r.cfg) && r.settings.UsingDefaultPassword()
}

func (r *Resource) syncBootstrapPassword(username, plain string) {
	if username != auth.ConfigUsername(r.cfg) {
		return
	}
	if err := r.settings.SetPassword(plain); err != nil {
		slog.Warn("同步管理员密码失败", "err", err.Error())
	}
}

func (r *Resource) audit(ctx context.Context, action, target string) {
	scope := domain.FromContext(ctx)
	user := ""
	if scope != nil {
		user = scope.Username
	}
	if target != "" {
		slog.Info(action, "target", target, "operator", user)
		return
	}
	slog.Info(action, "operator", user)
}

func (req *RoleRequest) input() auth.RoleInput {
	return auth.RoleInput{Name: req.Name, Description: req.Description, Permissions: req.Permissions}
}

func (req *UserRequest) input() auth.UserInput {
	return auth.UserInput{
		Username: req.Username, DisplayName: req.DisplayName, Password: req.Password,
		Enabled: req.Enabled, RoleIDs: req.RoleIDs,
	}
}

// accessError 把访问控制业务错误映射为平台错误分类。
func accessError(err error) error {
	switch {
	case stderrors.Is(err, auth.ErrNotFound), stderrors.Is(err, auth.ErrRoleMissing):
		return errors.NotFound(err.Error())
	case stderrors.Is(err, auth.ErrBadCredentials), stderrors.Is(err, auth.ErrDisabled):
		return errors.Unauthorized(err.Error())
	case stderrors.Is(err, auth.ErrUsernameTaken),
		stderrors.Is(err, auth.ErrRoleNameTaken),
		stderrors.Is(err, auth.ErrBuiltinDelete),
		stderrors.Is(err, auth.ErrBuiltinRename),
		stderrors.Is(err, auth.ErrRoleInUse),
		stderrors.Is(err, auth.ErrLastAdmin),
		stderrors.Is(err, auth.ErrBadUsername),
		stderrors.Is(err, auth.ErrBadPassword),
		stderrors.Is(err, auth.ErrNoRole),
		stderrors.Is(err, auth.ErrBadPerm),
		stderrors.Is(err, auth.ErrNoPerm),
		stderrors.Is(err, auth.ErrBadName),
		stderrors.Is(err, auth.ErrDisableSelf),
		stderrors.Is(err, auth.ErrDeleteSelf):
		return errors.BadRequest(err.Error())
	default:
		return errors.Internal("保存失败")
	}
}
