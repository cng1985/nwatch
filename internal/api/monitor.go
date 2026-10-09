package api

import (
	"context"
	"net/http"
	"time"

	"github.com/cng1985/nwatch/internal/dashboard"
	"github.com/cng1985/nwatch/internal/metric"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/monitor"
	"github.com/gin-gonic/gin"
)

func (s *Server) listMonitors(c *gin.Context) {
	page, size := pageParams(c)
	q := s.db.Model(&model.Monitor{}).Preload("Group").Preload("Notifiers")
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR url LIKE ? OR host LIKE ?", like, like, like)
	}
	if t := c.Query("type"); t != "" {
		q = q.Where("type = ?", t)
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if gid := c.Query("groupId"); gid != "" {
		q = q.Where("group_id = ?", gid)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	var items []model.Monitor
	if err := q.Order(dashboard.StatusOrder()).Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	avail, err := metric.AvailabilityByMonitor(s.db, time.Now().Add(-24*time.Hour), model.BucketHour)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	listed := make([]monitorListItem, 0, len(items))
	for _, item := range items {
		row := monitorListItem{Monitor: item}
		if stat, ok := avail[item.ID]; ok && stat.Total > 0 {
			row.HasAvailability = true
			row.Availability24h = float64(stat.Success) / float64(stat.Total) * 100
		}
		listed = append(listed, row)
	}
	ok(c, gin.H{"items": listed, "total": total})
}

type monitorListItem struct {
	model.Monitor
	Availability24h float64 `json:"availability24h"`
	HasAvailability bool    `json:"hasAvailability"`
}

func (s *Server) getMonitor(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var m model.Monitor
	if err := s.db.Preload("Group").Preload("Notifiers").First(&m, id).Error; err != nil {
		fail(c, http.StatusNotFound, "监控不存在")
		return
	}
	ok(c, m)
}

func (s *Server) createMonitor(c *gin.Context) {
	var req monitor.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	snap := s.settings.Snapshot()
	if err := req.Normalize(snap.DefaultInterval, snap.DefaultTimeout); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.ensureGroup(req.GroupID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var m model.Monitor
	req.Apply(&m)
	monitor.TouchNew(&m)
	notifiers, err := s.loadNotifiers(req.NotifierIDs)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	m.Notifiers = notifiers
	if err := s.db.Create(&m).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建失败")
		return
	}
	_ = s.db.Preload("Group").Preload("Notifiers").First(&m, m.ID)
	ok(c, m)
}

func (s *Server) updateMonitor(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var m model.Monitor
	if err := s.db.First(&m, id).Error; err != nil {
		fail(c, http.StatusNotFound, "监控不存在")
		return
	}
	var req monitor.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	snap := s.settings.Snapshot()
	if err := req.Normalize(snap.DefaultInterval, snap.DefaultTimeout); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.ensureGroup(req.GroupID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	wasEnabled := m.Enabled
	req.Apply(&m)
	if wasEnabled && !m.Enabled {
		m.Status = model.StatusPaused
	}
	if !wasEnabled && m.Enabled {
		m.Status = model.StatusUnknown
		m.ConsecutiveFailures = 0
		m.ConsecutiveSuccesses = 0
		now := time.Now()
		m.NextCheckAt = &now
	}
	notifiers, err := s.loadNotifiers(req.NotifierIDs)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.db.Save(&m).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	if err := s.db.Model(&m).Association("Notifiers").Replace(notifiers); err != nil {
		fail(c, http.StatusInternalServerError, "保存通知绑定失败")
		return
	}
	_ = s.db.Preload("Group").Preload("Notifiers").First(&m, m.ID)
	ok(c, m)
}

func (s *Server) deleteMonitor(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var m model.Monitor
	if err := s.db.First(&m, id).Error; err != nil {
		fail(c, http.StatusNotFound, "监控不存在")
		return
	}
	_ = s.db.Model(&m).Association("Notifiers").Clear()
	_ = s.db.Where("monitor_id = ?", id).Delete(&model.MonitorCheck{})
	_ = s.db.Where("monitor_id = ?", id).Delete(&model.MonitorMetric{})
	if err := s.db.Delete(&m).Error; err != nil {
		fail(c, http.StatusInternalServerError, "删除失败")
		return
	}
	ok(c, gin.H{"ok": true})
}

func (s *Server) enableMonitor(c *gin.Context) {
	s.setEnabled(c, true)
}

func (s *Server) disableMonitor(c *gin.Context) {
	s.setEnabled(c, false)
}

func (s *Server) setEnabled(c *gin.Context, enabled bool) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var m model.Monitor
	if err := s.db.First(&m, id).Error; err != nil {
		fail(c, http.StatusNotFound, "监控不存在")
		return
	}
	now := time.Now()
	updates := map[string]any{"enabled": enabled, "updated_at": now}
	if enabled {
		updates["status"] = model.StatusUnknown
		updates["consecutive_failures"] = 0
		updates["consecutive_successes"] = 0
		updates["next_check_at"] = now
	} else {
		updates["status"] = model.StatusPaused
	}
	if err := s.db.Model(&m).Updates(updates).Error; err != nil {
		fail(c, http.StatusInternalServerError, "更新失败")
		return
	}
	_ = s.db.Preload("Group").Preload("Notifiers").First(&m, id)
	ok(c, m)
}

func (s *Server) checkMonitor(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	if !s.pool.TryAcquire(id) {
		fail(c, http.StatusConflict, "该监控正在检测中")
		return
	}
	defer s.pool.Release(id)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	out, err := s.processor.Process(ctx, id, true)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	events := out.Events
	if events == nil {
		events = []model.AlertEvent{}
	}
	ok(c, gin.H{"monitor": out.Monitor, "result": out.Result, "events": events})
}

func (s *Server) listChecks(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	page, size := pageParams(c)
	q := s.db.Model(&model.MonitorCheck{}).Where("monitor_id = ?", id)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	var items []model.MonitorCheck
	if err := q.Order("checked_at desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, gin.H{"items": items, "total": total})
}

func (s *Server) monitorMetrics(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	from, bucket := metricRange(c.Query("range"))
	sum, err := metric.Query(s.db, id, from, bucket)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, sum)
}

func (s *Server) monitorEvents(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	page, size := pageParams(c)
	q := s.db.Model(&model.AlertEvent{}).Where("monitor_id = ?", id)
	var total int64
	_ = q.Count(&total)
	var items []model.AlertEvent
	if err := q.Order("occurred_at desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, gin.H{"items": items, "total": total})
}

func metricRange(name string) (time.Time, string) {
	now := time.Now()
	switch name {
	case "1h":
		return now.Add(-time.Hour), model.BucketMinute
	case "7d":
		return now.AddDate(0, 0, -7), model.BucketHour
	case "30d":
		return now.AddDate(0, 0, -30), model.BucketDay
	default:
		return now.Add(-24 * time.Hour), model.BucketHour
	}
}

func (s *Server) ensureGroup(id *uint) error {
	if id == nil || *id == 0 {
		return nil
	}
	var n int64
	if err := s.db.Model(&model.Group{}).Where("id = ?", *id).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return errGroupMissing
	}
	return nil
}

func (s *Server) loadNotifiers(ids []uint) ([]model.Notifier, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []model.Notifier
	if err := s.db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(unique(ids)) {
		return nil, errNotifierMissing
	}
	return rows, nil
}

func unique(ids []uint) []uint {
	seen := map[uint]struct{}{}
	var out []uint
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
