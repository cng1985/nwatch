package k8sctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLogLines = 200
	minLogLines     = 20
	maxLogLines     = 500
	maxLogBytes     = 256 * 1024
	maxOutputBytes  = 8 << 20
	maxReplicas     = 64
)

var (
	ErrDisabled    = errors.New("Kubernetes 管理已关闭")
	ErrUnavailable = errors.New("当前环境没有 kubectl，无法管理 Kubernetes")
	ErrBadName     = errors.New("名称不合法")
	ErrBadKind     = errors.New("资源类型不支持")
	ErrBadScale    = errors.New("副本数需要是 0 到 64 的整数")
	ErrBadConfig   = errors.New("Kubernetes 配置不合法")
)

var (
	dnsLabel = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	dnsName  = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
)

type kindSpec struct {
	resource string
	restart  bool
	scale    bool
}

var kinds = map[string]kindSpec{
	"pods":         {resource: "pods"},
	"deployments":  {resource: "deployments", restart: true, scale: true},
	"statefulsets": {resource: "statefulsets", restart: true, scale: true},
	"daemonsets":   {resource: "daemonsets", restart: true},
	"services":     {resource: "services"},
}

// Resource 是一种工作负载或 Service 的摘要。
type Resource struct {
	Kind       string   `json:"kind"`
	Namespace  string   `json:"namespace"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Ready      string   `json:"ready"`
	Restarts   int      `json:"restarts"`
	Node       string   `json:"node,omitempty"`
	Image      string   `json:"image,omitempty"`
	Ports      string   `json:"ports,omitempty"`
	ClusterIP  string   `json:"clusterIP,omitempty"`
	Age        string   `json:"age"`
	Containers []string `json:"containers,omitempty"`
}

// Namespace 是一个命名空间。
type Namespace struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ListResult 是某一类资源的列表。
type ListResult struct {
	Enabled   bool       `json:"enabled"`
	Available bool       `json:"available"`
	Reason    string     `json:"reason,omitempty"`
	Context   string     `json:"context,omitempty"`
	Kind      string     `json:"kind,omitempty"`
	Items     []Resource `json:"items"`
}

// NamespaceList 是命名空间列表。
type NamespaceList struct {
	Enabled   bool        `json:"enabled"`
	Available bool        `json:"available"`
	Reason    string      `json:"reason,omitempty"`
	Context   string      `json:"context,omitempty"`
	Items     []Namespace `json:"items"`
}

// Logs 是 Pod 日志。
type Logs struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Container string `json:"container,omitempty"`
	Lines     int    `json:"lines"`
	Text      string `json:"text"`
}

// Options 是 kubectl 的调用方式。Context 和 Kubeconfig 为空时使用本机默认配置。
type Options struct {
	Enabled    bool
	Context    string
	Kubeconfig string
}

// Runner 执行本机命令。参数按数组传递，不经过 shell。
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// Manager 通过本机 kubectl 查看和操作 Kubernetes。
type Manager struct {
	enabled    bool
	context    string
	kubeconfig string
	configErr  error
	run        Runner
	lookPath   func(string) (string, error)
	now        func() time.Time
}

func New(opt Options) *Manager {
	m := &Manager{
		enabled:    opt.Enabled,
		context:    strings.TrimSpace(opt.Context),
		kubeconfig: strings.TrimSpace(opt.Kubeconfig),
		run:        execRunner{},
		lookPath:   exec.LookPath,
		now:        time.Now,
	}
	m.configErr = validateConfig(m.context, m.kubeconfig)
	return m
}

// NewWithRunner 供测试注入命令执行器。lookPath 视为可用。
func NewWithRunner(opt Options, run Runner) *Manager {
	m := New(opt)
	m.run = run
	m.lookPath = func(name string) (string, error) {
		if strings.TrimSpace(name) == "" {
			return "", errors.New("missing")
		}
		return name, nil
	}
	m.now = func() time.Time { return time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC) }
	return m
}

// NormalizeNamespace 只接受 DNS-1123 label。
func NormalizeNamespace(name string) (string, error) {
	name = strings.TrimSpace(name)
	if len(name) == 0 || len(name) > 63 || !dnsLabel.MatchString(name) {
		return "", ErrBadName
	}
	return name, nil
}

// NormalizeName 只接受 DNS-1123 subdomain，用于 Pod、工作负载和 Service。
func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if len(name) == 0 || len(name) > 253 || strings.HasPrefix(name, "-") || !dnsName.MatchString(name) {
		return "", ErrBadName
	}
	return name, nil
}

// NormalizeContainer 只接受容器名。
func NormalizeContainer(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	if len(name) > 63 || !dnsLabel.MatchString(name) {
		return "", ErrBadName
	}
	return name, nil
}

func (m *Manager) Namespaces(ctx context.Context) (NamespaceList, error) {
	empty := NamespaceList{Enabled: m.enabled, Context: m.context, Items: []Namespace{}}
	ok, reason := m.usable()
	if !ok {
		empty.Reason = reason
		return empty, nil
	}
	args := append(m.base(), "get", "namespaces", "-o", "json", "--request-timeout=8s")
	out, err := m.run.Run(ctx, "kubectl", args...)
	if err != nil {
		return NamespaceList{}, fmt.Errorf("读取命名空间失败: %s", clean(err.Error()))
	}
	items, err := parseNamespaces(out)
	if err != nil {
		return NamespaceList{}, err
	}
	return NamespaceList{Enabled: true, Available: true, Context: m.context, Items: items}, nil
}

func (m *Manager) List(ctx context.Context, kind, namespace, query string) (ListResult, error) {
	kind = strings.TrimSpace(kind)
	spec, known := kinds[kind]
	empty := ListResult{Enabled: m.enabled, Context: m.context, Kind: kind, Items: []Resource{}}
	if !known {
		return ListResult{}, ErrBadKind
	}
	ok, reason := m.usable()
	if !ok {
		empty.Reason = reason
		return empty, nil
	}
	nsArgs, err := namespaceArgs(namespace)
	if err != nil {
		return ListResult{}, err
	}
	args := append(m.base(), "get", spec.resource, "-o", "json", "--request-timeout=8s")
	args = append(args, nsArgs...)
	out, err := m.run.Run(ctx, "kubectl", args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("读取%s失败: %s", kindLabel(kind), clean(err.Error()))
	}
	items, err := parseResources(kind, out, m.now())
	if err != nil {
		return ListResult{}, err
	}
	items = filterResources(items, query)
	sortResources(items)
	return ListResult{Enabled: true, Available: true, Context: m.context, Kind: kind, Items: items}, nil
}

func (m *Manager) Restart(ctx context.Context, kind, namespace, name string) (string, string, error) {
	spec, ns, target, err := m.ref(kind, namespace, name)
	if err != nil {
		return "", "", err
	}
	if !spec.restart {
		return "", "", ErrBadKind
	}
	if err := m.ready(); err != nil {
		return "", "", err
	}
	args := append(m.base(), "rollout", "restart", spec.resource, target, "-n", ns)
	if _, err := m.run.Run(ctx, "kubectl", args...); err != nil {
		return ns, target, fmt.Errorf("重启 %s/%s 失败: %s", ns, target, clean(err.Error()))
	}
	return ns, target, nil
}

func (m *Manager) Scale(ctx context.Context, kind, namespace, name string, replicas int) (string, string, error) {
	if replicas < 0 || replicas > maxReplicas {
		return "", "", ErrBadScale
	}
	spec, ns, target, err := m.ref(kind, namespace, name)
	if err != nil {
		return "", "", err
	}
	if !spec.scale {
		return "", "", ErrBadKind
	}
	if err := m.ready(); err != nil {
		return "", "", err
	}
	args := append(m.base(), "scale", spec.resource, target, "-n", ns, "--replicas="+strconv.Itoa(replicas))
	if _, err := m.run.Run(ctx, "kubectl", args...); err != nil {
		return ns, target, fmt.Errorf("调整 %s/%s 副本失败: %s", ns, target, clean(err.Error()))
	}
	return ns, target, nil
}

func (m *Manager) DeletePod(ctx context.Context, namespace, name string) (string, string, error) {
	_, ns, target, err := m.ref("pods", namespace, name)
	if err != nil {
		return "", "", err
	}
	if err := m.ready(); err != nil {
		return "", "", err
	}
	args := append(m.base(), "delete", "pod", target, "-n", ns, "--wait=false")
	if _, err := m.run.Run(ctx, "kubectl", args...); err != nil {
		return ns, target, fmt.Errorf("删除 %s/%s 失败: %s", ns, target, clean(err.Error()))
	}
	return ns, target, nil
}

func (m *Manager) Logs(ctx context.Context, namespace, name, container string, lines int) (Logs, error) {
	_, ns, target, err := m.ref("pods", namespace, name)
	if err != nil {
		return Logs{}, err
	}
	container, err = NormalizeContainer(container)
	if err != nil {
		return Logs{}, err
	}
	lines = clampLines(lines)
	if err := m.ready(); err != nil {
		return Logs{}, err
	}
	args := append(m.base(), "logs", target, "-n", ns, "--tail="+strconv.Itoa(lines), "--timestamps=true")
	if container != "" {
		args = append(args, "-c", container)
	}
	text, err := m.run.Run(ctx, "kubectl", args...)
	if err != nil {
		return Logs{}, fmt.Errorf("读取 %s/%s 日志失败: %s", ns, target, clean(err.Error()))
	}
	return Logs{Namespace: ns, Name: target, Container: container, Lines: lines, Text: trimLog(text)}, nil
}

func (m *Manager) ref(kind, namespace, name string) (kindSpec, string, string, error) {
	spec, ok := kinds[strings.TrimSpace(kind)]
	if !ok {
		return kindSpec{}, "", "", ErrBadKind
	}
	ns, err := NormalizeNamespace(namespace)
	if err != nil {
		return kindSpec{}, "", "", err
	}
	target, err := NormalizeName(name)
	if err != nil {
		return kindSpec{}, "", "", err
	}
	return spec, ns, target, nil
}

func (m *Manager) usable() (bool, string) {
	if !m.enabled {
		return false, ErrDisabled.Error()
	}
	if m.configErr != nil {
		return false, m.configErr.Error()
	}
	if _, err := m.lookPath("kubectl"); err != nil {
		return false, ErrUnavailable.Error()
	}
	return true, ""
}

func (m *Manager) ready() error {
	if !m.enabled {
		return ErrDisabled
	}
	if m.configErr != nil {
		return m.configErr
	}
	if _, err := m.lookPath("kubectl"); err != nil {
		return ErrUnavailable
	}
	return nil
}

func (m *Manager) base() []string {
	args := make([]string, 0, 4)
	if m.kubeconfig != "" {
		args = append(args, "--kubeconfig", m.kubeconfig)
	}
	if m.context != "" {
		args = append(args, "--context", m.context)
	}
	return args
}

func namespaceArgs(namespace string) ([]string, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" || namespace == "all" || namespace == "*" {
		return []string{"--all-namespaces"}, nil
	}
	ns, err := NormalizeNamespace(namespace)
	if err != nil {
		return nil, err
	}
	return []string{"-n", ns}, nil
}

func validateConfig(contextName, kubeconfig string) error {
	if strings.ContainsAny(contextName, "\r\n\x00") || strings.HasPrefix(contextName, "-") || len(contextName) > 250 {
		return fmt.Errorf("%w：context 不合法", ErrBadConfig)
	}
	if strings.ContainsAny(kubeconfig, "\r\n\x00") || strings.HasPrefix(kubeconfig, "-") || len(kubeconfig) > 4096 {
		return fmt.Errorf("%w：kubeconfig 路径不合法", ErrBadConfig)
	}
	return nil
}

type objectList struct {
	Items []kobject `json:"items"`
}

type kobject struct {
	Metadata struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
	} `json:"metadata"`
	Spec struct {
		Replicas   *int32 `json:"replicas"`
		Type       string `json:"type"`
		ClusterIP  string `json:"clusterIP"`
		NodeName   string `json:"nodeName"`
		Containers []struct {
			Name  string `json:"name"`
			Image string `json:"image"`
		} `json:"containers"`
		Ports []struct {
			Port     int32  `json:"port"`
			Protocol string `json:"protocol"`
		} `json:"ports"`
	} `json:"spec"`
	Status struct {
		Phase                  string `json:"phase"`
		ReadyReplicas          int32  `json:"readyReplicas"`
		DesiredNumberScheduled int32  `json:"desiredNumberScheduled"`
		NumberReady            int32  `json:"numberReady"`
		ContainerStatuses      []struct {
			Ready        bool  `json:"ready"`
			RestartCount int32 `json:"restartCount"`
			State        struct {
				Waiting *struct {
					Reason string `json:"reason"`
				} `json:"waiting"`
				Terminated *struct {
					Reason string `json:"reason"`
				} `json:"terminated"`
			} `json:"state"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

func parseNamespaces(out string) ([]Namespace, error) {
	list, err := decodeList(out)
	if err != nil {
		return nil, err
	}
	items := make([]Namespace, 0, len(list.Items))
	for _, obj := range list.Items {
		name, err := NormalizeNamespace(obj.Metadata.Name)
		if err != nil {
			continue
		}
		status := obj.Status.Phase
		if status == "" {
			status = "Active"
		}
		items = append(items, Namespace{Name: name, Status: status})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func parseResources(kind, out string, now time.Time) ([]Resource, error) {
	list, err := decodeList(out)
	if err != nil {
		return nil, err
	}
	items := make([]Resource, 0, len(list.Items))
	for _, obj := range list.Items {
		item, ok := toResource(kind, obj, now)
		if ok {
			items = append(items, item)
		}
	}
	return items, nil
}

func decodeList(out string) (objectList, error) {
	if len(out) > maxOutputBytes {
		return objectList{}, errors.New("返回内容过大")
	}
	var list objectList
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return objectList{}, errors.New("无法解析 kubectl 返回")
	}
	if list.Items == nil {
		list.Items = []kobject{}
	}
	return list, nil
}

func toResource(kind string, obj kobject, now time.Time) (Resource, bool) {
	name, err := NormalizeName(obj.Metadata.Name)
	if err != nil {
		return Resource{}, false
	}
	namespace := obj.Metadata.Namespace
	if namespace != "" {
		namespace, err = NormalizeNamespace(namespace)
		if err != nil {
			return Resource{}, false
		}
	}
	item := Resource{
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		Age:       ageOf(obj.Metadata.CreationTimestamp, now),
	}
	switch kind {
	case "pods":
		fillPod(&item, obj)
	case "deployments", "statefulsets":
		want := int32(1)
		if obj.Spec.Replicas != nil {
			want = *obj.Spec.Replicas
		}
		item.Ready = fmt.Sprintf("%d/%d", obj.Status.ReadyReplicas, want)
		switch {
		case want == 0:
			item.Status = "Stopped"
		case obj.Status.ReadyReplicas >= want:
			item.Status = "Available"
		default:
			item.Status = "Progressing"
		}
	case "daemonsets":
		item.Ready = fmt.Sprintf("%d/%d", obj.Status.NumberReady, obj.Status.DesiredNumberScheduled)
		switch {
		case obj.Status.DesiredNumberScheduled == 0:
			item.Status = "Stopped"
		case obj.Status.NumberReady >= obj.Status.DesiredNumberScheduled:
			item.Status = "Available"
		default:
			item.Status = "Progressing"
		}
	case "services":
		item.Status = obj.Spec.Type
		if item.Status == "" {
			item.Status = "ClusterIP"
		}
		item.ClusterIP = obj.Spec.ClusterIP
		item.Ports = formatPorts(obj.Spec.Ports)
	}
	return item, true
}

func fillPod(item *Resource, obj kobject) {
	total := len(obj.Spec.Containers)
	ready := 0
	restarts := 0
	names := make([]string, 0, total)
	images := make([]string, 0, total)
	for _, container := range obj.Spec.Containers {
		if container.Name != "" {
			names = append(names, container.Name)
		}
		if container.Image != "" {
			images = append(images, container.Image)
		}
	}
	reason := ""
	for _, status := range obj.Status.ContainerStatuses {
		restarts += int(status.RestartCount)
		if status.Ready {
			ready++
		}
		if reason == "" && status.State.Waiting != nil && status.State.Waiting.Reason != "" {
			reason = status.State.Waiting.Reason
		}
	}
	if total == 0 {
		total = len(obj.Status.ContainerStatuses)
	}
	item.Ready = fmt.Sprintf("%d/%d", ready, total)
	item.Restarts = restarts
	item.Node = obj.Spec.NodeName
	item.Containers = names
	item.Image = strings.Join(images, ", ")
	item.Status = obj.Status.Phase
	if reason != "" && item.Status != "Succeeded" {
		item.Status = reason
	}
	if item.Status == "" {
		item.Status = "Unknown"
	}
}

func formatPorts(ports []struct {
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
}) string {
	parts := make([]string, 0, len(ports))
	for _, port := range ports {
		proto := port.Protocol
		if proto == "" {
			proto = "TCP"
		}
		parts = append(parts, fmt.Sprintf("%d/%s", port.Port, proto))
	}
	return strings.Join(parts, ", ")
}

func filterResources(items []Resource, query string) []Resource {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return items
	}
	out := make([]Resource, 0, len(items))
	for _, item := range items {
		text := strings.ToLower(strings.Join([]string{
			item.Namespace, item.Name, item.Status, item.Image, item.Node, item.Ports, item.ClusterIP, item.Ready,
		}, " "))
		if strings.Contains(text, query) {
			out = append(out, item)
		}
	}
	return out
}

func sortResources(items []Resource) {
	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := statusRank(items[i].Status), statusRank(items[j].Status)
		if ri != rj {
			return ri < rj
		}
		if items[i].Namespace != items[j].Namespace {
			return items[i].Namespace < items[j].Namespace
		}
		return items[i].Name < items[j].Name
	})
}

func statusRank(status string) int {
	switch status {
	case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "Error", "Failed", "OOMKilled", "CreateContainerConfigError", "RunContainerError":
		return 0
	case "Pending", "ContainerCreating", "PodInitializing", "Progressing":
		return 1
	case "Unknown":
		return 2
	default:
		return 3
	}
}

func ageOf(created, now time.Time) string {
	if created.IsZero() {
		return "-"
	}
	d := now.Sub(created)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func kindLabel(kind string) string {
	switch kind {
	case "pods":
		return "Pod"
	case "deployments":
		return "Deployment"
	case "statefulsets":
		return "StatefulSet"
	case "daemonsets":
		return "DaemonSet"
	case "services":
		return "Service"
	default:
		return "资源"
	}
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
	return out, nil
}
