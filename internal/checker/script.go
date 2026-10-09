package checker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cng1985/nwatch/internal/model"
	"gorm.io/gorm"
)

const maxScriptOutput = 16 * 1024

type ScriptChecker struct{}

func NewScriptChecker() *ScriptChecker { return &ScriptChecker{} }

func (c *ScriptChecker) Type() string { return model.TypeScript }

func (c *ScriptChecker) Check(ctx context.Context, m *model.Monitor) (*Result, error) {
	timeout := time.Duration(m.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	out := Execute(ctx, m.Command, m.WorkDir, timeout)
	ok, message := out.Judge(m.BodyContains, m.BodyNotContains)
	status := "OK"
	if out.TimedOut {
		status = "TIMEOUT"
	} else if !ok {
		status = "FAILED"
	}
	return &Result{
		Success:      ok,
		Status:       status,
		StatusCode:   out.ExitCode,
		ResponseTime: int(out.Duration.Milliseconds()),
		Message:      message,
		CheckedAt:    time.Now(),
		Metadata: map[string]any{
			"exitCode": out.ExitCode,
			"stdout":   out.Stdout,
			"stderr":   out.Stderr,
			"timedOut": out.TimedOut,
		},
	}, nil
}

// Output 是一次脚本执行的结果。
type Output struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	TimedOut bool
	Err      string
}

// Execute 用系统 shell 执行命令。Linux 上到时会结束整个进程组，Windows 上通过 cmd /C 执行。
func Execute(ctx context.Context, command, workDir string, timeout time.Duration) Output {
	command = strings.TrimSpace(command)
	workDir = strings.TrimSpace(workDir)
	if command == "" {
		return Output{ExitCode: -1, Err: "脚本内容不能为空"}
	}
	if workDir != "" {
		info, err := os.Stat(workDir)
		if err != nil || !info.IsDir() {
			return Output{ExitCode: -1, Err: "工作目录不存在"}
		}
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	start := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := startScript(runCtx, command, workDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitWriter{buf: &stdout, max: maxScriptOutput}
	cmd.Stderr = &limitWriter{buf: &stderr, max: maxScriptOutput}
	err := cmd.Run()
	out := Output{
		Stdout:   cleanOutput(stdout.String()),
		Stderr:   cleanOutput(stderr.String()),
		Duration: time.Since(start),
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		out.TimedOut = true
		out.ExitCode = -1
		return out
	}
	if err == nil {
		out.ExitCode = 0
		return out
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		out.ExitCode = exitErr.ExitCode()
		return out
	}
	out.ExitCode = -1
	out.Err = "无法执行脚本"
	if out.Stderr == "" {
		out.Stderr = cleanOutput(err.Error())
	}
	return out
}

func (o Output) Judge(contains, notContains string) (bool, string) {
	if o.TimedOut {
		return false, "脚本执行超时"
	}
	if o.Err != "" {
		return false, o.Err
	}
	if o.ExitCode != 0 {
		msg := firstLine(o.Stderr)
		if msg == "" {
			msg = firstLine(o.Stdout)
		}
		if msg == "" {
			msg = fmt.Sprintf("退出码 %d", o.ExitCode)
		}
		return false, msg
	}
	if strings.TrimSpace(contains) != "" && !strings.Contains(o.Stdout, contains) {
		return false, "输出未包含指定内容"
	}
	if strings.TrimSpace(notContains) != "" && strings.Contains(o.Stdout, notContains) {
		return false, "输出包含了被禁止的内容"
	}
	if msg := firstLine(o.Stdout); msg != "" {
		return true, msg
	}
	return true, "脚本执行成功"
}

func (o Output) Model(name, command, workDir string, monitorID *uint) model.ScriptRun {
	finished := time.Now()
	message := ""
	ok, judged := o.Judge("", "")
	if !ok {
		message = judged
	}
	if name == "" {
		name = "手动执行"
	}
	return model.ScriptRun{
		MonitorID:    monitorID,
		Name:         clipText(name, 128),
		Command:      command,
		WorkDir:      workDir,
		ExitCode:     o.ExitCode,
		Success:      ok,
		Stdout:       o.Stdout,
		Stderr:       o.Stderr,
		ErrorMessage: clipText(message, 2000),
		DurationMs:   int(o.Duration.Milliseconds()),
		StartedAt:    finished.Add(-o.Duration),
		FinishedAt:   finished,
	}
}

// ScriptRunFromCheck 把定时检测的输出记成一条执行记录。
func ScriptRunFromCheck(m *model.Monitor, result *Result) model.ScriptRun {
	var stdout, stderr string
	exit := -1
	if result.Metadata != nil {
		stdout, _ = result.Metadata["stdout"].(string)
		stderr, _ = result.Metadata["stderr"].(string)
		switch v := result.Metadata["exitCode"].(type) {
		case int:
			exit = v
		case int64:
			exit = int(v)
		case float64:
			exit = int(v)
		}
	}
	message := ""
	if result != nil && !result.Success {
		message = result.Message
	}
	finished := time.Now()
	response := 0
	success := false
	if result != nil {
		success = result.Success
		response = result.ResponseTime
		if !result.CheckedAt.IsZero() {
			finished = result.CheckedAt
		}
	}
	id := m.ID
	return model.ScriptRun{
		MonitorID:    &id,
		Name:         m.Name,
		Command:      m.Command,
		WorkDir:      m.WorkDir,
		ExitCode:     exit,
		Success:      success,
		Stdout:       stdout,
		Stderr:       stderr,
		ErrorMessage: clipText(message, 2000),
		DurationMs:   response,
		StartedAt:    finished.Add(-time.Duration(response) * time.Millisecond),
		FinishedAt:   finished,
	}
}

// PruneScriptRuns 只保留最近 30 天、最多 5000 条执行记录。
func PruneScriptRuns(db *gorm.DB) {
	if db == nil {
		return
	}
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	_ = db.Where("started_at < ?", cutoff).Delete(&model.ScriptRun{}).Error
	var keep model.ScriptRun
	err := db.Model(&model.ScriptRun{}).Order("id desc").Offset(4999).Limit(1).Take(&keep).Error
	if err != nil || keep.ID == 0 {
		return
	}
	_ = db.Where("id < ?", keep.ID).Delete(&model.ScriptRun{}).Error
}

type limitWriter struct {
	buf *bytes.Buffer
	max int
	n   int
}

func (w *limitWriter) Write(p []byte) (int, error) {
	remain := w.max - w.n
	if remain > 0 {
		chunk := p
		if len(chunk) > remain {
			chunk = chunk[:remain]
		}
		_, _ = w.buf.Write(chunk)
		w.n += len(chunk)
	}
	return len(p), nil
}

func cleanOutput(s string) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	s = strings.Map(func(r rune) rune {
		if r == 0 {
			return -1
		}
		return r
	}, s)
	return clipText(s, maxScriptOutput)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return clipText(strings.TrimSpace(s), 180)
}

func clipText(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if len(s) <= max && utf8.ValidString(s) {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max && len(s) <= max*4 {
		if len(s) <= max {
			return s
		}
	}
	if len(runes) > max {
		runes = runes[:max]
	}
	out := string(runes)
	for len(out) > max && len(runes) > 0 {
		runes = runes[:len(runes)-1]
		out = string(runes)
	}
	return out
}

// ValidateScript 检查手动执行的参数。保存监控时不要求工作目录已经存在。
func ValidateScript(command, workDir string, timeoutSec int, statDir bool) (time.Duration, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return 0, errors.New("脚本内容不能为空")
	}
	if len(command) > 8192 {
		return 0, errors.New("脚本不能超过 8KB")
	}
	if strings.ContainsRune(command, 0) {
		return 0, errors.New("脚本内容不合法")
	}
	workDir = strings.TrimSpace(workDir)
	if workDir != "" {
		if !filepath.IsAbs(workDir) {
			return 0, errors.New("工作目录需要是绝对路径")
		}
		if statDir {
			info, err := os.Stat(workDir)
			if err != nil || !info.IsDir() {
				return 0, errors.New("工作目录不存在")
			}
		}
	}
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	if timeoutSec < 1 || timeoutSec > 120 {
		return 0, errors.New("超时时间需在 1 到 120 秒之间")
	}
	return time.Duration(timeoutSec) * time.Second, nil
}
