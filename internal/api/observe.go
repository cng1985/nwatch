package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/checker"
	"github.com/cng1985/nwatch/internal/host"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/gin-gonic/gin"
)

func (s *Server) hostStatus(c *gin.Context) {
	snap, err := s.collector.Snapshot()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	from, name := host.Since(c.Query("range"))
	history, err := host.LoadHistory(s.db, from)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	if len(history) == 0 {
		history = []host.Point{{
			Time:        snap.SampledAt,
			CPUPercent:  snap.CPUPercent,
			MemPercent:  snap.MemPercent,
			DiskPercent: host.RootPercent(snap.Disks),
		}}
	}
	ok(c, gin.H{"current": snap, "history": history, "range": name})
}

type scriptRunRequest struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	WorkDir string `json:"workDir"`
	Timeout int    `json:"timeout"`
}

func (s *Server) runScript(c *gin.Context) {
	var req scriptRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	timeout, err := checker.ValidateScript(req.Command, req.WorkDir, req.Timeout, true)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout+time.Second)
	defer cancel()
	out := checker.Execute(ctx, req.Command, req.WorkDir, timeout)
	row := out.Model(strings.TrimSpace(req.Name), strings.TrimSpace(req.Command), strings.TrimSpace(req.WorkDir), nil)
	if err := s.db.Create(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存执行记录失败")
		return
	}
	checker.PruneScriptRuns(s.db)
	ok(c, row)
}

func (s *Server) listScriptRuns(c *gin.Context) {
	page, size := pageParams(c)
	q := s.db.Model(&model.ScriptRun{})
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR command LIKE ?", like, like)
	}
	if id := c.Query("monitorId"); id != "" {
		q = q.Where("monitor_id = ?", id)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	var items []model.ScriptRun
	if err := q.Order("started_at desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, gin.H{"items": items, "total": total})
}

func (s *Server) getScriptRun(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var row model.ScriptRun
	if err := s.db.First(&row, id).Error; err != nil {
		fail(c, http.StatusNotFound, "执行记录不存在")
		return
	}
	ok(c, row)
}

func (s *Server) listLogs(c *gin.Context) {
	page, size := pageParams(c)
	q := s.db.Model(&model.AppLog{})
	if level := strings.ToUpper(strings.TrimSpace(c.Query("level"))); level != "" {
		q = q.Where("level = ?", level)
	}
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("message LIKE ? OR attrs LIKE ?", like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	var items []model.AppLog
	if err := q.Order("logged_at desc, id desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, gin.H{"items": items, "total": total})
}
