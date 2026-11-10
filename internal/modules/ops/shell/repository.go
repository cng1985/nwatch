package shell

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/platform/response"
	"gorm.io/gorm"
)

// Repository 负责命令审计的持久化与查询。
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Record 保存一条终端命令审计记录。
func (r *Repository) Record(ctx context.Context, username, remote, sessionID, command string) error {
	row := ShellCommand{
		Username:  username,
		Remote:    remote,
		SessionID: sessionID,
		Command:   command,
		CreatedAt: time.Now(),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	slog.Info("在线终端执行命令: "+command, "user", username, "remote", remote, "session", sessionID)
	return nil
}

// Search 分页查询命令审计。
func (r *Repository) Search(ctx context.Context, search *ShellCommandSearch) (response.PageResponse[ShellCommandSimple], error) {
	var result response.PageResponse[ShellCommandSimple]
	q := r.db.WithContext(ctx).Model(&ShellCommand{})
	if userName := strings.TrimSpace(search.Username); userName != "" {
		q = q.Where("username = ?", userName)
	}
	if kw := strings.TrimSpace(search.Keyword); kw != "" {
		q = q.Where("command LIKE ?", "%"+kw+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return result, err
	}
	items := make([]ShellCommand, 0)
	if err := q.Order("created_at desc, id desc").Offset((search.Page - 1) * search.PageSize).Limit(search.PageSize).Find(&items).Error; err != nil {
		return result, err
	}
	return response.Page(items, total, search.Page, search.PageSize), nil
}

// RecordCommand 是 Hub 的命令记录回调。失败仅记日志，不影响会话。
func (r *Repository) RecordCommand(username, remote, sessionID, command string) {
	if err := r.Record(context.Background(), username, remote, sessionID, command); err != nil {
		slog.Error("记录终端命令失败", "err", err.Error(), "user", username, "session", sessionID)
	}
}
