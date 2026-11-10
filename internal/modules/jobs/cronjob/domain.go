package cronjob

import "time"

// CronJobSearch 是定时任务分页查询参数。
type CronJobSearch struct {
	Keyword  string `form:"keyword"`
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
}

// CronJobSimple 与 CronJobResponse 是定时任务业务结果。
type CronJobSimple = CronJob
type CronJobResponse = CronJob

// CronJobOKResponse 是删除动作回执。
type CronJobOKResponse struct {
	OK bool `json:"ok"`
}

// CronRunSearch 是执行记录分页查询参数。FromTime/ToTime 由 Resource 解析后填充。
type CronRunSearch struct {
	JobID    string    `form:"jobId"`
	Keyword  string    `form:"keyword"`
	Success  string    `form:"success"`
	Trigger  string    `form:"trigger"`
	From     string    `form:"from"`
	To       string    `form:"to"`
	FromTime time.Time `form:"-"`
	ToTime   time.Time `form:"-"`
	Page     int       `form:"page"`
	PageSize int       `form:"pageSize"`
}

// CronRunSimple 是执行记录列表行。
type CronRunSimple = CronRun

// CronRunResponse 是立即执行的结果，即一条执行记录。
type CronRunResponse = CronRun

// CronPreviewRequest 是 cron 表达式预览参数。
type CronPreviewRequest struct {
	Expr  string `form:"expr"`
	Count int    `form:"count"`
}
