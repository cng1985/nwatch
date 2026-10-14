package rbac

import "errors"

var (
	ErrBadCredentials = errors.New("用户名或密码错误")
	ErrDisabled       = errors.New("账号已停用")
	ErrNotFound       = errors.New("用户不存在")
	ErrRoleMissing    = errors.New("角色不存在")
	ErrUsernameTaken  = errors.New("用户名已存在")
	ErrRoleNameTaken  = errors.New("角色名称已存在")
	ErrBuiltinDelete  = errors.New("内置角色不能删除")
	ErrBuiltinRename  = errors.New("内置角色不能改名")
	ErrRoleInUse      = errors.New("仍有用户使用该角色")
	ErrLastAdmin      = errors.New("至少保留一个可管理用户与角色的启用账号")
	ErrBadUsername    = errors.New("用户名需以字母开头，2 到 32 位，只能包含字母、数字、点、下划线和中划线")
	ErrBadPassword    = errors.New("密码需要 6 到 72 位")
	ErrNoRole         = errors.New("请至少选择一个角色")
	ErrBadPerm        = errors.New("包含未知权限")
	ErrNoPerm         = errors.New("请至少选择一项权限")
	ErrBadName        = errors.New("请填写角色名称，最长 32 个字符")
	ErrDisableSelf    = errors.New("不能停用当前登录账号")
	ErrDeleteSelf     = errors.New("不能删除当前登录账号")
)
