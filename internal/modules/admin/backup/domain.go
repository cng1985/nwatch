// Package backup 提供数据库备份与配置导入导出业务。
package backup

// BackupFileResponse 是手动备份结果，返回备份文件位置。
type BackupFileResponse struct {
	File string `json:"file"`
}

// BackupOKResponse 是导入动作回执。
type BackupOKResponse struct {
	OK bool `json:"ok"`
}

// ExportRequest 是配置导出参数。
type ExportRequest struct {
	IncludeSecrets bool `form:"includeSecrets"`
}

// ImportRequest 是配置导入参数，内嵌导出文件结构，内容由 Controller 绑定。
type ImportRequest struct {
	Bundle
}

// ExportResponse 是配置导出结果。
type ExportResponse = Bundle
