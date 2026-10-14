package dockerctl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	psFormat        = "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
	imageFormat     = "{{.ID}}\t{{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}"
)

var (
	ErrDisabled    = errors.New("Docker 管理已关闭")
	ErrUnavailable = errors.New("当前环境没有 docker，无法管理容器")
	ErrBadName     = errors.New("容器名称不合法")
)

// 容器名和短 ID 都不以横线开头，避免被 docker 当成参数。
var containerName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

// Container 是 docker ps 里的一行。
type Container struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`
	Status string `json:"status"`
	Ports  string `json:"ports"`
}

// Image 是本地镜像。
type Image struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       string `json:"size"`
	Created    string `json:"created"`
}

// ContainerList 是容器列表。没有 docker 时 Available 为 false，页面据此提示。
type ContainerList struct {
	Enabled   bool        `json:"enabled"`
	Available bool        `json:"available"`
	Reason    string      `json:"reason,omitempty"`
	Items     []Container `json:"items"`
}

// ImageList 是镜像列表。
type ImageList struct {
	Enabled   bool    `json:"enabled"`
	Available bool    `json:"available"`
	Reason    string  `json:"reason,omitempty"`
	Items     []Image `json:"items"`
}

// Logs 是 docker logs 返回的文本。
type Logs struct {
	Name  string `json:"name"`
	Lines int    `json:"lines"`
	Text  string `json:"text"`
}

// Runner 执行本机命令。参数按数组传递，不经过 shell。
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// Manager 通过本机 docker 客户端管理容器。
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

// Normalize 只接受容器名或容器 ID，拒绝路径、空白和会被当成参数的值。
func Normalize(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "/")
	if name == "" || strings.Contains(name, "..") || strings.HasPrefix(name, "-") {
		return "", ErrBadName
	}
	if strings.ContainsAny(name, "/ \t\r\n;|&$`<>'\"\\") || !containerName.MatchString(name) {
		return "", ErrBadName
	}
	return name, nil
}

func (m *Manager) Containers(ctx context.Context, query, state string) (ContainerList, error) {
	empty := ContainerList{Enabled: m.enabled, Items: []Container{}}
	if !m.enabled {
		empty.Reason = ErrDisabled.Error()
		return empty, nil
	}
	if _, err := m.lookPath("docker"); err != nil {
		empty.Reason = ErrUnavailable.Error()
		return empty, nil
	}
	out, err := m.run.Run(ctx, "docker", "ps", "-a", "--format", psFormat)
	if err != nil {
		return ContainerList{}, fmt.Errorf("读取容器列表失败: %s", clean(err.Error()))
	}
	items, err := parseContainers(out)
	if err != nil {
		return ContainerList{}, err
	}
	items = filterContainers(items, query, state)
	sortContainers(items)
	return ContainerList{Enabled: true, Available: true, Items: items}, nil
}

func (m *Manager) Images(ctx context.Context, query string) (ImageList, error) {
	empty := ImageList{Enabled: m.enabled, Items: []Image{}}
	if !m.enabled {
		empty.Reason = ErrDisabled.Error()
		return empty, nil
	}
	if _, err := m.lookPath("docker"); err != nil {
		empty.Reason = ErrUnavailable.Error()
		return empty, nil
	}
	out, err := m.run.Run(ctx, "docker", "images", "--format", imageFormat)
	if err != nil {
		return ImageList{}, fmt.Errorf("读取镜像列表失败: %s", clean(err.Error()))
	}
	items, err := parseImages(out)
	if err != nil {
		return ImageList{}, err
	}
	items = filterImages(items, query)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Repository != items[j].Repository {
			return items[i].Repository < items[j].Repository
		}
		return items[i].Tag < items[j].Tag
	})
	return ImageList{Enabled: true, Available: true, Items: items}, nil
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
	target, err := Normalize(name)
	if err != nil {
		return "", err
	}
	if _, err := m.lookPath("docker"); err != nil {
		return "", ErrUnavailable
	}
	if _, err := m.run.Run(ctx, "docker", action, "--", target); err != nil {
		return target, fmt.Errorf("%s %s 失败: %s", actionLabel(action), target, clean(err.Error()))
	}
	return target, nil
}

func (m *Manager) Logs(ctx context.Context, name string, lines int) (Logs, error) {
	if !m.enabled {
		return Logs{}, ErrDisabled
	}
	target, err := Normalize(name)
	if err != nil {
		return Logs{}, err
	}
	lines = clampLines(lines)
	if _, err := m.lookPath("docker"); err != nil {
		return Logs{}, ErrUnavailable
	}
	text, err := m.run.Run(ctx, "docker", "logs", "--tail", strconv.Itoa(lines), "--timestamps", "--", target)
	if err != nil {
		return Logs{}, fmt.Errorf("读取 %s 日志失败: %s", target, clean(err.Error()))
	}
	return Logs{Name: target, Lines: lines, Text: trimLog(text)}, nil
}

func parseContainers(out string) ([]Container, error) {
	items := []Container{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			return nil, errors.New("无法解析容器列表")
		}
		for len(parts) < 5 {
			parts = append(parts, "")
		}
		if len(parts) > 5 {
			parts = []string{parts[0], parts[1], parts[2], parts[3], strings.Join(parts[4:], "\t")}
		}
		name := containerLabel(parts[1])
		if _, err := Normalize(name); err != nil {
			continue
		}
		items = append(items, Container{
			ID:     parts[0],
			Name:   name,
			Image:  parts[2],
			Status: parts[3],
			Ports:  parts[4],
			State:  inferState(parts[3]),
		})
	}
	return items, nil
}

func containerLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if i := strings.IndexByte(raw, ','); i >= 0 {
		raw = raw[:i]
	}
	return strings.TrimPrefix(strings.TrimSpace(raw), "/")
}

func inferState(status string) string {
	text := strings.ToLower(strings.TrimSpace(status))
	switch {
	case strings.Contains(text, "paused"):
		return "paused"
	case strings.HasPrefix(text, "up ") || text == "up":
		return "running"
	case strings.HasPrefix(text, "exited"):
		return "exited"
	case strings.HasPrefix(text, "created"):
		return "created"
	case strings.HasPrefix(text, "restarting"):
		return "restarting"
	case strings.HasPrefix(text, "dead"):
		return "dead"
	case strings.Contains(text, "removal") || strings.Contains(text, "removing"):
		return "removing"
	default:
		return "unknown"
	}
}

func filterContainers(items []Container, query, state string) []Container {
	query = strings.ToLower(strings.TrimSpace(query))
	state = strings.ToLower(strings.TrimSpace(state))
	out := make([]Container, 0, len(items))
	for _, item := range items {
		if query != "" && !strings.Contains(strings.ToLower(item.Name+" "+item.Image+" "+item.ID+" "+item.Ports+" "+item.Status), query) {
			continue
		}
		switch state {
		case "", "all":
		default:
			if item.State != state {
				continue
			}
		}
		out = append(out, item)
	}
	return out
}

func sortContainers(items []Container) {
	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := stateRank(items[i].State), stateRank(items[j].State)
		if ri != rj {
			return ri < rj
		}
		return items[i].Name < items[j].Name
	})
}

func stateRank(state string) int {
	switch state {
	case "restarting":
		return 0
	case "dead", "removing":
		return 1
	case "paused":
		return 2
	case "running":
		return 3
	case "created":
		return 4
	default:
		return 5
	}
}

func parseImages(out string) ([]Image, error) {
	items := []Image{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			return nil, errors.New("无法解析镜像列表")
		}
		for len(parts) < 5 {
			parts = append(parts, "")
		}
		repo := parts[1]
		if repo == "" {
			repo = "<none>"
		}
		tag := parts[2]
		if tag == "" {
			tag = "<none>"
		}
		items = append(items, Image{
			ID: parts[0], Repository: repo, Tag: tag, Size: parts[3], Created: parts[4],
		})
	}
	return items, nil
}

func filterImages(items []Image, query string) []Image {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return items
	}
	out := make([]Image, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Repository+" "+item.Tag+" "+item.ID), query) {
			out = append(out, item)
		}
	}
	return out
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
	if strings.TrimSpace(text) == "" {
		return "没有日志"
	}
	return text
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
	cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
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
	if strings.TrimSpace(out) == "" && stderr.Len() > 0 {
		return stderr.String(), nil
	}
	if stderr.Len() > 0 {
		return out + stderr.String(), nil
	}
	return out, nil
}
