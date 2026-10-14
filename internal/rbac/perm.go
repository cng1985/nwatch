package rbac

import "strings"

const (
	PermDashboardRead = "dashboard.read"

	PermMonitorRead  = "monitor.read"
	PermMonitorWrite = "monitor.write"
	PermCertRead     = "cert.read"
	PermGroupRead    = "group.read"
	PermGroupWrite   = "group.write"

	PermHostRead     = "host.read"
	PermServiceRead  = "service.read"
	PermServiceWrite = "service.write"
	PermScriptRead   = "script.read"
	PermScriptWrite  = "script.write"
	PermCronRead     = "cron.read"
	PermCronWrite    = "cron.write"
	PermShellUse     = "shell.use"

	PermEventRead     = "event.read"
	PermNotifierRead  = "notifier.read"
	PermNotifierWrite = "notifier.write"

	PermLogRead       = "log.read"
	PermSettingsRead  = "settings.read"
	PermSettingsWrite = "settings.write"
	PermAccessRead    = "access.read"
	PermAccessWrite   = "access.write"
)

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// PermInfo 是一项可分配的权限。
type PermInfo struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// PermGroup 把权限按页面分组，供角色编辑使用。
type PermGroup struct {
	Key   string     `json:"key"`
	Label string     `json:"label"`
	Items []PermInfo `json:"items"`
}

type permDef struct {
	Key        string
	Label      string
	Group      string
	GroupLabel string
}

var catalog = []permDef{
	{PermDashboardRead, "查看总览", "overview", "总览"},

	{PermMonitorRead, "查看监控", "monitor", "监控"},
	{PermMonitorWrite, "管理监控", "monitor", "监控"},
	{PermCertRead, "查看证书", "monitor", "监控"},
	{PermGroupRead, "查看分组", "monitor", "监控"},
	{PermGroupWrite, "管理分组", "monitor", "监控"},

	{PermHostRead, "查看主机资源", "host", "主机"},
	{PermServiceRead, "查看系统服务", "host", "主机"},
	{PermServiceWrite, "操作系统服务", "host", "主机"},
	{PermScriptRead, "查看脚本记录", "host", "主机"},
	{PermScriptWrite, "执行脚本", "host", "主机"},
	{PermCronRead, "查看定时任务", "host", "主机"},
	{PermCronWrite, "管理定时任务", "host", "主机"},
	{PermShellUse, "使用在线终端", "host", "主机"},

	{PermEventRead, "查看告警", "alert", "告警"},
	{PermNotifierRead, "查看通知渠道", "alert", "告警"},
	{PermNotifierWrite, "管理通知渠道", "alert", "告警"},

	{PermLogRead, "查看运行日志", "system", "系统"},
	{PermSettingsRead, "查看系统配置", "system", "系统"},
	{PermSettingsWrite, "修改系统配置", "system", "系统"},
	{PermAccessRead, "查看用户与角色", "system", "系统"},
	{PermAccessWrite, "管理用户与角色", "system", "系统"},
}

var known = func() map[string]struct{} {
	out := make(map[string]struct{}, len(catalog))
	for _, item := range catalog {
		out[item.Key] = struct{}{}
	}
	return out
}()

// Catalog 返回权限分组。
func Catalog() []PermGroup {
	groups := []PermGroup{}
	index := map[string]int{}
	for _, item := range catalog {
		pos, ok := index[item.Group]
		if !ok {
			pos = len(groups)
			index[item.Group] = pos
			groups = append(groups, PermGroup{Key: item.Group, Label: item.GroupLabel})
		}
		groups[pos].Items = append(groups[pos].Items, PermInfo{Key: item.Key, Label: item.Label})
	}
	return groups
}

// All 按目录顺序返回全部权限。
func All() []string {
	out := make([]string, 0, len(catalog))
	for _, item := range catalog {
		out = append(out, item.Key)
	}
	return out
}

func knownPerm(key string) bool {
	_, ok := known[key]
	return ok
}

// Normalize 去掉空项和重复项，并在选择了管理权限时补上对应的查看权限。
func Normalize(keys []string) ([]string, error) {
	want := map[string]struct{}{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if !knownPerm(key) {
			return nil, ErrBadPerm
		}
		want[key] = struct{}{}
		if strings.HasSuffix(key, ".write") {
			read := strings.TrimSuffix(key, ".write") + ".read"
			if knownPerm(read) {
				want[read] = struct{}{}
			}
		}
	}
	if len(want) == 0 {
		return nil, ErrNoPerm
	}
	out := make([]string, 0, len(want))
	for _, key := range All() {
		if _, ok := want[key]; ok {
			out = append(out, key)
		}
	}
	return out, nil
}

func operatorPerms() []string {
	return []string{
		PermDashboardRead,
		PermMonitorRead, PermMonitorWrite,
		PermCertRead,
		PermGroupRead, PermGroupWrite,
		PermHostRead,
		PermServiceRead, PermServiceWrite,
		PermScriptRead, PermScriptWrite,
		PermCronRead, PermCronWrite,
		PermEventRead,
		PermNotifierRead, PermNotifierWrite,
		PermLogRead,
		PermSettingsRead,
	}
}

func viewerPerms() []string {
	return []string{
		PermDashboardRead,
		PermMonitorRead,
		PermCertRead,
		PermGroupRead,
		PermHostRead,
		PermServiceRead,
		PermScriptRead,
		PermCronRead,
		PermEventRead,
		PermNotifierRead,
		PermLogRead,
	}
}
