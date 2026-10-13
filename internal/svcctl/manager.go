package svcctl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultLogLines = 200
	minLogLines     = 20
	maxLogLines     = 500
	maxLogBytes     = 256 * 1024
)

var (
	ErrDisabled    = errors.New("系统服务管理已关闭")
	ErrUnavailable = errors.New("当前环境没有 systemctl，无法管理系统服务")
	ErrNoJournal   = errors.New("当前环境没有 journalctl，无法查看服务日志")
	ErrBadName     = errors.New("服务名称不合法")
	ErrProtected   = errors.New("不能操作系统核心服务")
)

var (
	unitName  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:_.\\@-]{0,220}\.service$`)
	otherUnit = regexp.MustCompile(`(?i)\.(socket|target|mount|timer|path|slice|scope|swap|device|automount|snapshot)$`)
)

var protectedUnits = map[string]struct{}{
	"systemd.service": {},
	"init.service":    {},
}

// Unit 是一个 systemd service 单元。
type Unit struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	UnitFile    string `json:"unitFile"`
}

// ListResult 是服务列表。没有 systemd 时 Available 为 false，页面据此提示，而不是报错。
type ListResult struct {
	Enabled   bool   `json:"enabled"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	Items     []Unit `json:"items"`
}

// Logs 是 journalctl 返回的文本。
type Logs struct {
	Name  string `json:"name"`
	Lines int    `json:"lines"`
	Text  string `json:"text"`
}

// Runner 执行本机命令。参数按数组传递，不经过 shell。
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// Manager 通过 systemctl 和 journalctl 管理系统服务。
type Manager struct {
	enabled  bool
	run      Runner
	lookPath func(string) (string, error)
}

func New(enabled bool) *Manager {
	return &Manager{enabled: enabled, run: execRunner{}, lookPath: exec.LookPath}
}

// NewWithRunner 供测试注入命令执行器。lookPath 视为可用。
func NewWithRunner(enabled bool, run Runner) *Manager {
	return &Manager{
		enabled: enabled,
		run:     run,
		lookPath: func(name string) (string, error) {
			if strings.TrimSpace(name) == "" {
				return "", errors.New("missing")
			}
			return name, nil
		},
	}
}

func (m *Manager) Enabled() bool { return m.enabled }

// Normalize 只接受 service 单元名，并拒绝路径、空白和 shell 元字符。
func Normalize(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, "/ \t\r\n;|&$`<>'\"") {
		return "", ErrBadName
	}
	if otherUnit.MatchString(name) {
		return "", ErrBadName
	}
	if !strings.HasSuffix(strings.ToLower(name), ".service") {
		name += ".service"
	}
	if strings.Contains(name, "@.") || !unitName.MatchString(name) {
		return "", ErrBadName
	}
	if _, blocked := protectedUnits[strings.ToLower(name)]; blocked {
		return "", ErrProtected
	}
	return name, nil
}

func (m *Manager) List(ctx context.Context, query, state string) (ListResult, error) {
	empty := ListResult{Enabled: m.enabled, Items: []Unit{}}
	if !m.enabled {
		empty.Reason = ErrDisabled.Error()
		return empty, nil
	}
	if _, err := m.lookPath("systemctl"); err != nil {
		empty.Reason = ErrUnavailable.Error()
		return empty, nil
	}
	unitsOut, unitsErr := m.run.Run(ctx, "systemctl", "list-units", "--type=service", "--all", "--no-legend", "--no-pager")
	filesOut, filesErr := m.run.Run(ctx, "systemctl", "list-unit-files", "--type=service", "--no-legend", "--no-pager")
	if unitsErr != nil && filesErr != nil {
		return ListResult{}, fmt.Errorf("读取服务列表失败: %s", clean(unitsErr.Error()))
	}
	var reason string
	units := map[string]Unit{}
	files := map[string]string{}
	if unitsErr != nil {
		reason = "无法读取运行状态，仅显示已安装的服务：" + clean(unitsErr.Error())
	} else {
		units = parseUnits(unitsOut)
	}
	if filesErr != nil {
		if reason != "" {
			reason += " "
		}
		reason += "无法读取开机启动状态：" + clean(filesErr.Error())
	} else {
		files = parseUnitFiles(filesOut)
	}
	items := mergeUnits(units, files)
	fillDescriptions(items, defaultUnitDirs)
	items = filterUnits(items, query, state)
	sortUnits(items)
	return ListResult{Enabled: true, Available: true, Reason: reason, Items: items}, nil
}

func (m *Manager) Action(ctx context.Context, name, action string) (string, error) {
	if !m.enabled {
		return "", ErrDisabled
	}
	switch action {
	case "start", "stop", "restart":
	default:
		return "", ErrBadName
	}
	unit, err := Normalize(name)
	if err != nil {
		return "", err
	}
	if _, err := m.lookPath("systemctl"); err != nil {
		return "", ErrUnavailable
	}
	if _, err := m.run.Run(ctx, "systemctl", action, "--no-pager", unit); err != nil {
		return unit, fmt.Errorf("%s %s 失败: %s", actionLabel(action), unit, clean(err.Error()))
	}
	return unit, nil
}

func (m *Manager) Logs(ctx context.Context, name string, lines int) (Logs, error) {
	if !m.enabled {
		return Logs{}, ErrDisabled
	}
	unit, err := Normalize(name)
	if err != nil {
		return Logs{}, err
	}
	lines = clampLines(lines)
	if _, err := m.lookPath("journalctl"); err != nil {
		return Logs{}, ErrNoJournal
	}
	text, err := m.run.Run(ctx, "journalctl", "-u", unit, "-n", strconv.Itoa(lines), "--no-pager", "-o", "short-iso")
	if err != nil {
		return Logs{}, fmt.Errorf("读取 %s 日志失败: %s", unit, clean(err.Error()))
	}
	return Logs{Name: unit, Lines: lines, Text: trimLog(text)}, nil
}

func clampLines(lines int) int {
	if lines <= 0 {
		return defaultLogLines
	}
	if lines < minLogLines {
		return minLogLines
	}
	if lines > maxLogLines {
		return maxLogLines
	}
	return lines
}

func trimLog(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if len(text) > maxLogBytes {
		text = text[len(text)-maxLogBytes:]
		if i := strings.IndexByte(text, '\n'); i >= 0 && i < len(text)-1 {
			text = text[i+1:]
		}
	}
	if strings.TrimSpace(text) == "" || strings.TrimSpace(text) == "-- No entries --" {
		return "没有日志"
	}
	return text
}

func parseUnits(out string) map[string]Unit {
	items := map[string]Unit{}
	for _, line := range strings.Split(out, "\n") {
		name, load, active, sub, desc, ok := splitUnitLine(line)
		if !ok || load == "not-found" {
			continue
		}
		if _, err := Normalize(name); err != nil {
			continue
		}
		items[name] = Unit{
			Name: name, Description: desc, Load: load, Active: active, Sub: sub,
		}
	}
	return items
}

func splitUnitLine(line string) (name, load, active, sub, desc string, ok bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 || fields[0] == "UNIT" {
		return
	}
	if fields[0] == "●" || fields[0] == "*" || fields[0] == "○" {
		fields = fields[1:]
	}
	if len(fields) < 4 {
		return
	}
	name = strings.TrimPrefix(strings.TrimPrefix(fields[0], "●"), "*")
	if !strings.HasSuffix(name, ".service") {
		return
	}
	load, active, sub = fields[1], fields[2], fields[3]
	desc = strings.Join(fields[4:], " ")
	ok = true
	return
}

func parseUnitFiles(out string) map[string]string {
	files := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 || fields[0] == "UNIT" {
			continue
		}
		name := fields[0]
		if !strings.HasSuffix(name, ".service") {
			continue
		}
		if _, err := Normalize(name); err != nil {
			continue
		}
		files[name] = fields[1]
	}
	return files
}

func mergeUnits(units map[string]Unit, files map[string]string) []Unit {
	out := make([]Unit, 0, len(units)+len(files))
	seen := map[string]struct{}{}
	for name, unit := range units {
		unit.UnitFile = files[name]
		out = append(out, unit)
		seen[name] = struct{}{}
	}
	for name, state := range files {
		if _, ok := seen[name]; ok {
			continue
		}
		out = append(out, Unit{
			Name: name, Load: "loaded", Active: "inactive", Sub: "dead", UnitFile: state,
		})
	}
	return out
}

func filterUnits(items []Unit, query, state string) []Unit {
	query = strings.ToLower(strings.TrimSpace(query))
	state = strings.ToLower(strings.TrimSpace(state))
	out := make([]Unit, 0, len(items))
	for _, item := range items {
		if query != "" && !strings.Contains(strings.ToLower(item.Name+" "+item.Description), query) {
			continue
		}
		switch state {
		case "", "all":
		case "active":
			if item.Active != "active" && item.Active != "activating" && item.Active != "reloading" {
				continue
			}
		case "inactive":
			if item.Active != "inactive" && item.Active != "deactivating" {
				continue
			}
		case "failed":
			if item.Active != "failed" {
				continue
			}
		}
		out = append(out, item)
	}
	return out
}

var defaultUnitDirs = []string{
	"/etc/systemd/system",
	"/run/systemd/system",
	"/usr/lib/systemd/system",
	"/lib/systemd/system",
}

func fillDescriptions(items []Unit, dirs []string) {
	for i := range items {
		if strings.TrimSpace(items[i].Description) != "" {
			continue
		}
		items[i].Description = unitDescription(dirs, items[i].Name)
	}
}

func unitDescription(dirs []string, name string) string {
	if _, err := Normalize(name); err != nil {
		return ""
	}
	for _, dir := range dirs {
		base := filepath.Clean(dir)
		path := filepath.Join(base, name)
		if !strings.HasPrefix(path, base+string(os.PathSeparator)) {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if desc := descriptionFrom(raw); desc != "" {
			return desc
		}
	}
	return ""
}

func descriptionFrom(raw []byte) string {
	inUnit := false
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inUnit = strings.EqualFold(line, "[Unit]")
			continue
		}
		if !inUnit || line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "Description") {
			return strings.Trim(strings.TrimSpace(val), `"`)
		}
	}
	return ""
}

func sortUnits(items []Unit) {
	sort.Slice(items, func(i, j int) bool {
		ri, rj := activeRank(items[i].Active), activeRank(items[j].Active)
		if ri != rj {
			return ri < rj
		}
		return items[i].Name < items[j].Name
	})
}

func activeRank(active string) int {
	switch active {
	case "failed":
		return 0
	case "activating", "deactivating", "reloading":
		return 1
	case "active":
		return 2
	default:
		return 3
	}
}

func actionLabel(action string) string {
	switch action {
	case "start":
		return "启动"
	case "stop":
		return "停止"
	case "restart":
		return "重启"
	default:
		return "操作"
	}
}

func clean(msg string) string {
	msg = strings.TrimSpace(strings.ReplaceAll(msg, "\r", ""))
	if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
		msg = strings.TrimSpace(msg[:idx])
	}
	if len(msg) > 300 {
		msg = msg[:300]
	}
	return msg
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C", "SYSTEMD_COLORS=0", "SYSTEMD_PAGER=cat")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(out)
		}
		if msg == "" {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				msg = "操作超时"
			} else {
				msg = err.Error()
			}
		}
		return out, errors.New(clean(msg))
	}
	return out, nil
}
