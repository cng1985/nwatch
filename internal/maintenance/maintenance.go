package maintenance

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/settings"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

type Service struct {
	db       *gorm.DB
	cfg      *config.Config
	settings *settings.Store
	stop     chan struct{}
}

func New(db *gorm.DB, cfg *config.Config, store *settings.Store, lc fx.Lifecycle) *Service {
	s := &Service{db: db, cfg: cfg, settings: store, stop: make(chan struct{})}
	if lc != nil {
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				go s.loop()
				return nil
			},
			OnStop: func(context.Context) error {
				close(s.stop)
				return nil
			},
		})
	}
	return s
}

func (s *Service) loop() {
	s.RunOnce()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.RunOnce()
		case <-s.stop:
			return
		}
	}
}

func (s *Service) RunOnce() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("数据维护异常", "panic", r)
		}
	}()
	snap := s.settings.Snapshot()
	now := time.Now()
	s.purge(&model.MonitorCheck{}, "checked_at", now.AddDate(0, 0, -snap.CheckRetentionDays))
	s.purge(&model.AlertEvent{}, "occurred_at", now.AddDate(0, 0, -snap.EventRetentionDays))
	s.purge(&model.NotificationLog{}, "sent_at", now.AddDate(0, 0, -snap.EventRetentionDays))
	if err := s.db.Where("bucket_type = ? AND bucket_time < ?", model.BucketMinute, now.AddDate(0, 0, -snap.MetricRetentionDays)).
		Delete(&model.MonitorMetric{}).Error; err != nil {
		slog.Error("清理分钟统计失败", "err", err.Error())
	}
	if err := s.db.Where("bucket_type = ? AND bucket_time < ?", model.BucketHour, now.AddDate(0, 0, -snap.HourMetricRetentionDays)).
		Delete(&model.MonitorMetric{}).Error; err != nil {
		slog.Error("清理小时统计失败", "err", err.Error())
	}
	if s.cfg.Backup.Enabled {
		s.maybeBackup(now)
	}
}

func (s *Service) purge(model any, column string, before time.Time) {
	if err := s.db.Where(column+" < ?", before).Delete(model).Error; err != nil {
		slog.Error("清理历史数据失败", "column", column, "err", err.Error())
	}
}

func (s *Service) maybeBackup(now time.Time) {
	loc := s.settings.Location()
	local := now.In(loc)
	if local.Hour() < s.cfg.Backup.Hour {
		return
	}
	name := fmt.Sprintf("nmonitor-%s.db", local.Format("20060102"))
	if err := os.MkdirAll(s.cfg.Backup.Dir, 0o755); err != nil {
		slog.Error("创建备份目录失败", "err", err.Error())
		return
	}
	dest := filepath.Join(s.cfg.Backup.Dir, name)
	if _, err := os.Stat(dest); err == nil {
		return
	}
	if err := BackupTo(s.db, dest); err != nil {
		slog.Error("自动备份失败", "err", err.Error())
		return
	}
	slog.Info("数据库已备份", "file", dest)
	s.pruneBackups()
}

func (s *Service) pruneBackups() {
	entries, err := os.ReadDir(s.cfg.Backup.Dir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -s.cfg.Backup.RetentionDays)
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "nmonitor-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(s.cfg.Backup.Dir, e.Name()))
		}
	}
}

func BackupTo(db *gorm.DB, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	safe := strings.ReplaceAll(dest, "'", "''")
	return db.Exec(fmt.Sprintf("VACUUM INTO '%s'", safe)).Error
}
