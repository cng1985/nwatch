package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/dashboard"
	"github.com/cng1985/nwatch/internal/maintenance"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/settings"
	"github.com/cng1985/nwatch/internal/transfer"
	"github.com/cng1985/nwatch/internal/version"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) health(c *gin.Context) {
	dbStatus := "UP"
	sqlDB, err := s.db.DB()
	if err != nil || sqlDB.Ping() != nil {
		dbStatus = "DOWN"
	}
	sched := "DOWN"
	if s.sched.Running() {
		sched = "UP"
	}
	pool := "UP"
	status := "UP"
	if dbStatus != "UP" {
		status = "DOWN"
	}
	code := http.StatusOK
	if status != "UP" {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, gin.H{
		"status":     status,
		"database":   dbStatus,
		"scheduler":  sched,
		"workerPool": pool,
		"queue":      s.pool.QueueLen(),
		"workers":    s.pool.Workers(),
		"version":    version.Version,
	})
}

func (s *Server) dashboard(c *gin.Context) {
	sum, err := dashboard.Query(s.db, c.DefaultQuery("range", "24h"))
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, sum)
}

func (s *Server) listGroups(c *gin.Context) {
	var rows []model.Group
	if err := s.db.Order("sort asc, id asc").Find(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	type item struct {
		model.Group
		MonitorCount int64 `json:"monitorCount"`
	}
	out := make([]item, 0, len(rows))
	for _, g := range rows {
		var n int64
		_ = s.db.Model(&model.Monitor{}).Where("group_id = ?", g.ID).Count(&n)
		out = append(out, item{Group: g, MonitorCount: n})
	}
	ok(c, out)
}

func (s *Server) createGroup(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Sort        int    `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		fail(c, http.StatusBadRequest, "名称不能为空")
		return
	}
	g := model.Group{Name: req.Name, Description: strings.TrimSpace(req.Description), Sort: req.Sort}
	if err := s.db.Create(&g).Error; err != nil {
		fail(c, http.StatusBadRequest, "分组名称已存在")
		return
	}
	ok(c, g)
}

func (s *Server) updateGroup(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var g model.Group
	if err := s.db.First(&g, id).Error; err != nil {
		fail(c, http.StatusNotFound, "分组不存在")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Sort        int    `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		fail(c, http.StatusBadRequest, "名称不能为空")
		return
	}
	g.Name = req.Name
	g.Description = strings.TrimSpace(req.Description)
	g.Sort = req.Sort
	if err := s.db.Save(&g).Error; err != nil {
		fail(c, http.StatusBadRequest, "保存失败")
		return
	}
	ok(c, g)
}

func (s *Server) deleteGroup(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	if err := s.db.Model(&model.Monitor{}).Where("group_id = ?", id).Update("group_id", gorm.Expr("NULL")).Error; err != nil {
		fail(c, http.StatusInternalServerError, "解除关联失败")
		return
	}
	if err := s.db.Delete(&model.Group{}, id).Error; err != nil {
		fail(c, http.StatusInternalServerError, "删除失败")
		return
	}
	ok(c, gin.H{"ok": true})
}

func (s *Server) listEvents(c *gin.Context) {
	page, size := pageParams(c)
	q := s.db.Model(&model.AlertEvent{})
	if v := c.Query("monitorId"); v != "" {
		q = q.Where("monitor_id = ?", v)
	}
	if v := c.Query("eventType"); v != "" {
		q = q.Where("event_type = ?", v)
	}
	var total int64
	_ = q.Count(&total)
	var items []model.AlertEvent
	if err := q.Order("occurred_at desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, gin.H{"items": items, "total": total})
}

func (s *Server) listCertificates(c *gin.Context) {
	var items []model.Monitor
	if err := s.db.Where("type = ?", model.TypeTLS).
		Order("CASE WHEN cert_days_remaining IS NULL THEN 1 ELSE 0 END, cert_days_remaining ASC, id ASC").
		Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, items)
}

func (s *Server) listNotifyLogs(c *gin.Context) {
	page, size := pageParams(c)
	q := s.db.Model(&model.NotificationLog{})
	if v := c.Query("notifierId"); v != "" {
		q = q.Where("notifier_id = ?", v)
	}
	if v := c.Query("success"); v == "true" || v == "false" {
		q = q.Where("success = ?", v == "true")
	}
	var total int64
	_ = q.Count(&total)
	var items []model.NotificationLog
	if err := q.Order("sent_at desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, gin.H{"items": items, "total": total})
}

func (s *Server) getSettings(c *gin.Context) {
	snap := s.settings.Snapshot()
	ok(c, gin.H{
		"timezone":                snap.Timezone,
		"checkRetentionDays":      snap.CheckRetentionDays,
		"eventRetentionDays":      snap.EventRetentionDays,
		"metricRetentionDays":     snap.MetricRetentionDays,
		"hourMetricRetentionDays": snap.HourMetricRetentionDays,
		"defaultTimeout":          snap.DefaultTimeout,
		"defaultInterval":         snap.DefaultInterval,
		"workerCount":             s.cfg.Scheduler.WorkerCount,
		"scanInterval":            s.cfg.Scheduler.ScanInterval.String(),
		"usingDefaultPassword":    s.settings.UsingDefaultPassword(),
		"version":                 version.Version,
		"database":                s.cfg.Database.Path,
		"mail":                    mailView(s.settings.SMTP()),
	})
}

func (s *Server) updateSettings(c *gin.Context) {
	var req struct {
		Timezone                string `json:"timezone"`
		CheckRetentionDays      int    `json:"checkRetentionDays"`
		EventRetentionDays      int    `json:"eventRetentionDays"`
		MetricRetentionDays     int    `json:"metricRetentionDays"`
		HourMetricRetentionDays int    `json:"hourMetricRetentionDays"`
		DefaultTimeout          int    `json:"defaultTimeout"`
		DefaultInterval         int    `json:"defaultInterval"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil {
		fail(c, http.StatusBadRequest, "时区无效")
		return
	}
	if req.CheckRetentionDays < 1 || req.EventRetentionDays < 1 || req.MetricRetentionDays < 1 || req.HourMetricRetentionDays < 1 {
		fail(c, http.StatusBadRequest, "保留天数至少为 1")
		return
	}
	if req.DefaultTimeout < 1 || req.DefaultTimeout > 120 || req.DefaultInterval < 5 {
		fail(c, http.StatusBadRequest, "默认超时或频率不合法")
		return
	}
	err := s.settings.Update(map[string]string{
		settings.KeyTimezone:            req.Timezone,
		settings.KeyCheckRetention:      strconv.Itoa(req.CheckRetentionDays),
		settings.KeyEventRetention:      strconv.Itoa(req.EventRetentionDays),
		settings.KeyMetricRetention:     strconv.Itoa(req.MetricRetentionDays),
		settings.KeyHourMetricRetention: strconv.Itoa(req.HourMetricRetentionDays),
		settings.KeyDefaultTimeout:      strconv.Itoa(req.DefaultTimeout),
		settings.KeyDefaultInterval:     strconv.Itoa(req.DefaultInterval),
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	s.getSettings(c)
}

func (s *Server) downloadBackup(c *gin.Context) {
	dest := filepath.Join(os.TempDir(), "nmonitor-backup-"+time.Now().Format("20060102-150405")+".db")
	if err := maintenance.BackupTo(s.db, dest); err != nil {
		fail(c, http.StatusInternalServerError, "备份失败")
		return
	}
	defer os.Remove(dest)
	c.FileAttachment(dest, filepath.Base(dest))
}

func (s *Server) createBackup(c *gin.Context) {
	if err := os.MkdirAll(s.cfg.Backup.Dir, 0o755); err != nil {
		fail(c, http.StatusInternalServerError, "创建备份目录失败")
		return
	}
	name := "nmonitor-manual-" + time.Now().In(s.settings.Location()).Format("20060102-150405") + ".db"
	dest := filepath.Join(s.cfg.Backup.Dir, name)
	if err := maintenance.BackupTo(s.db, dest); err != nil {
		fail(c, http.StatusInternalServerError, "备份失败")
		return
	}
	ok(c, gin.H{"file": dest})
}

func (s *Server) exportConfig(c *gin.Context) {
	include := c.Query("includeSecrets") == "true"
	bundle, err := transfer.Export(s.db, include)
	if err != nil {
		fail(c, http.StatusInternalServerError, "导出失败")
		return
	}
	ok(c, bundle)
}

func (s *Server) importConfig(c *gin.Context) {
	var bundle transfer.Bundle
	if err := c.ShouldBindJSON(&bundle); err != nil {
		fail(c, http.StatusBadRequest, "导入内容不正确")
		return
	}
	if bundle.Version != 1 {
		fail(c, http.StatusBadRequest, "不支持的配置版本")
		return
	}
	if err := transfer.Import(s.db, bundle); err != nil {
		fail(c, http.StatusBadRequest, "导入失败："+err.Error())
		return
	}
	ok(c, gin.H{"ok": true})
}
