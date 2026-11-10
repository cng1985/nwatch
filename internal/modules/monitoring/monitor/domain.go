package monitor

import (
	"github.com/cng1985/nwatch/internal/modules/monitoring/checker"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
)

// 说明：监控分组 shared 实体是分组内已发布的响应契约，被引擎与查询模块共用
// （记录见 docs/decisions.md）。Simple/Response 用别名与包装结构保持 y=f(x) 的
// 类型化外壳，不把 GORM 实体直接暴露为公开业务方法的裸返回。

// MonitorSearch 是监控分页查询参数。
type MonitorSearch struct {
	Keyword  string `form:"keyword"`
	Type     string `form:"type"`
	Status   string `form:"status"`
	GroupID  string `form:"groupId"`
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
}

// MonitorSimple 是监控列表行：监控项数据与 24 小时可用率。
type MonitorSimple struct {
	shared.Monitor
	Availability24h float64 `json:"availability24h"`
	HasAvailability bool    `json:"hasAvailability"`
}

// MonitorResponse 是详情与创建、修改、启停操作的业务结果。
type MonitorResponse = shared.Monitor

// MonitorOKResponse 是删除等动作的结果回执。
type MonitorOKResponse struct {
	OK bool `json:"ok"`
}

// MonitorCheckResponse 是手动检测的业务结果。
type MonitorCheckResponse struct {
	Monitor *MonitorResponse    `json:"monitor"`
	Result  *checker.Result     `json:"result"`
	Events  []shared.AlertEvent `json:"events"`
}

// MonitorCheckSearch 是检测记录分页查询参数。
type MonitorCheckSearch struct {
	ID       uint `form:"-"`
	Page     int  `form:"page"`
	PageSize int  `form:"pageSize"`
}

// MonitorEventSearch 是告警事件分页查询参数。MonitorID 为空表示跨监控查询。
type MonitorEventSearch struct {
	MonitorID string `form:"monitorId"`
	EventType string `form:"eventType"`
	Page      int    `form:"page"`
	PageSize  int    `form:"pageSize"`
}

// EventSimple 是告警事件列表行。
type EventSimple = shared.AlertEvent

// MonitorMetricSearch 是监控指标查询参数。
type MonitorMetricSearch struct {
	ID    uint   `form:"-"`
	Range string `form:"range"`
}

// CertificateSimple 是证书视图行（TLS 监控项的证书信息）。
type CertificateSimple = shared.Monitor

// CertificateListResponse 是证书列表结果。
type CertificateListResponse struct {
	Items []CertificateSimple `json:"items"`
}

// MonitorExportRequest 是监控表格导出参数。
type MonitorExportRequest struct {
	Template bool   `form:"template"`
	Keyword  string `form:"keyword"`
	Type     string `form:"type"`
	Status   string `form:"status"`
	GroupID  string `form:"groupId"`
}

// MonitorImportRequest 是监控表格导入参数，内容由 Controller 提取。
type MonitorImportRequest struct {
	Data            []byte
	DefaultInterval int
	DefaultTimeout  int
}

// MonitorExportResponse 承载导出文件信息；文件字节由 Controller 按下载协议输出。
type MonitorExportResponse struct {
	Filename string `json:"filename"`
	Data     []byte `json:"-"`
}

// RequestExportRequest 是网络请求表格导出参数。
type RequestExportRequest struct {
	Template bool   `form:"template"`
	Keyword  string `form:"keyword"`
	Status   string `form:"status"`
	GroupID  string `form:"groupId"`
}
