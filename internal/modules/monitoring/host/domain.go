package host

// HostStatusRequest 是主机资源查询参数。
type HostStatusRequest struct {
	Range string `form:"range"`
}

// HostStatusResponse 是主机资源状态与历史曲线。
type HostStatusResponse struct {
	Current Snapshot `json:"current"`
	History []Point  `json:"history"`
	Range   string   `json:"range"`
}

// ProcessSearch 是进程列表查询参数。Limit 为空串表示默认上限。
type ProcessSearch struct {
	Sort  string `form:"sort"`
	Limit string `form:"limit"`
}

// ProcessListResponse 是进程列表结果，行数信息由引擎结果承载。
type ProcessListResponse = ProcessList
