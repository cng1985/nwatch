package shell

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"os/user"
	"sync"
	"time"
)

const maxPendingTickets = 32

var (
	ErrDisabled       = errors.New("在线终端已关闭")
	ErrNoShell        = errors.New("系统没有可用的 shell")
	ErrBusy           = errors.New("在线终端会话已满")
	ErrTicket         = errors.New("连接凭证无效或已过期")
	ErrTooManyTickets = errors.New("连接凭证过多，请稍后再试")
)

// Options 是终端会话的运行限制。零值会换成默认值，Enabled 除外。
type Options struct {
	Enabled     bool
	IdleTimeout time.Duration
	MaxLifetime time.Duration
	MaxSessions int
	TicketTTL   time.Duration
}

// Info 是管理台展示的终端环境。
type Info struct {
	Enabled        bool   `json:"enabled"`
	Shell          string `json:"shell"`
	User           string `json:"user"`
	Hostname       string `json:"hostname"`
	Cwd            string `json:"cwd"`
	Active         int    `json:"active"`
	MaxSessions    int    `json:"maxSessions"`
	IdleTimeoutSec int    `json:"idleTimeoutSec"`
	MaxLifetimeSec int    `json:"maxLifetimeSec"`
}

type ticket struct {
	user string
	exp  time.Time
}

// Hub 签发一次性连接凭证，并限制同时存在的终端数量。
type Hub struct {
	opts     Options
	mu       sync.Mutex
	closed   bool
	sessions map[string]*Session
	tickets  map[string]ticket
}

func NewHub(opts Options) *Hub {
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = 15 * time.Minute
	}
	if opts.MaxLifetime <= 0 {
		opts.MaxLifetime = 4 * time.Hour
	}
	if opts.MaxSessions <= 0 {
		opts.MaxSessions = 8
	}
	if opts.MaxSessions > 32 {
		opts.MaxSessions = 32
	}
	if opts.TicketTTL <= 0 {
		opts.TicketTTL = 30 * time.Second
	}
	return &Hub{
		opts:     opts,
		sessions: map[string]*Session{},
		tickets:  map[string]ticket{},
	}
}

func (h *Hub) Enabled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.opts.Enabled && !h.closed
}

func (h *Hub) Active() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.sessions)
}

func (h *Hub) Info() Info {
	userName, host, dir, path := Identity()
	h.mu.Lock()
	enabled := h.opts.Enabled && !h.closed
	active := len(h.sessions)
	h.mu.Unlock()
	return Info{
		Enabled:        enabled,
		Shell:          path,
		User:           userName,
		Hostname:       host,
		Cwd:            dir,
		Active:         active,
		MaxSessions:    h.opts.MaxSessions,
		IdleTimeoutSec: int(h.opts.IdleTimeout / time.Second),
		MaxLifetimeSec: int(h.opts.MaxLifetime / time.Second),
	}
}

// Issue 签发 30 秒内、只能使用一次的连接凭证。
func (h *Hub) Issue(username string) (string, error) {
	if LookShell() == "" {
		return "", ErrNoShell
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.opts.Enabled || h.closed {
		return "", ErrDisabled
	}
	h.purgeTicketsLocked(time.Now())
	if len(h.tickets) >= maxPendingTickets {
		return "", ErrTooManyTickets
	}
	h.tickets[token] = ticket{user: username, exp: time.Now().Add(h.opts.TicketTTL)}
	return token, nil
}

// Redeem 核销凭证。同一枚凭证第二次调用会失败。
func (h *Hub) Redeem(token string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.opts.Enabled || h.closed {
		return "", ErrDisabled
	}
	item, ok := h.tickets[token]
	if !ok {
		return "", ErrTicket
	}
	delete(h.tickets, token)
	if time.Now().After(item.exp) {
		return "", ErrTicket
	}
	return item.user, nil
}

// Open 启动一个交互式会话。调用方负责在结束时 Close。
func (h *Hub) Open(username, remote string, cols, rows int) (*Session, error) {
	h.mu.Lock()
	if !h.opts.Enabled || h.closed {
		h.mu.Unlock()
		return nil, ErrDisabled
	}
	if len(h.sessions) >= h.opts.MaxSessions {
		h.mu.Unlock()
		return nil, ErrBusy
	}
	sess, err := startSession(cols, rows)
	if err != nil {
		h.mu.Unlock()
		if errors.Is(err, ErrNoShell) {
			return nil, err
		}
		slog.Error("打开在线终端失败", "err", err.Error(), "user", username)
		return nil, errors.New("无法打开终端")
	}
	id, err := randomToken()
	if err != nil {
		h.mu.Unlock()
		sess.CloseWith("exit")
		if sess.cmd != nil {
			_ = sess.cmd.Wait()
		}
		return nil, err
	}
	sess.id = id[:16]
	sess.user = username
	sess.remote = remote
	sess.hub = h
	sess.started = time.Now()
	sess.lastInput = sess.started
	h.sessions[sess.id] = sess
	idle, life := h.opts.IdleTimeout, h.opts.MaxLifetime
	h.mu.Unlock()

	sess.watchExit()
	sess.watchLimits(idle, life)
	slog.Info("在线终端已连接", "user", username, "remote", remote, "session", sess.id, "shell", sess.shellPath)
	return sess, nil
}

// Close 结束全部会话，之后不能再签发凭证。
func (h *Hub) Close() {
	h.mu.Lock()
	h.closed = true
	list := make([]*Session, 0, len(h.sessions))
	for _, sess := range h.sessions {
		list = append(list, sess)
	}
	h.tickets = map[string]ticket{}
	h.mu.Unlock()
	for _, sess := range list {
		sess.CloseWith("exit")
	}
}

func (h *Hub) forget(id string) {
	h.mu.Lock()
	delete(h.sessions, id)
	h.mu.Unlock()
}

func (h *Hub) purgeTicketsLocked(now time.Time) {
	for id, item := range h.tickets {
		if now.After(item.exp) {
			delete(h.tickets, id)
		}
	}
}

func randomToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// Identity 返回当前进程的用户、主机名、工作目录和 shell 路径。
func Identity() (userName, host, dir, shellPath string) {
	shellPath = LookShell()
	dir = workingDir()
	host, _ = os.Hostname()
	if current, err := user.Current(); err == nil {
		userName = current.Username
	}
	return userName, host, dir, shellPath
}
