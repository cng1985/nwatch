package cronjob

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const (
	CronSuccess = "SUCCESS"
	CronFailed  = "FAILED"
	CronTimeout = "TIMEOUT"
	CronRunning = "RUNNING"

	CronTriggerSchedule = "schedule"
	CronTriggerManual   = "manual"

	CronKindScript = "script"
	CronKindHTTP   = "http"

	CronBodyForm     = "form"
	CronBodyJSON     = "json"
	CronBodyFormData = "formdata"
	CronBodyXML      = "xml"
	CronBodyText     = "text"
)

// CronJob 是按 cron 表达式执行的定时任务。
type CronJob struct {
	ID          uint              `gorm:"primaryKey" json:"id"`
	Name        string            `gorm:"size:128;not null;uniqueIndex" json:"name"`
	Cron        string            `gorm:"size:128;not null" json:"cron"`
	Kind        string            `gorm:"size:16;index" json:"kind"`
	Command     string            `gorm:"type:text" json:"command"`
	WorkDir     string            `gorm:"size:1024" json:"workDir"`
	URL         string            `gorm:"size:2048" json:"url"`
	Method      string            `gorm:"size:16" json:"method"`
	BodyType    string            `gorm:"size:16" json:"bodyType"`
	HeadersRaw  string            `gorm:"column:headers;type:text" json:"-"`
	Headers     map[string]string `gorm:"-" json:"headers"`
	Body        string            `gorm:"type:text" json:"body"`
	Timeout     int               `gorm:"not null" json:"timeout"`
	Description string            `gorm:"size:512" json:"description"`
	Enabled     bool              `gorm:"not null;index" json:"enabled"`

	LastRunAt      *time.Time `json:"lastRunAt"`
	NextRunAt      *time.Time `gorm:"index" json:"nextRunAt"`
	LastStatus     string     `gorm:"size:16" json:"lastStatus"`
	LastExitCode   int        `json:"lastExitCode"`
	LastMessage    string     `gorm:"size:2048" json:"lastMessage"`
	LastDurationMs int        `json:"lastDurationMs"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	ScheduleText string `gorm:"-" json:"scheduleText"`
	TargetText   string `gorm:"-" json:"targetText"`
}

func (CronJob) TableName() string { return "cron_jobs" }

func (j *CronJob) AfterFind(*gorm.DB) error {
	j.Headers = map[string]string{}
	if j.HeadersRaw != "" {
		_ = json.Unmarshal([]byte(j.HeadersRaw), &j.Headers)
	}
	if j.Kind == "" {
		j.Kind = CronKindScript
	}
	return nil
}

func (j *CronJob) SyncHeaders() {
	if len(j.Headers) == 0 {
		j.HeadersRaw = ""
		return
	}
	raw, _ := json.Marshal(j.Headers)
	j.HeadersRaw = string(raw)
}

func (j *CronJob) IsHTTP() bool { return j != nil && j.Kind == CronKindHTTP }

// CronRun 是一次定时任务执行的输出。
type CronRun struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	JobID        uint      `gorm:"index" json:"jobId"`
	Name         string    `gorm:"size:128" json:"name"`
	Cron         string    `gorm:"size:128" json:"cron"`
	Command      string    `gorm:"type:text" json:"command"`
	WorkDir      string    `gorm:"size:1024" json:"workDir"`
	ExitCode     int       `json:"exitCode"`
	Success      bool      `json:"success"`
	Stdout       string    `gorm:"type:text" json:"stdout"`
	Stderr       string    `gorm:"type:text" json:"stderr"`
	ErrorMessage string    `gorm:"size:2048" json:"errorMessage"`
	DurationMs   int       `json:"durationMs"`
	Trigger      string    `gorm:"size:16" json:"trigger"`
	StartedAt    time.Time `gorm:"index" json:"startedAt"`
	FinishedAt   time.Time `json:"finishedAt"`
}

func (CronRun) TableName() string { return "cron_runs" }
