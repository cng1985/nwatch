package monitor

import (
	"context"
	"strconv"
	"time"

	"github.com/cng1985/nwatch/internal/modules/monitoring/metric"
	"github.com/cng1985/nwatch/internal/modules/monitoring/scheduler"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"github.com/cng1985/nwatch/internal/platform/domain"
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/cng1985/nwatch/internal/platform/settings"
)

// Resource 编排监控项业务：参数校验、业务规则与动作入口，不写 SQL。
type Resource struct {
	repo      *Repository
	settings  *settings.Store
	pool      *scheduler.Pool
	processor *scheduler.Processor
}

func NewResource(repo *Repository, store *settings.Store, pool *scheduler.Pool, processor *scheduler.Processor) *Resource {
	return &Resource{repo: repo, settings: store, pool: pool, processor: processor}
}

func (r *Resource) defaults() (int, int) {
	snap := r.settings.Snapshot()
	return snap.DefaultInterval, snap.DefaultTimeout
}

// Create 创建监控项。客户端自设主键与非法参数直接拒绝。
func (r *Resource) Create(ctx context.Context, request *MonitorRequest) (response.Response[MonitorResponse], error) {
	if request == nil {
		return response.Response[MonitorResponse]{}, errors.BadRequest("请求格式不正确")
	}
	if request.ID != 0 {
		return response.Response[MonitorResponse]{}, errors.BadRequest("创建不接受客户端指定主键")
	}
	defInterval, defTimeout := r.defaults()
	if err := request.Normalize(defInterval, defTimeout); err != nil {
		return response.Response[MonitorResponse]{}, errors.BadRequest(err.Error())
	}
	return r.repo.Create(ctx, request)
}

// Update 修改监控项，只应用允许修改的字段。
func (r *Resource) Update(ctx context.Context, request *MonitorRequest) (response.Response[MonitorResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[MonitorResponse]{}, errors.BadRequest("无效的监控编号")
	}
	defInterval, defTimeout := r.defaults()
	if err := request.Normalize(defInterval, defTimeout); err != nil {
		return response.Response[MonitorResponse]{}, errors.BadRequest(err.Error())
	}
	return r.repo.Update(ctx, request)
}

// One 查询单个监控详情。
func (r *Resource) One(ctx context.Context, request *MonitorRequest) (response.Response[MonitorResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[MonitorResponse]{}, errors.BadRequest("无效的监控编号")
	}
	return r.repo.One(ctx, request.ID)
}

// Delete 删除监控项及其检测记录。
func (r *Resource) Delete(ctx context.Context, request *MonitorRequest) (response.Response[MonitorOKResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[MonitorOKResponse]{}, errors.BadRequest("无效的监控编号")
	}
	return r.repo.Delete(ctx, request.ID)
}

// Enable 启用监控项。
func (r *Resource) Enable(ctx context.Context, request *MonitorRequest) (response.Response[MonitorResponse], error) {
	return r.SetEnabled(ctx, request, true)
}

// Disable 暂停监控项。
func (r *Resource) Disable(ctx context.Context, request *MonitorRequest) (response.Response[MonitorResponse], error) {
	return r.SetEnabled(ctx, request, false)
}

func (r *Resource) SetEnabled(ctx context.Context, request *MonitorRequest, enabled bool) (response.Response[MonitorResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[MonitorResponse]{}, errors.BadRequest("无效的监控编号")
	}
	return r.repo.SetEnabled(ctx, request.ID, enabled)
}

// Check 手动执行一次检测。同一监控并发检测直接冲突返回。
func (r *Resource) Check(ctx context.Context, request *MonitorRequest) (response.Response[MonitorCheckResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[MonitorCheckResponse]{}, errors.BadRequest("无效的监控编号")
	}
	if !r.pool.TryAcquire(request.ID) {
		return response.Response[MonitorCheckResponse]{}, errors.Conflict("该监控正在检测中")
	}
	defer r.pool.Release(request.ID)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	out, err := r.processor.Process(ctx, request.ID, true)
	if err != nil {
		return response.Response[MonitorCheckResponse]{}, errors.BadRequest(err.Error())
	}
	events := out.Events
	if events == nil {
		events = []shared.AlertEvent{}
	}
	return response.OK(MonitorCheckResponse{Monitor: out.Monitor, Result: out.Result, Events: events}), nil
}

// Search 分页查询监控项。
func (r *Resource) Search(ctx context.Context, search *MonitorSearch) (response.PageResponse[MonitorSimple], error) {
	if err := r.normalizeSearch(search); err != nil {
		return response.PageResponse[MonitorSimple]{}, err
	}
	return r.repo.Search(ctx, search)
}

// SearchChecks 分页查询检测记录。
func (r *Resource) SearchChecks(ctx context.Context, search *MonitorCheckSearch) (response.PageResponse[shared.MonitorCheck], error) {
	if search == nil || search.ID == 0 {
		return response.PageResponse[shared.MonitorCheck]{}, errors.BadRequest("无效的监控编号")
	}
	if err := normalizePage(&search.Page, &search.PageSize); err != nil {
		return response.PageResponse[shared.MonitorCheck]{}, err
	}
	return r.repo.SearchChecks(ctx, search)
}

// SearchEvents 分页查询告警事件。
func (r *Resource) SearchEvents(ctx context.Context, search *MonitorEventSearch) (response.PageResponse[EventSimple], error) {
	if search == nil {
		return response.PageResponse[EventSimple]{}, errors.BadRequest("请求格式不正确")
	}
	if err := normalizePage(&search.Page, &search.PageSize); err != nil {
		return response.PageResponse[EventSimple]{}, err
	}
	return r.repo.SearchEvents(ctx, search)
}

// ListCertificates 列出证书视图。
func (r *Resource) ListCertificates(ctx context.Context) (response.Response[CertificateListResponse], error) {
	return r.repo.ListCertificates(ctx)
}

// MetricSummary 查询单个监控的指标汇总。
func (r *Resource) MetricSummary(ctx context.Context, search *MonitorMetricSearch) (response.Response[metric.Summary], error) {
	if search == nil || search.ID == 0 {
		return response.Response[metric.Summary]{}, errors.BadRequest("无效的监控编号")
	}
	from, bucket := metricRange(search.Range)
	sum, err := metric.Query(r.repo.db.WithContext(ctx), search.ID, from, bucket)
	if err != nil {
		return response.Response[metric.Summary]{}, err
	}
	return response.OK(sum), nil
}

// ExportMonitors 导出监控表格。文件字节由 Controller 按下载协议输出。
func (r *Resource) ExportMonitors(ctx context.Context, request *MonitorExportRequest) (response.Response[MonitorExportResponse], error) {
	loc := r.settings.Location()
	if request.Template {
		data, err := MonitorsToXLSX(nil, nil, loc)
		if err != nil {
			return response.Response[MonitorExportResponse]{}, err
		}
		return response.OK(MonitorExportResponse{Filename: "nmonitor-monitors-template.xlsx", Data: data}), nil
	}
	search := &MonitorSearch{Keyword: request.Keyword, Type: request.Type, Status: request.Status, GroupID: request.GroupID}
	monitors, err := r.repo.SearchForExport(ctx, search, maxExportRows)
	if err != nil {
		return response.Response[MonitorExportResponse]{}, err
	}
	avail, err := metric.AvailabilityByMonitor(r.repo.db.WithContext(ctx), time.Now().Add(-24*time.Hour), shared.BucketHour)
	if err != nil {
		return response.Response[MonitorExportResponse]{}, err
	}
	availability := map[uint]string{}
	for id, stat := range avail {
		if stat.Total > 0 {
			availability[id] = formatPercent(float64(stat.Success)/float64(stat.Total)*100) + "%"
		}
	}
	data, err := MonitorsToXLSX(monitors, availability, loc)
	if err != nil {
		return response.Response[MonitorExportResponse]{}, err
	}
	name := "nmonitor-monitors-" + time.Now().In(loc).Format("20060102-150405") + ".xlsx"
	return response.OK(MonitorExportResponse{Filename: name, Data: data}), nil
}

// ExportRequests 导出网络请求表格。
func (r *Resource) ExportRequests(ctx context.Context, request *RequestExportRequest) (response.Response[MonitorExportResponse], error) {
	if request.Template {
		data, err := RequestsToXLSX(nil)
		if err != nil {
			return response.Response[MonitorExportResponse]{}, err
		}
		return response.OK(MonitorExportResponse{Filename: "nmonitor-requests-template.xlsx", Data: data}), nil
	}
	monitors, err := r.repo.HTTPMonitorsForExport(ctx, request, maxExportRows)
	if err != nil {
		return response.Response[MonitorExportResponse]{}, err
	}
	data, err := RequestsToXLSX(monitors)
	if err != nil {
		return response.Response[MonitorExportResponse]{}, err
	}
	loc := r.settings.Location()
	name := "nmonitor-requests-" + time.Now().In(loc).Format("20060102-150405") + ".xlsx"
	return response.OK(MonitorExportResponse{Filename: name, Data: data}), nil
}

// ImportMonitors 导入监控表格。
func (r *Resource) ImportMonitors(ctx context.Context, request *MonitorImportRequest) (response.Response[ImportStats], error) {
	if request == nil || len(request.Data) == 0 {
		return response.Response[ImportStats]{}, errors.BadRequest("请选择 xlsx 文件")
	}
	defInterval, defTimeout := r.defaults()
	request.DefaultInterval = defInterval
	request.DefaultTimeout = defTimeout
	return r.repo.ImportMonitors(ctx, request)
}

// ImportRequests 导入网络请求表格。
func (r *Resource) ImportRequests(ctx context.Context, request *MonitorImportRequest) (response.Response[ImportStats], error) {
	if request == nil || len(request.Data) == 0 {
		return response.Response[ImportStats]{}, errors.BadRequest("请选择 xlsx 文件")
	}
	defInterval, defTimeout := r.defaults()
	request.DefaultInterval = defInterval
	request.DefaultTimeout = defTimeout
	return r.repo.ImportRequests(ctx, request)
}

func (r *Resource) normalizeSearch(search *MonitorSearch) error {
	if search == nil {
		return errors.BadRequest("请求格式不正确")
	}
	if search.GroupID != "" && !validUintText(search.GroupID) {
		return errors.BadRequest("分组编号不正确")
	}
	return normalizePage(&search.Page, &search.PageSize)
}

// normalizePage 补全分页默认值并校验边界。
func normalizePage(page, size *int) error {
	if *page < 1 {
		*page = domain.DefaultPage
	}
	if *size < 1 {
		*size = domain.DefaultPageSize
	}
	if *size > domain.MaxPageSize {
		return errors.BadRequest("每页数量不能超过 200")
	}
	if int64(*page-1) > int64(domain.MaxOffsetRows)/int64(*size) {
		return errors.BadRequest("分页范围超出限制")
	}
	return nil
}

func metricRange(name string) (time.Time, string) {
	now := time.Now()
	switch name {
	case "1h":
		return now.Add(-time.Hour), shared.BucketMinute
	case "7d":
		return now.AddDate(0, 0, -7), shared.BucketHour
	case "30d":
		return now.AddDate(0, 0, -30), shared.BucketDay
	default:
		return now.Add(-24 * time.Hour), shared.BucketHour
	}
}

func validUintText(s string) bool {
	if s == "" {
		return true
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func formatPercent(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
