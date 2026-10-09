package host

import (
	"context"
	"log/slog"
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

const (
	sampleEvery = 15 * time.Second
	keepSamples = 7 * 24 * time.Hour
)

// Sampler 定期把本机资源写入数据库，供趋势图使用。
type Sampler struct {
	collector *Collector
	db        *gorm.DB
	stop      chan struct{}
}

func NewSampler(collector *Collector, db *gorm.DB, lc fx.Lifecycle) *Sampler {
	s := &Sampler{collector: collector, db: db, stop: make(chan struct{})}
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

func (s *Sampler) loop() {
	s.tick(true)
	ticker := time.NewTicker(sampleEvery)
	defer ticker.Stop()
	prune := time.NewTicker(time.Hour)
	defer prune.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.tick(false)
		case <-prune.C:
			s.prune()
		}
	}
}

func (s *Sampler) tick(prune bool) {
	snap, err := s.collector.Snapshot()
	if err != nil {
		slog.Warn("采集主机资源失败", "err", err.Error())
		return
	}
	row := SampleFrom(snap)
	if err := s.db.Create(&row).Error; err != nil {
		slog.Warn("保存主机采样失败", "err", err.Error())
		return
	}
	if prune {
		s.prune()
	}
}

func (s *Sampler) prune() {
	cutoff := time.Now().Add(-keepSamples)
	if err := s.db.Where("sampled_at < ?", cutoff).Delete(&model.HostSample{}).Error; err != nil {
		slog.Warn("清理主机采样失败", "err", err.Error())
	}
}
