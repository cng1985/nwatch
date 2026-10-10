package shell

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Session 是一个已经挂上伪终端的交互式 shell。
type Session struct {
	id        string
	user      string
	remote    string
	shellPath string
	pty       *os.File
	cmd       *exec.Cmd
	hub       *Hub
	started   time.Time

	mu        sync.Mutex
	lastInput time.Time
	reason    string
	done      chan struct{}
	once      sync.Once
}

func (s *Session) ID() string { return s.id }

func (s *Session) Shell() string { return s.shellPath }

func (s *Session) Done() <-chan struct{} { return s.done }

func (s *Session) Pid() int {
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Pid
	}
	return 0
}

func (s *Session) Reason() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reason == "" {
		return "exit"
	}
	return s.reason
}

// Touch 把空闲计时重置为现在，只在收到用户输入时调用。
func (s *Session) Touch() {
	s.mu.Lock()
	s.lastInput = time.Now()
	s.mu.Unlock()
}

func (s *Session) Read(p []byte) (int, error) {
	s.mu.Lock()
	f := s.pty
	s.mu.Unlock()
	if f == nil {
		return 0, io.EOF
	}
	return f.Read(p)
}

func (s *Session) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(p) > 256*1024 {
		return 0, io.ErrShortWrite
	}
	s.mu.Lock()
	f := s.pty
	s.mu.Unlock()
	if f == nil {
		return 0, io.EOF
	}
	return f.Write(p)
}

func (s *Session) SetReadDeadline(t time.Time) error {
	s.mu.Lock()
	f := s.pty
	s.mu.Unlock()
	if f == nil {
		return io.EOF
	}
	return f.SetReadDeadline(t)
}

// Resize 修改终端尺寸。超出范围的请求会被忽略。
func (s *Session) Resize(cols, rows int) error {
	if !validSize(cols, rows) {
		return nil
	}
	s.mu.Lock()
	f := s.pty
	s.mu.Unlock()
	if f == nil {
		return io.EOF
	}
	return resizePTY(f, cols, rows)
}

// CloseWith 结束会话。reason 只会保留第一次的值。
func (s *Session) CloseWith(reason string) {
	s.once.Do(func() {
		if reason == "" {
			reason = "exit"
		}
		s.mu.Lock()
		s.reason = reason
		f := s.pty
		pid := 0
		if s.cmd != nil && s.cmd.Process != nil {
			pid = s.cmd.Process.Pid
		}
		userName, remote, id := s.user, s.remote, s.id
		started := s.started
		s.mu.Unlock()
		close(s.done)
		if f != nil {
			_ = f.Close()
		}
		_ = killProcess(pid)
		if s.hub != nil {
			s.hub.forget(id)
		}
		if !started.IsZero() {
			slog.Info("在线终端已断开", "user", userName, "remote", remote, "session", id, "reason", reason, "duration", time.Since(started).Round(time.Millisecond).String())
		}
	})
}

func (s *Session) watchExit() {
	go func() {
		if s.cmd != nil {
			_ = s.cmd.Wait()
		}
		s.CloseWith("exit")
	}()
}

func (s *Session) watchLimits(idle, life time.Duration) {
	go func() {
		interval := time.Second
		shortest := idle
		if life > 0 && (shortest <= 0 || life < shortest) {
			shortest = life
		}
		if shortest > 0 && shortest < interval {
			interval = shortest / 2
			if interval < 20*time.Millisecond {
				interval = 20 * time.Millisecond
			}
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				now := time.Now()
				if life > 0 && !s.started.IsZero() && now.Sub(s.started) >= life {
					s.CloseWith("lifetime")
					return
				}
				s.mu.Lock()
				last := s.lastInput
				s.mu.Unlock()
				if idle > 0 && now.Sub(last) >= idle {
					s.CloseWith("idle")
					return
				}
			}
		}
	}()
}

func startSession(cols, rows int) (*Session, error) {
	path := LookShell()
	if path == "" {
		return nil, ErrNoShell
	}
	cols, rows = NormalizeSize(cols, rows)
	cmd := exec.Command(path)
	cmd.Dir = workingDir()
	cmd.Env = shellEnv()
	f, err := startPTY(cmd, cols, rows)
	if err != nil {
		return nil, err
	}
	return &Session{
		pty:       f,
		cmd:       cmd,
		done:      make(chan struct{}),
		shellPath: path,
		lastInput: time.Now(),
	}, nil
}

// LookShell 优先选择 bash，其次是 sh。
func LookShell() string {
	for _, name := range []string{"bash", "sh"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "/"
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "/"
	}
	return dir
}

func shellEnv() []string {
	src := os.Environ()
	env := make([]string, 0, len(src)+2)
	for _, item := range src {
		if strings.HasPrefix(item, "TERM=") || strings.HasPrefix(item, "NMONITOR_SHELL=") {
			continue
		}
		env = append(env, item)
	}
	return append(env, "TERM=xterm-256color", "NMONITOR_SHELL=1")
}

// NormalizeSize 把初始窗口修正到可用范围。0 或过小会回到 80x24。
func NormalizeSize(cols, rows int) (int, int) {
	if cols < 20 {
		cols = 80
	} else if cols > 500 {
		cols = 500
	}
	if rows < 5 {
		rows = 24
	} else if rows > 200 {
		rows = 200
	}
	return cols, rows
}

func validSize(cols, rows int) bool {
	return cols >= 20 && cols <= 500 && rows >= 5 && rows <= 200
}
