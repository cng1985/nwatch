package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/cronjob"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) listCronJobs(c *gin.Context) {
	page, size := pageParams(c)
	q := s.db.Model(&model.CronJob{})
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR command LIKE ? OR cron LIKE ? OR url LIKE ?", like, like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	var items []model.CronJob
	if err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	for i := range items {
		cronjob.Decorate(&items[i])
	}
	ok(c, gin.H{"items": items, "total": total})
}

func (s *Server) getCronJob(c *gin.Context) {
	job, okID := s.loadCronJob(c)
	if !okID {
		return
	}
	ok(c, job)
}

func (s *Server) createCronJob(c *gin.Context) {
	job, req, okReq := s.bindCronJob(c, &model.CronJob{})
	if !okReq {
		return
	}
	if err := req.Apply(job, s.cron.Now()); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.db.Create(job).Error; err != nil {
		if isUnique(err) {
			fail(c, http.StatusBadRequest, "名称已存在")
			return
		}
		fail(c, http.StatusInternalServerError, "创建失败")
		return
	}
	cronjob.Decorate(job)
	ok(c, job)
}

func (s *Server) updateCronJob(c *gin.Context) {
	job, okID := s.loadCronJob(c)
	if !okID {
		return
	}
	current, req, okReq := s.bindCronJob(c, job)
	if !okReq {
		return
	}
	if err := req.Apply(current, s.cron.Now()); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.db.Save(current).Error; err != nil {
		if isUnique(err) {
			fail(c, http.StatusBadRequest, "名称已存在")
			return
		}
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	cronjob.Decorate(current)
	ok(c, current)
}

func (s *Server) deleteCronJob(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var job model.CronJob
	if err := s.db.First(&job, id).Error; err != nil {
		fail(c, http.StatusNotFound, "定时任务不存在")
		return
	}
	if s.cron.Busy(id) {
		fail(c, http.StatusConflict, "任务正在执行，请稍后再删除")
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("job_id = ?", id).Delete(&model.CronRun{}).Error; err != nil {
			return err
		}
		return tx.Delete(&job).Error
	}); err != nil {
		fail(c, http.StatusInternalServerError, "删除失败")
		return
	}
	ok(c, gin.H{"ok": true})
}

func (s *Server) enableCronJob(c *gin.Context)  { s.setCronEnabled(c, true) }
func (s *Server) disableCronJob(c *gin.Context) { s.setCronEnabled(c, false) }

func (s *Server) setCronEnabled(c *gin.Context, enabled bool) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	job, err := s.cron.SetEnabled(id, enabled)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		fail(c, http.StatusNotFound, "定时任务不存在")
		return
	}
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, job)
}

func (s *Server) runCronJob(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	row, err := s.cron.Run(id, model.CronTriggerManual)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		fail(c, http.StatusNotFound, "定时任务不存在")
		return
	}
	if errors.Is(err, cronjob.ErrBusy) {
		fail(c, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "执行失败")
		return
	}
	ok(c, row)
}

func (s *Server) listCronRuns(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var job model.CronJob
	if err := s.db.First(&job, id).Error; err != nil {
		fail(c, http.StatusNotFound, "定时任务不存在")
		return
	}
	page, size := pageParams(c)
	q := s.db.Model(&model.CronRun{}).Where("job_id = ?", id)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	var items []model.CronRun
	if err := q.Order("started_at desc, id desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	ok(c, gin.H{"items": items, "total": total})
}

func (s *Server) previewCron(c *gin.Context) {
	expr := strings.TrimSpace(c.Query("expr"))
	count := atoi(c.Query("count"), 5)
	loc := s.settings.Location()
	view, err := cronjob.BuildPreview(expr, loc.String(), time.Now().In(loc), count)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, view)
}

func (s *Server) loadCronJob(c *gin.Context) (*model.CronJob, bool) {
	id, okID := parseID(c)
	if !okID {
		return nil, false
	}
	var job model.CronJob
	if err := s.db.First(&job, id).Error; err != nil {
		fail(c, http.StatusNotFound, "定时任务不存在")
		return nil, false
	}
	cronjob.Decorate(&job)
	return &job, true
}

func (s *Server) bindCronJob(c *gin.Context, job *model.CronJob) (*model.CronJob, *cronjob.Request, bool) {
	var req cronjob.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return nil, nil, false
	}
	if err := req.Normalize(); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return nil, nil, false
	}
	return job, &req, true
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}
