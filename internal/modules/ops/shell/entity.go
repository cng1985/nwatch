package shell

import "time"

// ShellCommand 是在线终端里提交的一条命令，用于审计。
type ShellCommand struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:64;index" json:"username"`
	SessionID string    `gorm:"size:32;index" json:"sessionId"`
	Remote    string    `gorm:"size:64" json:"remote"`
	Command   string    `gorm:"size:4000" json:"command"`
	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

func (ShellCommand) TableName() string { return "shell_commands" }
