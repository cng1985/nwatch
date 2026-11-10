package backup

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/cng1985/nwatch/internal/modules/jobs/cronjob"
	"github.com/cng1985/nwatch/internal/modules/monitoring/maintenance"
	"github.com/cng1985/nwatch/internal/platform/config"
	apperrors "github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/cng1985/nwatch/internal/platform/settings"
	"gorm.io/gorm"
)

// Resource 编排备份与配置导入导出业务。
type Resource struct {
	cfg   *config.Config
	db    *gorm.DB
	store *settings.Store
	cron  *cronjob.Service
}

func NewResource(cfg *config.Config, db *gorm.DB, store *settings.Store, cron *cronjob.Service) *Resource {
	return &Resource{cfg: cfg, db: db, store: store, cron: cron}
}

// CreateBackup 在备份目录生成一份手动备份。
func (r *Resource) CreateBackup(ctx context.Context) (response.Response[BackupFileResponse], error) {
	if err := os.MkdirAll(r.cfg.Backup.Dir, 0o755); err != nil {
		return response.Response[BackupFileResponse]{}, apperrors.Internal("创建备份目录失败")
	}
	name := "nmonitor-manual-" + time.Now().In(r.store.Location()).Format("20060102-150405") + ".db"
	dest := filepath.Join(r.cfg.Backup.Dir, name)
	if err := maintenance.BackupTo(r.db, dest); err != nil {
		return response.Response[BackupFileResponse]{}, apperrors.Internal("备份失败")
	}
	return response.OK(BackupFileResponse{File: dest}), nil
}

// DownloadBackup 生成临时备份并交给调用方按下载协议输出。
func (r *Resource) DownloadBackup(ctx context.Context) (string, string, error) {
	dest := filepath.Join(os.TempDir(), "nmonitor-backup-"+time.Now().Format("20060102-150405")+".db")
	if err := maintenance.BackupTo(r.db, dest); err != nil {
		return "", "", apperrors.Internal("备份失败")
	}
	return dest, filepath.Base(dest), nil
}

// Export 导出全部配置。
func (r *Resource) Export(ctx context.Context, request *ExportRequest) (response.Response[ExportResponse], error) {
	bundle, err := Export(r.db.WithContext(ctx), request.IncludeSecrets)
	if err != nil {
		return response.Response[ExportResponse]{}, apperrors.Internal("导出失败")
	}
	return response.OK(bundle), nil
}

// Import 导入配置并重算定时任务。
func (r *Resource) Import(ctx context.Context, request *ImportRequest) (response.Response[BackupOKResponse], error) {
	if request == nil || request.Bundle.Version != 1 {
		return response.Response[BackupOKResponse]{}, apperrors.BadRequest("不支持的配置版本")
	}
	if err := Import(r.db.WithContext(ctx), request.Bundle); err != nil {
		return response.Response[BackupOKResponse]{}, apperrors.BadRequest("导入失败：" + err.Error())
	}
	if err := r.cron.RescheduleAll(); err != nil {
		return response.Response[BackupOKResponse]{}, apperrors.Internal("导入后重算定时任务失败")
	}
	return response.OK(BackupOKResponse{OK: true}), nil
}
