package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/cng1985/nwatch/internal/modules/monitoring/dashboard"
	"github.com/cng1985/nwatch/internal/modules/monitoring/metric"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"github.com/cng1985/nwatch/internal/modules/notify/notifier"
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository 负责监控项的持久化、事务与类型化结果。
// Resource 不写 SQL；这里集中所有 GORM 查询与原子写入。
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) scope(ctx context.Context, search *MonitorSearch) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&shared.Monitor{})
	if kw := search.Keyword; kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR url LIKE ? OR host LIKE ?", like, like, like)
	}
	if t := search.Type; t != "" {
		q = q.Where("type = ?", t)
	}
	if st := search.Status; st != "" {
		q = q.Where("status = ?", st)
	}
	if gid := search.GroupID; gid != "" {
		q = q.Where("group_id = ?", gid)
	}
	return q
}

// Search 分页查询监控项并附带 24 小时可用率。
func (r *Repository) Search(ctx context.Context, search *MonitorSearch) (response.PageResponse[MonitorSimple], error) {
	var result response.PageResponse[MonitorSimple]
	q := r.scope(ctx, search).Preload("Group").Preload("Notifiers")
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return result, err
	}
	var items []shared.Monitor
	if err := q.Order(dashboard.StatusOrder()).Offset((search.Page - 1) * search.PageSize).Limit(search.PageSize).Find(&items).Error; err != nil {
		return result, err
	}
	avail, err := metric.AvailabilityByMonitor(r.db.WithContext(ctx), time.Now().Add(-24*time.Hour), shared.BucketHour)
	if err != nil {
		return result, err
	}
	rows := make([]MonitorSimple, 0, len(items))
	for _, item := range items {
		row := MonitorSimple{Monitor: item}
		if stat, ok := avail[item.ID]; ok && stat.Total > 0 {
			row.HasAvailability = true
			row.Availability24h = float64(stat.Success) / float64(stat.Total) * 100
		}
		rows = append(rows, row)
	}
	return response.Page(rows, total, search.Page, search.PageSize), nil
}

// SearchForExport 查询导出用的监控集合，单次导出上限 10000 条。
func (r *Repository) SearchForExport(ctx context.Context, search *MonitorSearch, limit int) ([]shared.Monitor, error) {
	q := r.scope(ctx, search).Preload("Group").Preload("Notifiers")
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	if total > int64(limit) {
		return nil, errors.BadRequest(fmt.Sprintf("一次最多导出 %d 条，请先筛选", limit))
	}
	var items []shared.Monitor
	if err := q.Order("id").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// HTTPMonitorsForExport 查询导出用的网络请求监控集合。
func (r *Repository) HTTPMonitorsForExport(ctx context.Context, search *RequestExportRequest, limit int) ([]shared.Monitor, error) {
	q := r.db.WithContext(ctx).Model(&shared.Monitor{}).Where("type = ?", shared.TypeHTTP)
	if kw := search.Keyword; kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR url LIKE ?", like, like)
	}
	if st := search.Status; st != "" {
		q = q.Where("status = ?", st)
	}
	if gid := search.GroupID; gid != "" {
		q = q.Where("group_id = ?", gid)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	if total > int64(limit) {
		return nil, errors.BadRequest(fmt.Sprintf("一次最多导出 %d 条，请先筛选", limit))
	}
	var items []shared.Monitor
	if err := q.Preload("Group").Order("id").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// One 按 ID 读取监控详情。不存在返回分类 NotFound。
func (r *Repository) One(ctx context.Context, id uint) (response.Response[MonitorResponse], error) {
	var m shared.Monitor
	if err := r.db.WithContext(ctx).Preload("Group").Preload("Notifiers").First(&m, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.Response[MonitorResponse]{}, errors.NotFound("监控不存在")
		}
		return response.Response[MonitorResponse]{}, err
	}
	return response.OK(m), nil
}

// Create 创建监控项并绑定通知渠道，一次事务完成。
func (r *Repository) Create(ctx context.Context, request *MonitorRequest) (response.Response[MonitorResponse], error) {
	var result response.Response[MonitorResponse]
	var created shared.Monitor
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateGroupRef(tx, request.GroupID); err != nil {
			return err
		}
		notifiers, err := loadNotifiers(tx, request.NotifierIDs)
		if err != nil {
			return err
		}
		entity := MonitorRequestToEntity(request)
		TouchNew(entity)
		entity.Notifiers = notifiers
		if err := tx.Omit(clause.Associations).Create(entity).Error; err != nil {
			return err
		}
		if err := tx.Model(entity).Association("Notifiers").Replace(notifiers); err != nil {
			return err
		}
		created = *entity
		return nil
	})
	if err != nil {
		return result, err
	}
	return r.One(ctx, created.ID)
}

// Update 先读取已有实体，应用允许修改的字段后保存并整体替换通知绑定。
func (r *Repository) Update(ctx context.Context, request *MonitorRequest) (response.Response[MonitorResponse], error) {
	var result response.Response[MonitorResponse]
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m shared.Monitor
		if err := tx.Preload("Group").Preload("Notifiers").First(&m, request.ID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.NotFound("监控不存在")
			}
			return err
		}
		if err := validateGroupRef(tx, request.GroupID); err != nil {
			return err
		}
		notifiers, err := loadNotifiers(tx, request.NotifierIDs)
		if err != nil {
			return err
		}
		wasEnabled := m.Enabled
		request.Apply(&m)
		if wasEnabled && !m.Enabled {
			m.Status = shared.StatusPaused
		}
		if !wasEnabled && m.Enabled {
			m.Status = shared.StatusUnknown
			m.ConsecutiveFailures = 0
			m.ConsecutiveSuccesses = 0
			now := time.Now()
			m.NextCheckAt = &now
		}
		if err := tx.Omit(clause.Associations).Save(&m).Error; err != nil {
			return err
		}
		if err := tx.Model(&m).Association("Notifiers").Replace(notifiers); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	return r.One(ctx, request.ID)
}

// Delete 删除监控项并级联清理检测记录、指标与绑定，一次事务完成。
func (r *Repository) Delete(ctx context.Context, id uint) (response.Response[MonitorOKResponse], error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m shared.Monitor
		if err := tx.First(&m, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.NotFound("监控不存在")
			}
			return err
		}
		if err := tx.Model(&m).Association("Notifiers").Clear(); err != nil {
			return err
		}
		if err := tx.Where("monitor_id = ?", id).Delete(&shared.MonitorCheck{}).Error; err != nil {
			return err
		}
		if err := tx.Where("monitor_id = ?", id).Delete(&shared.MonitorMetric{}).Error; err != nil {
			return err
		}
		return tx.Delete(&m).Error
	})
	if err != nil {
		return response.Response[MonitorOKResponse]{}, err
	}
	return response.OK(MonitorOKResponse{OK: true}), nil
}

// SetEnabled 启用或暂停监控项。
func (r *Repository) SetEnabled(ctx context.Context, id uint, enabled bool) (response.Response[MonitorResponse], error) {
	var m shared.Monitor
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.Response[MonitorResponse]{}, errors.NotFound("监控不存在")
		}
		return response.Response[MonitorResponse]{}, err
	}
	now := time.Now()
	updates := map[string]any{"enabled": enabled, "updated_at": now}
	if enabled {
		updates["status"] = shared.StatusUnknown
		updates["consecutive_failures"] = 0
		updates["consecutive_successes"] = 0
		updates["next_check_at"] = now
	} else {
		updates["status"] = shared.StatusPaused
	}
	if err := r.db.WithContext(ctx).Model(&m).Updates(updates).Error; err != nil {
		return response.Response[MonitorResponse]{}, err
	}
	return r.One(ctx, id)
}

// SearchChecks 分页查询检测记录。
func (r *Repository) SearchChecks(ctx context.Context, search *MonitorCheckSearch) (response.PageResponse[shared.MonitorCheck], error) {
	var result response.PageResponse[shared.MonitorCheck]
	q := r.db.WithContext(ctx).Model(&shared.MonitorCheck{}).Where("monitor_id = ?", search.ID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return result, err
	}
	items := make([]shared.MonitorCheck, 0)
	if err := q.Order("checked_at desc").Offset((search.Page - 1) * search.PageSize).Limit(search.PageSize).Find(&items).Error; err != nil {
		return result, err
	}
	return response.Page(items, total, search.Page, search.PageSize), nil
}

// SearchEvents 分页查询告警事件，MonitorID 为空时跨监控查询。
func (r *Repository) SearchEvents(ctx context.Context, search *MonitorEventSearch) (response.PageResponse[EventSimple], error) {
	var result response.PageResponse[EventSimple]
	q := r.db.WithContext(ctx).Model(&shared.AlertEvent{})
	if v := search.MonitorID; v != "" {
		q = q.Where("monitor_id = ?", v)
	}
	if v := search.EventType; v != "" {
		q = q.Where("event_type = ?", v)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return result, err
	}
	items := make([]EventSimple, 0)
	if err := q.Order("occurred_at desc").Offset((search.Page - 1) * search.PageSize).Limit(search.PageSize).Find(&items).Error; err != nil {
		return result, err
	}
	return response.Page(items, total, search.Page, search.PageSize), nil
}

// ListCertificates 按到期时间列出 TLS 监控的证书视图。
func (r *Repository) ListCertificates(ctx context.Context) (response.Response[CertificateListResponse], error) {
	items := make([]CertificateSimple, 0)
	err := r.db.WithContext(ctx).Where("type = ?", shared.TypeTLS).
		Order("CASE WHEN cert_days_remaining IS NULL THEN 1 ELSE 0 END, cert_days_remaining ASC, id ASC").
		Find(&items).Error
	if err != nil {
		return response.Response[CertificateListResponse]{}, err
	}
	return response.OK(CertificateListResponse{Items: items}), nil
}

// ImportMonitors 导入监控表格。
func (r *Repository) ImportMonitors(ctx context.Context, request *MonitorImportRequest) (response.Response[ImportStats], error) {
	stats, err := ImportMonitorsXLSX(r.db.WithContext(ctx), newByteReader(request.Data), request.DefaultInterval, request.DefaultTimeout)
	if err != nil {
		return response.Response[ImportStats]{}, importError(err)
	}
	return response.OK(stats), nil
}

// ImportRequests 导入网络请求表格。
func (r *Repository) ImportRequests(ctx context.Context, request *MonitorImportRequest) (response.Response[ImportStats], error) {
	stats, err := ImportRequestsXLSX(r.db.WithContext(ctx), newByteReader(request.Data), request.DefaultInterval, request.DefaultTimeout)
	if err != nil {
		return response.Response[ImportStats]{}, importError(err)
	}
	return response.OK(stats), nil
}

func validateGroupRef(tx *gorm.DB, id *uint) error {
	if id == nil || *id == 0 {
		return nil
	}
	var n int64
	if err := tx.Model(&shared.Group{}).Where("id = ?", *id).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return errors.BadRequest("监控分组不存在")
	}
	return nil
}

func loadNotifiers(tx *gorm.DB, ids []uint) ([]notifier.Notifier, error) {
	ids = uniqueUint(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []notifier.Notifier
	if err := tx.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, errors.BadRequest("通知渠道不存在")
	}
	return rows, nil
}

func uniqueUint(ids []uint) []uint {
	seen := map[uint]struct{}{}
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
