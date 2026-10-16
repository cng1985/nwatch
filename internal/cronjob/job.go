package cronjob

import (
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cng1985/nwatch/internal/httpreq"
	"github.com/cng1985/nwatch/internal/model"
	"gorm.io/gorm"
)

// Request 是创建或更新定时任务的请求。
type Request struct {
	Name        string            `json:"name"`
	Cron        string            `json:"cron"`
	Kind        string            `json:"kind"`
	Command     string            `json:"command"`
	WorkDir     string            `json:"workDir"`
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	BodyType    string            `json:"bodyType"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	Timeout     int               `json:"timeout"`
	Description string            `json:"description"`
	Enabled     *bool             `json:"enabled"`
}

func (r *Request) Normalize() error {
	r.Name = strings.TrimSpace(r.Name)
	r.Kind = strings.ToLower(strings.TrimSpace(r.Kind))
	r.Command = strings.TrimSpace(r.Command)
	r.WorkDir = strings.TrimSpace(r.WorkDir)
	r.URL = strings.TrimSpace(r.URL)
	r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
	r.BodyType = strings.ToLower(strings.TrimSpace(r.BodyType))
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
	if r.Kind == "" {
		r.Kind = model.CronKindScript
	}
	switch r.Kind {
	case model.CronKindScript:
		if err := r.normalizeScript(); err != nil {
			return err
		}
	case model.CronKindHTTP:
		if err := r.normalizeHTTP(); err != nil {
			return err
		}
	default:
		return errors.New("任务类型仅支持脚本和网络请求")
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
	job.Kind = r.Kind
	job.Command = r.Command
	job.WorkDir = r.WorkDir
	job.URL = r.URL
	job.Method = r.Method
	job.BodyType = r.BodyType
	job.Headers = r.Headers
	job.SyncHeaders()
	job.Body = r.Body
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
	if job.Kind == "" {
		job.Kind = model.CronKindScript
	}
	job.ScheduleText = Describe(job.Cron)
	job.TargetText = targetText(job)
}

func targetText(job *model.CronJob) string {
	if job.IsHTTP() {
		switch {
		case job.Method == "GET":
			return "GET " + job.URL
		case job.BodyType == model.CronBodyJSON:
			return "POST JSON " + job.URL
		case job.BodyType == model.CronBodyForm:
			return "POST 表单 " + job.URL
		case job.BodyType == model.CronBodyFormData:
			return "POST FormData " + job.URL
		case job.BodyType == model.CronBodyXML:
			return "POST XML " + job.URL
		case job.BodyType == model.CronBodyText:
			return "POST 文本 " + job.URL
		default:
			return "POST " + job.URL
		}
	}
	return firstLine(job.Command)
}

func (r *Request) normalizeScript() error {
	r.Method = ""
	r.BodyType = ""
	r.URL = ""
	r.Headers = nil
	r.Body = ""
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
	return nil
}

func (r *Request) normalizeHTTP() error {
	r.Command = ""
	r.WorkDir = ""
	u, err := url.Parse(r.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("网络请求需要以 http:// 或 https:// 开头的地址")
	}
	if len(r.URL) > 2048 {
		return errors.New("地址过长")
	}
	switch r.Method {
	case "GET":
		r.BodyType = ""
		r.Body = ""
	case "POST":
		bodyType, body, err := httpreq.Normalize(r.BodyType, r.Body, false)
		if err != nil {
			return err
		}
		r.BodyType = bodyType
		r.Body = body
	default:
		return errors.New("网络请求仅支持 GET 和 POST")
	}
	cleaned := map[string]string{}
	for key, value := range r.Headers {
		name := strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if name == "" || len(name) > 128 || len(value) > 4096 || strings.ContainsAny(name, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return errors.New("请求头不合法")
		}
		cleaned[name] = value
	}
	if len(cleaned) > 20 {
		return errors.New("请求头不能超过 20 个")
	}
	if len(cleaned) == 0 {
		r.Headers = nil
	} else {
		r.Headers = cleaned
	}
	return nil
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
