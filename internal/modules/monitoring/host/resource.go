package host

import (
	"context"
	"strconv"
	"strings"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"gorm.io/gorm"
)

// Resource 编排主机资源查询与进程列表。
type Resource struct {
	collector *Collector
	db        *gorm.DB
}

func NewResource(collector *Collector, db *gorm.DB) *Resource {
	return &Resource{collector: collector, db: db}
}

// Status 返回当前采样与历史曲线。
func (r *Resource) Status(ctx context.Context, request *HostStatusRequest) (response.Response[HostStatusResponse], error) {
	snap, err := r.collector.Snapshot()
	if err != nil {
		return response.Response[HostStatusResponse]{}, err
	}
	from, name := Since(request.Range)
	history, err := LoadHistory(r.db.WithContext(ctx), from)
	if err != nil {
		return response.Response[HostStatusResponse]{}, err
	}
	if len(history) == 0 {
		history = []Point{{
			Time:        snap.SampledAt,
			CPUPercent:  snap.CPUPercent,
			MemPercent:  snap.MemPercent,
			DiskPercent: RootPercent(snap.Disks),
		}}
	}
	return response.OK(HostStatusResponse{Current: snap, History: history, Range: name}), nil
}

// Processes 返回进程列表。limit 未提供时返回默认上限内的全部进程。
func (r *Resource) Processes(ctx context.Context, search *ProcessSearch) (response.Response[ProcessListResponse], error) {
	if search == nil {
		return response.Response[ProcessListResponse]{}, errors.BadRequest("请求格式不正确")
	}
	limit := 0
	if raw := trimSpace(search.Limit); raw != "" {
		n, err := parseInt(raw)
		if err != nil || n < 1 {
			return response.Response[ProcessListResponse]{}, errors.BadRequest("进程数量不正确")
		}
		limit = n
	}
	list, err := r.collector.Processes(search.Sort, limit)
	if err != nil {
		return response.Response[ProcessListResponse]{}, err
	}
	return response.OK(list), nil
}

func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}
