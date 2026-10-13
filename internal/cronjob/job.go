package cronjob

import (
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cng1985/nwatch/internal/model"
	"gorm.io/gorm"
)

// Request 是创建或更新定时任务的请求。
type Request struct {
	Name        string `json:"name"`
	Cron        string `json:"cron"`
	Command     string `json:"command"`
	WorkDir     string `json:"workDir"`
	Timeout     int    `json:"timeout"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

func (r *Request) Normalize() error {
	r.Name = strings.TrimSpace(r.Name)
	r.Command = strings.TrimSpace(r.Command)
	r.WorkDir = strings.TrimSpace(r.WorkDir)
	r.Description = strings.TrimSpace(r.Description)
	if r.Name == "" {
		return errors.New("名称不能为空")
	}
	if len(r.Name) > 128 {
		return errors.New("名称过长")
	}
	norm, err := NormalizeExpr(r.Cron)
	if err != nil {
		return err
	}
	if _, err := Next(norm, time.Now()); err != nil {
		return err
	}
	r.Cron = norm
	if r.Command == "" {
		return errors.New("脚本内容不能为空")
	}
	if len(r.Command) > 8192 || strings.ContainsRune(r.Command, 0) {
		return errors.New("脚本内容不合法")
	}
	if r.WorkDir != "" && !filepath.IsAbs(r.WorkDir) {
		return errors.New("工作目录需要是绝对路径")
	}
	if len(r.WorkDir) > 1024 {
		return errors.New("工作目录过长")
	}
	if r.Timeout <= 0 {
		r.Timeout = 60
	}
	if r.Timeout < 1 || r.Timeout > 3600 {
		return errors.New("超时时间需在 1 秒到 1 小时之间")
	}
	if len(r.Description) > 512 {
		return errors.New("说明过长")
	}
	if r.Enabled == nil {
		on := true
		r.Enabled = &on
	}
	return nil
}

// Apply 把请求写进任务。表达式变化，或从停用改为启用且下次执行已过期时，重新计算下次执行。
func (r *Request) Apply(job *model.CronJob, now time.Time) error {
	exprChanged := job.Cron != r.Cron
	wasEnabled := job.Enabled
	enabled := wasEnabled
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	job.Name = r.Name
	job.Cron = r.Cron
	job.Command = r.Command
	job.WorkDir = r.WorkDir
	job.Timeout = r.Timeout
	job.Description = r.Description
	job.Enabled = enabled
	needNext := exprChanged || job.NextRunAt == nil || (!wasEnabled && enabled) || (job.NextRunAt != nil && !job.NextRunAt.After(now) && enabled)
	if !needNext {
		return nil
	}
	next, err := Next(r.Cron, now)
	if err != nil {
		return err
	}
	job.NextRunAt = &next
	return nil
}

func Decorate(job *model.CronJob) {
	if job == nil {
		return
	}
	job.ScheduleText = Describe(job.Cron)
}

type Preview struct {
	Cron     string      `json:"cron"`
	Text     string      `json:"text"`
	Timezone string      `json:"timezone"`
	Next     []time.Time `json:"next"`
}

func BuildPreview(raw, timezone string, from time.Time, count int) (Preview, error) {
	e, err := Parse(raw)
	if err != nil {
		return Preview{}, err
	}
	next, err := e.NextN(from, count)
	if err != nil {
		return Preview{}, err
	}
	if timezone == "" {
		timezone = from.Location().String()
	}
	return Preview{Cron: e.raw, Text: Describe(e.raw), Timezone: timezone, Next: next}, nil
}

// Prune 只保留最近 30 天、最多 5000 条执行记录。
func Prune(db *gorm.DB) {
	if db == nil {
		return
	}
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	_ = db.Where("started_at < ?", cutoff).Delete(&model.CronRun{}).Error
	var keep model.CronRun
	err := db.Model(&model.CronRun{}).Order("id desc").Offset(4999).Limit(1).Take(&keep).Error
	if err != nil || keep.ID == 0 {
		return
	}
	_ = db.Where("id < ?", keep.ID).Delete(&model.CronRun{}).Error
}

func clip(s string, max int) string {
	if max <= 0 || s == "" || len(s) <= max {
		return s
	}
	s = s[:max]
	for s != "" && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return clip(strings.TrimSpace(s), 180)
}
