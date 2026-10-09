package mailer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/settings"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

type Service struct {
	db             *gorm.DB
	settings       *settings.Store
	mu             sync.Mutex
	queue          map[string]*queued
	wake           chan struct{}
	stop           chan struct{}
	done           chan struct{}
	once           sync.Once
	running        bool
	initialBackoff time.Duration
	maxBackoff     time.Duration
}

type queued struct {
	notice   Notice
	gen      int
	failures int
	next     time.Time
}

func New(db *gorm.DB, store *settings.Store, lc fx.Lifecycle) *Service {
	s := &Service{
		db:             db,
		settings:       store,
		queue:          map[string]*queued{},
		wake:           make(chan struct{}, 1),
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
		initialBackoff: 2 * time.Second,
		maxBackoff:     5 * time.Minute,
	}
	if lc != nil {
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				s.Start()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				s.Stop(ctx)
				return nil
			},
		})
	}
	return s
}

func (s *Service) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	go s.loop()
}

func (s *Service) Stop(ctx context.Context) {
	s.once.Do(func() { close(s.stop) })
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	if !running {
		return
	}
	select {
	case <-s.done:
	case <-ctx.Done():
		slog.Warn("邮件重试队列未完全停止")
	}
}

// Notify 发送一封告警邮件。发送失败时会一直重试，直到成功或服务恢复。
func (s *Service) Notify(ctx context.Context, n Notice) {
	if s == nil {
		return
	}
	cfg, ok := s.current()
	if !ok {
		if isRecovery(n.Event.EventType) {
			s.drop(failKey(n))
		}
		return
	}
	if isRecovery(n.Event.EventType) {
		s.drop(failKey(n))
	}
	var err error
	for i := 0; i < 2; i++ {
		err = s.deliver(ctx, cfg, n)
		if err == nil {
			s.ack(n)
			s.writeLog(n, true, "")
			slog.Info("邮件已发送", "monitor", n.Event.MonitorName, "event", n.Event.EventType, "ongoing", n.Ongoing)
			return
		}
		if i == 0 {
			timer := time.NewTimer(300 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				s.enqueue(n, 1)
				s.writeLog(n, false, err.Error())
				slog.Warn("邮件发送失败，将持续重试", "monitor", n.Event.MonitorName, "err", err.Error())
				return
			case <-timer.C:
			}
		}
	}
	s.enqueue(n, 2)
	s.writeLog(n, false, err.Error())
	slog.Warn("邮件发送失败，将持续重试", "monitor", n.Event.MonitorName, "event", n.Event.EventType, "err", err.Error())
}

func (s *Service) SendTest(ctx context.Context, cfg Config) error {
	cfg = Normalize(cfg)
	if err := cfg.Validate(); err != nil {
		return err
	}
	n := Notice{Event: model.AlertEvent{
		EventType:   "TEST",
		MonitorName: "NMonitor",
		MonitorType: "system",
		NewStatus:   "UP",
		Target:      cfg.Host,
		Message:     "监控系统邮件通知测试",
		OccurredAt:  time.Now(),
	}}
	if err := s.deliver(ctx, cfg, n); err != nil {
		s.writeLog(n, false, err.Error())
		return err
	}
	s.writeLog(n, true, "")
	return nil
}

func (s *Service) deliver(ctx context.Context, cfg Config, n Notice) error {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	subject, body := Format(n, s.settings.Location())
	return Send(ctx, cfg, subject, body)
}

func (s *Service) current() (Config, bool) {
	raw := s.settings.SMTP()
	cfg := Normalize(Config{
		Enabled:    raw.Enabled,
		Host:       raw.Host,
		Port:       raw.Port,
		Username:   raw.Username,
		Password:   raw.Password,
		From:       raw.From,
		To:         raw.To,
		Encryption: raw.Encryption,
	})
	if !cfg.Enabled || cfg.Validate() != nil {
		return cfg, false
	}
	return cfg, true
}

func (s *Service) loop() {
	defer close(s.done)
	interval := time.Second
	if s.initialBackoff > 0 && s.initialBackoff < interval {
		interval = s.initialBackoff
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.drain()
		case <-s.wake:
			s.drain()
		}
	}
}

type job struct {
	key    string
	gen    int
	notice Notice
}

func (s *Service) drain() {
	s.mu.Lock()
	now := time.Now()
	var jobs []job
	for key, item := range s.queue {
		if item.next.After(now) {
			continue
		}
		item.next = now.Add(time.Minute)
		jobs = append(jobs, job{key: key, gen: item.gen, notice: item.notice})
	}
	s.mu.Unlock()
	for _, item := range jobs {
		cfg, ok := s.current()
		if !ok {
			s.drop(item.key)
			continue
		}
		err := s.deliver(context.Background(), cfg, item.notice)
		s.mu.Lock()
		queued, exists := s.queue[item.key]
		if !exists || queued.gen != item.gen {
			s.mu.Unlock()
			if err == nil {
				s.writeLog(item.notice, true, "")
			}
			continue
		}
		if err == nil {
			delete(s.queue, item.key)
			s.mu.Unlock()
			s.writeLog(item.notice, true, "")
			slog.Info("邮件已发送", "monitor", item.notice.Event.MonitorName, "event", item.notice.Event.EventType, "retry", true)
			continue
		}
		queued.failures++
		queued.next = time.Now().Add(s.backoff(queued.failures))
		s.mu.Unlock()
		s.writeLog(item.notice, false, err.Error())
		slog.Warn("邮件发送失败，将继续重试", "monitor", item.notice.Event.MonitorName, "err", err.Error())
	}
}

func (s *Service) enqueue(n Notice, attempts int) {
	s.mu.Lock()
	if isRecovery(n.Event.EventType) {
		s.dropLocked(failKey(n))
	}
	key := noticeKey(n)
	if old, ok := s.queue[key]; ok {
		old.notice = n
		old.gen++
		if attempts > old.failures {
			old.failures = attempts
		}
		sooner := time.Now().Add(s.backoff(old.failures))
		if old.next.After(sooner) {
			old.next = sooner
		}
	} else {
		s.queue[key] = &queued{
			notice:   n,
			gen:      1,
			failures: attempts,
			next:     time.Now().Add(s.backoff(attempts)),
		}
	}
	s.mu.Unlock()
	s.kick()
}

func (s *Service) ack(n Notice) {
	s.mu.Lock()
	s.dropLocked(noticeKey(n))
	if isRecovery(n.Event.EventType) {
		s.dropLocked(failKey(n))
	}
	s.mu.Unlock()
}

func (s *Service) drop(key string) {
	s.mu.Lock()
	s.dropLocked(key)
	s.mu.Unlock()
}

func (s *Service) dropLocked(key string) {
	if item, ok := s.queue[key]; ok {
		item.gen++
		delete(s.queue, key)
	}
}

func (s *Service) kick() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) backoff(failures int) time.Duration {
	base := s.initialBackoff
	if base <= 0 {
		base = 2 * time.Second
	}
	max := s.maxBackoff
	if max <= 0 {
		max = 5 * time.Minute
	}
	if failures < 1 {
		failures = 1
	}
	d := base
	for i := 1; i < failures; i++ {
		if d >= max/2 {
			return max
		}
		d *= 2
	}
	if d > max {
		return max
	}
	return d
}

func noticeKey(n Notice) string {
	kind := "fail"
	if isRecovery(n.Event.EventType) {
		kind = "recover"
	} else if !isFailure(n.Event.EventType) && n.Event.EventType != "" && n.Event.EventType != model.EventDown {
		kind = n.Event.EventType
	}
	return itoa(n.Event.MonitorID) + ":" + kind
}

func failKey(n Notice) string {
	return itoa(n.Event.MonitorID) + ":fail"
}

func itoa(id uint) string {
	if id == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = byte('0' + id%10)
		id /= 10
	}
	return string(buf[i:])
}

func (s *Service) writeLog(n Notice, ok bool, errMsg string) {
	if s.db == nil {
		return
	}
	if len(errMsg) > 2000 {
		errMsg = errMsg[:2000]
	}
	row := model.NotificationLog{
		MonitorID:    n.Event.MonitorID,
		NotifierName: "邮件",
		EventID:      n.Event.ID,
		EventType:    n.Event.EventType,
		Success:      ok,
		ErrorMessage: errMsg,
		SentAt:       time.Now(),
	}
	if ok {
		row.Response = "已发送"
	}
	if err := s.db.Create(&row).Error; err != nil {
		slog.Error("写入邮件通知日志失败", "err", err.Error())
	}
}
