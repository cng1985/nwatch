package cronjob

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/cng1985/nwatch/internal/checker"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/settings"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

const workerSlots = 4

// ErrBusy 表示同一个任务还在执行，这次触发被跳过。
var ErrBusy = errors.New("任务正在执行")

// Service 每秒领取到期的定时任务，并在固定大小的工作池里执行。
type Service struct {
	db       *gorm.DB
	settings *settings.Store
	interval time.Duration
	stop     chan struct{}
	stopOnce sync.Once
	ctx      context.Context
	cancel   context.CancelFunc
	running  sync.Map
	sem      chan struct{}
	wg       sync.WaitGroup
}

func New(db *gorm.DB, store *settings.Store, interval time.Duration, lc fx.Lifecycle) *Service {
	if interval <= 0 {
		interval = time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		db:       db,
		settings: store,
		interval: interval,
		stop:     make(chan struct{}),
		ctx:      ctx,
		cancel:   cancel,
		sem:      make(chan struct{}, workerSlots),
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

func (s *Service) Start() { go s.loop() }

func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)
		s.cancel()
	})
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		slog.Info("定时任务仍在执行，先停止继续调度")
	}
}

func (s *Service) Now() time.Time {
	loc := time.Local
	if s != nil && s.settings != nil {
		loc = s.settings.Location()
	}
	return time.Now().In(loc)
}

func (s *Service) Busy(id uint) bool {
	_, ok := s.running.Load(id)
	return ok
}

func (s *Service) loop() {
	slog.Info("定时任务调度已启动", "interval", s.interval.String())
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	s.scan()
	for {
		select {
		case <-ticker.C:
			s.scan()
		case <-s.stop:
			slog.Info("定时任务调度已停止")
			return
		}
	}
}

func (s *Service) scan() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("定时任务扫描异常", "panic", r)
		}
	}()
	now := s.Now()
	var jobs []model.CronJob
	err := s.db.Where("enabled = ? AND (next_run_at IS NULL OR next_run_at <= ?)", true, now).
		Order("next_run_at").Limit(100).Find(&jobs).Error
	if err != nil {
		slog.Error("查询到期定时任务失败", "err", err.Error())
		return
	}
	for _, job := range jobs {
		if s.Busy(job.ID) {
			s.deferNext(job.ID, now)
			continue
		}
		next, err := Next(job.Cron, now)
		if err != nil {
			slog.Error("定时任务无法计算下次执行", "job", job.ID, "err", err.Error())
			_ = s.db.Model(&model.CronJob{}).Where("id = ?", job.ID).Update("next_run_at", now.Add(time.Hour)).Error
			continue
		}
		res := s.db.Model(&model.CronJob{}).
			Where("id = ? AND enabled = ? AND (next_run_at IS NULL OR next_run_at <= ?)", job.ID, true, now).
			Update("next_run_at", next)
		if res.Error != nil || res.RowsAffected == 0 {
			continue
		}
		s.launch(job.ID)
	}
}

func (s *Service) deferNext(id uint, now time.Time) {
	var job model.CronJob
	if err := s.db.Select("id", "cron", "next_run_at").First(&job, id).Error; err != nil {
		return
	}
	if job.NextRunAt != nil && job.NextRunAt.After(now) {
		return
	}
	next, err := Next(job.Cron, now)
	if err != nil {
		return
	}
	_ = s.db.Model(&model.CronJob{}).
		Where("id = ? AND (next_run_at IS NULL OR next_run_at <= ?)", id, now).
		Update("next_run_at", next).Error
}

func (s *Service) launch(id uint) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("定时任务执行异常", "job", id, "panic", r)
			}
		}()
		select {
		case s.sem <- struct{}{}:
			defer func() { <-s.sem }()
		case <-s.stop:
			return
		}
		if _, err := s.Run(id, model.CronTriggerSchedule); err != nil && !errors.Is(err, ErrBusy) {
			slog.Error("定时任务执行失败", "job", id, "err", err.Error())
		}
	}()
}

// Run 执行一次任务。调度触发和页面上的立即执行共用同一把锁，避免重叠。
func (s *Service) Run(id uint, trigger string) (*model.CronRun, error) {
	var job model.CronJob
	if err := s.db.First(&job, id).Error; err != nil {
		return nil, err
	}
	if _, loaded := s.running.LoadOrStore(id, struct{}{}); loaded {
		return nil, ErrBusy
	}
	defer s.running.Delete(id)

	_ = s.db.Model(&model.CronJob{}).Where("id = ?", id).Update("last_status", model.CronRunning).Error
	timeout := time.Duration(job.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(s.ctx, timeout+time.Second)
	defer cancel()
	out := checker.Execute(ctx, job.Command, job.WorkDir, timeout)
	row := record(&job, out, trigger)
	if err := s.db.Create(&row).Error; err != nil {
		_ = s.finish(&job, out, model.CronFailed, "保存执行记录失败")
		return nil, err
	}
	status := model.CronSuccess
	if out.TimedOut {
		status = model.CronTimeout
	} else if !row.Success {
		status = model.CronFailed
	}
	if err := s.finish(&job, out, status, row.ErrorMessage); err != nil {
		return &row, err
	}
	Prune(s.db)
	return &row, nil
}

func (s *Service) finish(job *model.CronJob, out checker.Output, status, message string) error {
	if status == model.CronSuccess {
		if line := firstLine(out.Stdout); line != "" {
			message = line
		} else {
			message = "执行成功"
		}
	}
	if message == "" {
		message = "执行失败"
	}
	finished := time.Now()
	return s.db.Model(&model.CronJob{}).Where("id = ?", job.ID).Updates(map[string]any{
		"last_run_at":      finished,
		"last_status":      status,
		"last_exit_code":   out.ExitCode,
		"last_message":     clip(message, 2000),
		"last_duration_ms": int(out.Duration.Milliseconds()),
	}).Error
}

func record(job *model.CronJob, out checker.Output, trigger string) model.CronRun {
	ok, judged := out.Judge("", "")
	message := ""
	if !ok {
		message = judged
	}
	if trigger == "" {
		trigger = model.CronTriggerSchedule
	}
	finished := time.Now()
	return model.CronRun{
		JobID:        job.ID,
		Name:         job.Name,
		Cron:         job.Cron,
		Command:      job.Command,
		WorkDir:      job.WorkDir,
		ExitCode:     out.ExitCode,
		Success:      ok,
		Stdout:       out.Stdout,
		Stderr:       out.Stderr,
		ErrorMessage: clip(message, 2000),
		DurationMs:   int(out.Duration.Milliseconds()),
		Trigger:      trigger,
		StartedAt:    finished.Add(-out.Duration),
		FinishedAt:   finished,
	}
}

// SetEnabled 启用时按当前时区重新计算下次执行，避免一打开就补跑已经过去的时间点。
func (s *Service) SetEnabled(id uint, enabled bool) (model.CronJob, error) {
	var job model.CronJob
	if err := s.db.First(&job, id).Error; err != nil {
		return job, err
	}
	updates := map[string]any{"enabled": enabled}
	job.Enabled = enabled
	if enabled {
		next, err := Next(job.Cron, s.Now())
		if err != nil {
			return job, err
		}
		job.NextRunAt = &next
		updates["next_run_at"] = next
	}
	if err := s.db.Model(&model.CronJob{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return job, err
	}
	Decorate(&job)
	return job, nil
}

// RescheduleAll 在时区变化后重算全部任务的下次执行时间。
func (s *Service) RescheduleAll() error {
	var jobs []model.CronJob
	if err := s.db.Find(&jobs).Error; err != nil {
		return err
	}
	now := s.Now()
	for _, job := range jobs {
		next, err := Next(job.Cron, now)
		if err != nil {
			slog.Error("定时任务无法计算下次执行", "job", job.ID, "err", err.Error())
			continue
		}
		if err := s.db.Model(&model.CronJob{}).Where("id = ?", job.ID).Update("next_run_at", next).Error; err != nil {
			return err
		}
	}
	return nil
}
