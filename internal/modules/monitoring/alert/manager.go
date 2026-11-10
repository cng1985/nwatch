package alert

import (
	"context"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"github.com/cng1985/nwatch/internal/modules/notify/notifier"
	"go.uber.org/fx"
	"gorm.io/gorm"
	"log/slog"
	"sync"
	"time"
)

type job struct {
	notifier notifier.Notifier
	event    shared.AlertEvent
}

type Manager struct {
	db       *gorm.DB
	registry *notifier.Registry
	queue    chan job
	wg       sync.WaitGroup
	once     sync.Once
}

func NewManager(db *gorm.DB, registry *notifier.Registry, lc fx.Lifecycle) *Manager {
	m := &Manager{
		db:       db,
		registry: registry,
		queue:    make(chan job, 256),
	}
	if lc != nil {
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				m.Start(2)
				return nil
			},
			OnStop: func(ctx context.Context) error {
				m.Stop(ctx)
				return nil
			},
		})
	}
	return m
}

func (m *Manager) Start(workers int) {
	if workers <= 0 {
		workers = 2
	}
	for i := 0; i < workers; i++ {
		m.wg.Add(1)
		go m.loop()
	}
}

func (m *Manager) Stop(ctx context.Context) {
	m.once.Do(func() { close(m.queue) })
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		slog.Warn("通知队列未完全排空")
	}
}

func (m *Manager) loop() {
	defer m.wg.Done()
	for job := range m.queue {
		m.dispatch(job)
	}
}

func (m *Manager) dispatch(job job) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("通知发送异常", "panic", r, "notifier", job.notifier.Name)
			m.writeLog(job, false, "", "通知发送异常")
		}
	}()
	sender, err := m.registry.Get(job.notifier.Type)
	if err != nil {
		m.writeLog(job, false, "", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	body, err := sender.Send(ctx, &job.notifier, EventView(&job.event))
	if err != nil {
		slog.Warn("通知发送失败", "notifier", job.notifier.Name, "type", job.notifier.Type, "event", job.event.EventType, "err", err.Error())
		m.writeLog(job, false, body, err.Error())
		return
	}
	slog.Info("通知已发送", "notifier", job.notifier.Name, "event", job.event.EventType, "monitor", job.event.MonitorName)
	m.writeLog(job, true, body, "")
}

func (m *Manager) writeLog(job job, ok bool, response, errMsg string) {
	if len(response) > 2000 {
		response = response[:2000]
	}
	if len(errMsg) > 2000 {
		errMsg = errMsg[:2000]
	}
	row := notifier.NotificationLog{
		MonitorID:    job.event.MonitorID,
		NotifierID:   job.notifier.ID,
		NotifierName: job.notifier.Name,
		EventID:      job.event.ID,
		EventType:    job.event.EventType,
		Success:      ok,
		Response:     response,
		ErrorMessage: errMsg,
		SentAt:       time.Now(),
	}
	if err := m.db.Create(&row).Error; err != nil {
		slog.Error("写入通知日志失败", "err", err.Error())
	}
}

// Fanout 把一条告警发给所有已启用的通知渠道。关闭的渠道不发送。
func (m *Manager) Fanout(monitorID uint, event shared.AlertEvent) {
	var notifiers []notifier.Notifier
	if err := m.db.Where("enabled = ?", true).Find(&notifiers).Error; err != nil {
		slog.Error("加载通知渠道失败", "monitor", monitorID, "err", err.Error())
		return
	}
	for _, n := range notifiers {
		m.enqueue(n, event)
	}
}

func (m *Manager) SendTest(ctx context.Context, n *notifier.Notifier) error {
	event := shared.AlertEvent{
		EventType:   "TEST",
		MonitorName: "NMonitor",
		MonitorType: "system",
		NewStatus:   "UP",
		Target:      "nmonitor",
		Message:     "监控系统通知测试成功",
		OccurredAt:  time.Now(),
	}
	sender, err := m.registry.Get(n.Type)
	if err != nil {
		m.writeLog(job{notifier: *n, event: event}, false, "", err.Error())
		return err
	}
	body, err := sender.Send(ctx, n, EventView(&event))
	if err != nil {
		m.writeLog(job{notifier: *n, event: event}, false, body, err.Error())
		return err
	}
	m.writeLog(job{notifier: *n, event: event}, true, body, "")
	return nil
}

func (m *Manager) enqueue(n notifier.Notifier, event shared.AlertEvent) {
	select {
	case m.queue <- job{notifier: n, event: event}:
	default:
		slog.Warn("通知队列已满", "notifier", n.Name, "event", event.EventType)
		m.writeLog(job{notifier: n, event: event}, false, "", "通知队列已满")
	}
}
