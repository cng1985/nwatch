package auth

import "time"

// User 是管理台登录账号，可以同时拥有多个角色。
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;not null;uniqueIndex" json:"username"`
	DisplayName  string    `gorm:"size:64" json:"displayName"`
	PasswordHash string    `gorm:"size:128" json:"-"`
	Enabled      bool      `gorm:"not null;index" json:"enabled"`
	Roles        []Role    `gorm:"many2many:user_roles;" json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (User) TableName() string { return "users" }

// Role 是一组权限。内置角色的 Key 固定，自定义角色使用随机 Key。
type Role struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Key         string    `gorm:"size:32;not null;uniqueIndex" json:"key"`
	Name        string    `gorm:"size:64;not null;uniqueIndex" json:"name"`
	Description string    `gorm:"size:512" json:"description"`
	Builtin     bool      `gorm:"not null" json:"builtin"`
	Permissions string    `gorm:"type:text" json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (Role) TableName() string { return "roles" }
