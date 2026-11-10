// Package script 提供手动脚本执行与执行记录查询业务。
package script

import "github.com/cng1985/nwatch/internal/modules/monitoring/shared"

// ScriptRunRequest 是手动执行脚本的参数。
type ScriptRunRequest struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	WorkDir string `json:"workDir"`
	Timeout int    `json:"timeout"`
}

// ScriptRunResponse 是脚本执行结果，即一条执行记录。
type ScriptRunResponse = shared.ScriptRun

// ScriptRunSimple 是执行记录列表行。
type ScriptRunSimple = shared.ScriptRun

// ScriptRunSearch 是执行记录分页查询参数。
type ScriptRunSearch struct {
	Keyword   string `form:"keyword"`
	MonitorID string `form:"monitorId"`
	Page      int    `form:"page"`
	PageSize  int    `form:"pageSize"`
}

// ScriptRunOneRequest 是单条执行记录查询参数。
type ScriptRunOneRequest struct {
	ID uint `json:"id"`
}
