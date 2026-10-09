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
	slog.Info("故障提醒已启动")
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
	r.Scan(time.Now())
	for {
		select {
		case <-r.stop:
			return
		case now := <-ticker.C:
			r.Scan(now)
		}
	}
}

// Scan 找出仍在故障中的监控。是否到点在程序里判断，避免数据库里的时间字符串对不上。
// 每个时间点最多发一次。升级前就已经异常、还没有通知计划的监控也会补上。
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
	err := r.db.Where("enabled = ? AND status = ?", true, model.StatusDown).Find(&rows).Error
	if err != nil {
		slog.Error("读取待通知监控失败", "err", err.Error())
		return
	}
	for i := range rows {
		r.fire(&rows[i], now)
	}
}

func (r *Reminder) fire(m *model.Monitor, now time.Time) {
	if !r.ensureIncident(m, now) {
		return
	}
	send, sentStage, newStage, next := Plan(*m.IncidentStartedAt, m.NotifyStage, now)
	if !send {
		r.rememberNext(m, next)
		return
	}
	// 旧数据的 notify_stage 是 NULL。GORM 读出来是 0，但 SQL 里 NULL = 0 不成立，认领会一直失败。
	res := r.db.Model(&model.Monitor{}).
		Where("id = ? AND status = ? AND COALESCE(notify_stage, 0) = ?", m.ID, model.StatusDown, m.NotifyStage).
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

// ensureIncident 给升级前就已异常、没有故障开始时间的监控补上时间。
// 用最近一次失败时间，这样已经超过 15 秒的会立刻补发，而不是再等一轮。
func (r *Reminder) ensureIncident(m *model.Monitor, now time.Time) bool {
	if m.IncidentStartedAt != nil {
		return true
	}
	start := now
	if m.LastFailureAt != nil && !m.LastFailureAt.After(now) {
		start = *m.LastFailureAt
	}
	res := r.db.Model(&model.Monitor{}).
		Where("id = ? AND status = ? AND incident_started_at IS NULL", m.ID, model.StatusDown).
		Updates(map[string]any{"incident_started_at": start, "updated_at": time.Now()})
	if res.Error != nil {
		slog.Error("补记故障开始时间失败", "monitor", m.Name, "err", res.Error.Error())
		return false
	}
	if res.RowsAffected == 0 {
		return false
	}
	m.IncidentStartedAt = &start
	slog.Info("补记故障开始时间", "monitor", m.Name, "start", start.Format(time.RFC3339))
	return true
}

func (r *Reminder) rememberNext(m *model.Monitor, next time.Time) {
	if m.NextNotifyAt != nil {
		return
	}
	res := r.db.Model(&model.Monitor{}).
		Where("id = ? AND status = ? AND next_notify_at IS NULL", m.ID, model.StatusDown).
		Updates(map[string]any{"next_notify_at": next, "updated_at": time.Now()})
	if res.Error != nil {
		slog.Error("写入通知计划失败", "monitor", m.Name, "err", res.Error.Error())
	}
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
