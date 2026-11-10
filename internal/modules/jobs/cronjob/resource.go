package cronjob

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/cng1985/nwatch/internal/platform/settings"
	"gorm.io/gorm"
)

// Resource 编排定时任务业务：校验、CRUD、执行与预览。
type Resource struct {
	repo     *Repository
	service  *Service
	settings *settings.Store
}

func NewResource(repo *Repository, service *Service, store *settings.Store) *Resource {
	return &Resource{repo: repo, service: service, settings: store}
}

// Search 分页查询定时任务。
func (r *Resource) Search(ctx context.Context, search *CronJobSearch) (response.PageResponse[CronJobSimple], error) {
	if search == nil {
		return response.PageResponse[CronJobSimple]{}, apperrors.BadRequest("请求格式不正确")
	}
	if err := normalizePaging(&search.Page, &search.PageSize); err != nil {
		return response.PageResponse[CronJobSimple]{}, err
	}
	return r.repo.Search(ctx, search)
}

// One 查询单个定时任务。
func (r *Resource) One(ctx context.Context, id uint) (response.Response[CronJobResponse], error) {
	if id == 0 {
		return response.Response[CronJobResponse]{}, apperrors.BadRequest("无效的编号")
	}
	return r.repo.One(ctx, id)
}

// Create 创建定时任务。
func (r *Resource) Create(ctx context.Context, request *CronJobRequest) (response.Response[CronJobResponse], error) {
	job, err := r.prepare(request, &CronJob{})
	if err != nil {
		return response.Response[CronJobResponse]{}, err
	}
	return r.repo.Create(ctx, job)
}

// Update 修改定时任务。
func (r *Resource) Update(ctx context.Context, id uint, request *CronJobRequest) (response.Response[CronJobResponse], error) {
	if id == 0 {
		return response.Response[CronJobResponse]{}, apperrors.BadRequest("无效的编号")
	}
	current, err := r.repo.OneEntity(ctx, id)
	if err != nil {
		return response.Response[CronJobResponse]{}, err
	}
	job, err := r.prepare(request, current)
	if err != nil {
		return response.Response[CronJobResponse]{}, err
	}
	return r.repo.Update(ctx, job)
}

// Delete 删除定时任务。执行中的任务返回冲突。
func (r *Resource) Delete(ctx context.Context, id uint) (response.Response[CronJobOKResponse], error) {
	if id == 0 {
		return response.Response[CronJobOKResponse]{}, apperrors.BadRequest("无效的编号")
	}
	if r.service.Busy(id) {
		return response.Response[CronJobOKResponse]{}, apperrors.Conflict("任务正在执行，请稍后再删除")
	}
	return r.repo.Delete(ctx, id)
}

// Enable 启用定时任务。
func (r *Resource) Enable(ctx context.Context, id uint) (response.Response[CronJobResponse], error) {
	return r.SetEnabled(ctx, id, true)
}

// Disable 停用定时任务。
func (r *Resource) Disable(ctx context.Context, id uint) (response.Response[CronJobResponse], error) {
	return r.SetEnabled(ctx, id, false)
}

func (r *Resource) SetEnabled(ctx context.Context, id uint, enabled bool) (response.Response[CronJobResponse], error) {
	if id == 0 {
		return response.Response[CronJobResponse]{}, apperrors.BadRequest("无效的编号")
	}
	job, err := r.service.SetEnabled(id, enabled)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return response.Response[CronJobResponse]{}, apperrors.NotFound("定时任务不存在")
	}
	if err != nil {
		return response.Response[CronJobResponse]{}, apperrors.BadRequest(err.Error())
	}
	Decorate(&job)
	return response.OK(job), nil
}

// Run 立即执行定时任务。
func (r *Resource) Run(ctx context.Context, id uint) (response.Response[CronRunResponse], error) {
	if id == 0 {
		return response.Response[CronRunResponse]{}, apperrors.BadRequest("无效的编号")
	}
	row, err := r.service.Run(id, CronTriggerManual)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return response.Response[CronRunResponse]{}, apperrors.NotFound("定时任务不存在")
	}
	if errors.Is(err, ErrBusy) {
		return response.Response[CronRunResponse]{}, apperrors.Conflict(err.Error())
	}
	if err != nil {
		return response.Response[CronRunResponse]{}, apperrors.Internal("执行失败")
	}
	return response.OK(*row), nil
}

// SearchRuns 分页查询执行记录。
func (r *Resource) SearchRuns(ctx context.Context, search *CronRunSearch) (response.PageResponse[CronRunSimple], error) {
	if search == nil {
		return response.PageResponse[CronRunSimple]{}, apperrors.BadRequest("请求格式不正确")
	}
	if raw := strings.TrimSpace(search.JobID); raw != "" {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || n == 0 {
			return response.PageResponse[CronRunSimple]{}, apperrors.BadRequest("任务编号不正确")
		}
		search.JobID = strconv.FormatUint(n, 10)
	} else {
		search.JobID = ""
	}
	if err := normalizePaging(&search.Page, &search.PageSize); err != nil {
		return response.PageResponse[CronRunSimple]{}, err
	}
	switch search.Success {
	case "", "true", "false":
	default:
		return response.PageResponse[CronRunSimple]{}, apperrors.BadRequest("结果条件不正确")
	}
	if trigger := strings.TrimSpace(search.Trigger); trigger != "" {
		if trigger != CronTriggerManual && trigger != CronTriggerSchedule {
			return response.PageResponse[CronRunSimple]{}, apperrors.BadRequest("触发方式不正确")
		}
	}
	if _, _, err := r.runWindow(search.From, search.To); err != nil {
		return response.PageResponse[CronRunSimple]{}, apperrors.BadRequest(err.Error())
	}
	from, to, _ := r.runWindow(search.From, search.To)
	search.FromTime, search.ToTime = from, to
	return r.repo.SearchRuns(ctx, search)
}

// Preview 预览 cron 表达式。
func (r *Resource) Preview(ctx context.Context, request *CronPreviewRequest) (response.Response[Preview], error) {
	if request == nil {
		return response.Response[Preview]{}, apperrors.BadRequest("请求格式不正确")
	}
	expr := strings.TrimSpace(request.Expr)
	count := request.Count
	if count <= 0 {
		count = 5
	}
	loc := r.settings.Location()
	view, err := BuildPreview(expr, loc.String(), time.Now().In(loc), count)
	if err != nil {
		return response.Response[Preview]{}, apperrors.BadRequest(err.Error())
	}
	return response.OK(view), nil
}

func (r *Resource) prepare(request *CronJobRequest, job *CronJob) (*CronJob, error) {
	if request == nil {
		return nil, apperrors.BadRequest("请求格式不正确")
	}
	if err := request.Normalize(); err != nil {
		return nil, apperrors.BadRequest(err.Error())
	}
	if err := request.Apply(job, r.service.Now()); err != nil {
		return nil, apperrors.BadRequest(err.Error())
	}
	return job, nil
}

// runWindow 把查询时间转成 [from, to)。日期按系统时区的自然日，to 当天包含在内。
func (r *Resource) runWindow(fromRaw, toRaw string) (time.Time, time.Time, error) {
	loc := r.settings.Location()
	from, err := parseRunBound(fromRaw, loc, false)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := parseRunBound(toRaw, loc, true)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !from.IsZero() && !to.IsZero() && !to.After(from) {
		return time.Time{}, time.Time{}, errors.New("结束时间需要晚于开始时间")
	}
	return from, to, nil
}

func parseRunBound(raw string, loc *time.Location, end bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	if loc == nil {
		loc = time.Local
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		if end {
			return t.Add(time.Second), nil
		}
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", raw, loc); err == nil {
		if end {
			return t.Add(time.Second), nil
		}
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", raw, loc); err == nil {
		if end {
			return t.AddDate(0, 0, 1), nil
		}
		return t, nil
	}
	return time.Time{}, errors.New("时间格式不正确，使用 YYYY-MM-DD")
}

// normalizePaging 补全分页默认值并校验边界。
func normalizePaging(page, size *int) error {
	if *page < 1 {
		*page = 1
	}
	if *size < 1 {
		*size = 20
	}
	if *size > 200 {
		return apperrors.BadRequest("每页数量不能超过 200")
	}
	if int64(*page-1) > 100000/int64(*size) {
		return apperrors.BadRequest("分页范围超出限制")
	}
	return nil
}
