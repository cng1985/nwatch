package alert

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/cng1985/nwatch/internal/checker"
	"github.com/cng1985/nwatch/internal/mailer"
	"github.com/cng1985/nwatch/internal/model"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// Reminder 在故障期间按 15 秒、45 秒、之后每分钟通知一次，直到服务恢复。
type Reminder struct {
	db      *gorm.DB
	alerts  *Manager
	mail    *mailer.Service
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
	scanMu  sync.Mutex
	stateMu sync.Mutex
	running bool
}

func NewReminder(db *gorm.DB, alerts *Manager, mail *mailer.Service, lc fx.Lifecycle) *Reminder {
	r := &Reminder{
		db:     db,
		alerts: alerts,
		mail:   mail,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	if lc != nil {
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				r.Start()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				r.Stop(ctx)
				return nil
			},
		})
	}
	return r
}

func (r *Reminder) Start() {
	if r == nil {
		return
	}
	r.stateMu.Lock()
	if r.running {
		r.stateMu.Unlock()
		return
	}
	r.running = true
	r.stateMu.Unlock()
	go r.loop()
}

func (r *Reminder) Stop(ctx context.Context) {
	if r == nil {
		return
	}
	r.once.Do(func() { close(r.stop) })
	r.stateMu.Lock()
	running := r.running
	r.stateMu.Unlock()
	if !running {
		return
	}
	select {
	case <-r.done:
	case <-ctx.Done():
		slog.Warn("故障提醒未完全停止")
	}
}

func (r *Reminder) loop() {
	defer close(r.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case now := <-ticker.C:
			r.Scan(now)
		}
	}
}

// Scan 找出到点的故障监控，每个时间点最多发一次。
func (r *Reminder) Scan(now time.Time) {
	if r == nil || r.db == nil {
		return
	}
	if !r.scanMu.TryLock() {
		return
	}
	defer r.scanMu.Unlock()
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("故障提醒异常", "panic", rec)
		}
	}()

	var rows []model.Monitor
	err := r.db.Where(
		"enabled = ? AND status = ? AND incident_started_at IS NOT NULL AND (next_notify_at IS NULL OR next_notify_at <= ?)",
		true, model.StatusDown, now,
	).Find(&rows).Error
	if err != nil {
		slog.Error("读取待通知监控失败", "err", err.Error())
		return
	}
	for i := range rows {
		r.fire(&rows[i], now)
	}
}

func (r *Reminder) fire(m *model.Monitor, now time.Time) {
	if m.IncidentStartedAt == nil {
		return
	}
	send, sentStage, newStage, next := Plan(*m.IncidentStartedAt, m.NotifyStage, now)
	if !send {
		if m.NextNotifyAt == nil {
			res := r.db.Model(&model.Monitor{}).
				Where("id = ? AND status = ? AND next_notify_at IS NULL", m.ID, model.StatusDown).
				Updates(map[string]any{"next_notify_at": next, "updated_at": time.Now()})
			if res.Error != nil {
				slog.Error("写入通知计划失败", "monitor", m.Name, "err", res.Error.Error())
			}
		}
		return
	}
	res := r.db.Model(&model.Monitor{}).
		Where("id = ? AND status = ? AND notify_stage = ?", m.ID, model.StatusDown, m.NotifyStage).
		Updates(map[string]any{
			"notify_stage":   newStage,
			"next_notify_at": next,
			"updated_at":     time.Now(),
		})
	if res.Error != nil {
		slog.Error("更新通知计划失败", "monitor", m.Name, "err", res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		return
	}
	var fresh model.Monitor
	if err := r.db.Select("status").First(&fresh, m.ID).Error; err == nil && fresh.Status != model.StatusDown {
		return
	}
	slog.Info("发送故障提醒", "monitor", m.Name, "stage", sentStage, "next", next.Format(time.RFC3339))
	r.dispatch(m, now, sentStage > 0)
}

func (r *Reminder) dispatch(m *model.Monitor, now time.Time, ongoing bool) {
	var duration int64
	if m.IncidentStartedAt != nil {
		duration = int64(now.Sub(*m.IncidentStartedAt).Seconds())
		if duration < 0 {
			duration = 0
		}
	}
	ev := model.AlertEvent{
		MonitorID:    m.ID,
		MonitorName:  m.Name,
		MonitorType:  m.Type,
		EventType:    model.EventDown,
		OldStatus:    model.StatusDown,
		NewStatus:    model.StatusDown,
		Target:       checker.FormatTarget(m),
		Message:      clip(m.LastMessage, 2000),
		ResponseTime: m.LastResponseTime,
		OccurredAt:   now,
		Duration:     duration,
	}
	if m.CertNotAfter != nil {
		expire := *m.CertNotAfter
		ev.CertExpireAt = &expire
	}
	if m.CertDaysRemaining != nil {
		days := *m.CertDaysRemaining
		ev.CertDaysLeft = &days
	}
	if r.alerts != nil {
		r.alerts.Fanout(m.ID, ev)
	}
	if r.mail != nil {
		r.mail.Notify(context.Background(), mailer.Notice{
			Event:    ev,
			Failures: m.ConsecutiveFailures,
			Ongoing:  ongoing,
		})
	}
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
