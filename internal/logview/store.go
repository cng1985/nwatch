package logview

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/cng1985/nwatch/internal/model"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

// Bound 表示日志已经可以写入数据库。
type Bound struct{}

// Store 把进程日志同时写到标准输出和数据库。
type Store struct {
	mu     sync.Mutex
	db     *gorm.DB
	recent []model.AppLog
	stop   chan struct{}
	once   sync.Once
}

func NewStore() *Store {
	return &Store{stop: make(chan struct{})}
}

func BindDB(store *Store, db *gorm.DB, lc fx.Lifecycle) Bound {
	store.Bind(db)
	if lc != nil {
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				go store.loop()
				return nil
			},
			OnStop: func(context.Context) error {
				store.once.Do(func() { close(store.stop) })
				return nil
			},
		})
	}
	return Bound{}
}

func (s *Store) Bind(db *gorm.DB) {
	s.mu.Lock()
	pending := append([]model.AppLog(nil), s.recent...)
	s.db = db
	s.mu.Unlock()
	for i := range pending {
		pending[i].ID = 0
		_ = db.Create(&pending[i]).Error
	}
}

func (s *Store) Handler(level slog.Level) slog.Handler {
	inner := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return &teeHandler{inner: inner, store: s}
}

func (s *Store) Add(level, message, attrs string) {
	row := model.AppLog{
		Level:    level,
		Message:  clipLog(message, 4000),
		Attrs:    clipLog(attrs, 2000),
		LoggedAt: time.Now(),
	}
	s.mu.Lock()
	s.recent = append(s.recent, row)
	if len(s.recent) > 500 {
		s.recent = s.recent[len(s.recent)-500:]
	}
	db := s.db
	s.mu.Unlock()
	if db != nil {
		_ = db.Create(&row).Error
	}
}

func (s *Store) loop() {
	s.prune()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.prune()
		}
	}
}

func (s *Store) prune() {
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	if db == nil {
		return
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	_ = db.Where("logged_at < ?", cutoff).Delete(&model.AppLog{}).Error
}

type teeHandler struct {
	inner slog.Handler
	store *Store
}

func (h *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *teeHandler) Handle(ctx context.Context, r slog.Record) error {
	attrs := formatAttrs(r)
	err := h.inner.Handle(ctx, r)
	h.store.Add(levelName(r.Level), r.Message, attrs)
	return err
}

func (h *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &teeHandler{inner: h.inner.WithAttrs(attrs), store: h.store}
}

func (h *teeHandler) WithGroup(name string) slog.Handler {
	return &teeHandler{inner: h.inner.WithGroup(name), store: h.store}
}

func levelName(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERROR"
	case level >= slog.LevelWarn:
		return "WARN"
	case level >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}

func formatAttrs(r slog.Record) string {
	var b strings.Builder
	r.Attrs(func(a slog.Attr) bool {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(a.Value.String())
		return b.Len() < 2000
	})
	return b.String()
}

func clipLog(s string, max int) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	if len(s) <= max {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && len(string(runes)) > max {
		runes = runes[:len(runes)-1]
	}
	if !utf8.ValidString(string(runes)) {
		return ""
	}
	return string(runes)
}
