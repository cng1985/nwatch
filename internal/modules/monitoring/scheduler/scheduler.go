package scheduler

import (
	"context"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"go.uber.org/fx"
	"gorm.io/gorm"
	"log/slog"
	"sync/atomic"
	"time"
)

type Scheduler struct {
	db       *gorm.DB
	pool     *Pool
	interval time.Duration
	stop     chan struct{}
	running  atomic.Bool
}

func NewScheduler(scanInterval time.Duration, db *gorm.DB, pool *Pool, lc fx.Lifecycle) *Scheduler {
	if scanInterval <= 0 {
		scanInterval = time.Second
	}
	s := &Scheduler{
		db:       db,
		pool:     pool,
		interval: scanInterval,
		stop:     make(chan struct{}),
	}
	if lc != nil {
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				s.Start()
				return nil
			},
			OnStop: func(context.Context) error {
				s.Stop()
				return nil
			},
		})
	}
	return s
}

func (s *Scheduler) Running() bool { return s.running.Load() }

func (s *Scheduler) Start() { go s.loop() }

func (s *Scheduler) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
}

func (s *Scheduler) loop() {
	s.running.Store(true)
	defer s.running.Store(false)
	slog.Info("调度器已启动", "interval", s.interval.String())
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	s.scan()
	for {
		select {
		case <-ticker.C:
			s.scan()
		case <-s.stop:
			slog.Info("调度器已停止")
			return
		}
	}
}

func (s *Scheduler) scan() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("调度扫描异常", "panic", r)
		}
	}()
	now := time.Now()
	var ids []uint
	err := s.db.Model(&shared.Monitor{}).
		Where("enabled = ? AND (next_check_at IS NULL OR next_check_at <= ?)", true, now).
		Order("next_check_at").
		Limit(100).
		Pluck("id", &ids).Error
	if err != nil {
		slog.Error("查询待检测监控失败", "err", err.Error())
		return
	}
	for _, id := range ids {
		var item shared.Monitor
		if err := s.db.Select("id", "interval").First(&item, id).Error; err != nil {
			continue
		}
		interval := time.Duration(item.Interval) * time.Second
		if interval < 5*time.Second {
			interval = 5 * time.Second
		}
		if !s.pool.TryAcquire(id) {
			_ = s.db.Model(&shared.Monitor{}).Where("id = ?", id).Update("next_check_at", now.Add(interval)).Error
			continue
		}
		res := s.db.Model(&shared.Monitor{}).
			Where("id = ? AND (next_check_at IS NULL OR next_check_at <= ?)", id, now).
			Update("next_check_at", now.Add(interval))
		if res.Error != nil || res.RowsAffected == 0 {
			s.pool.Release(id)
			continue
		}
		if !s.pool.Enqueue(id) {
			s.pool.Release(id)
			_ = s.db.Model(&shared.Monitor{}).Where("id = ?", id).Update("next_check_at", now).Error
			slog.Warn("工作队列已满", "monitor", id)
		}
	}
}
