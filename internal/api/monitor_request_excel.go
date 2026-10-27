package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/transfer"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) requestQuery(c *gin.Context) *gorm.DB {
	q := s.db.Model(&model.Monitor{}).Where("type = ?", model.TypeHTTP)
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR url LIKE ?", like, like)
	}
	if st := strings.TrimSpace(c.Query("status")); st != "" {
		q = q.Where("status = ?", st)
	}
	if gid := strings.TrimSpace(c.Query("groupId")); gid != "" {
		q = q.Where("group_id = ?", gid)
	}
	return q
}

func (s *Server) exportRequests(c *gin.Context) {
	loc := s.settings.Location()
	filename := "nmonitor-requests-template.xlsx"
	var monitors []model.Monitor
	if c.Query("template") != "1" {
		var total int64
		if err := s.requestQuery(c).Count(&total).Error; err != nil {
			fail(c, http.StatusInternalServerError, "导出失败")
			return
		}
		if total > 10000 {
			fail(c, http.StatusBadRequest, "一次最多导出 10000 条，请先筛选")
			return
		}
		if err := s.requestQuery(c).Preload("Group").Order("id").Find(&monitors).Error; err != nil {
			fail(c, http.StatusInternalServerError, "导出失败")
			return
		}
		filename = "nmonitor-requests-" + time.Now().In(loc).Format("20060102-150405") + ".xlsx"
	}
	data, err := transfer.RequestsToXLSX(monitors)
	if err != nil {
		slog.Error("导出网络请求表格失败", "err", err.Error())
		fail(c, http.StatusInternalServerError, "导出失败")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

func (s *Server) importRequests(c *gin.Context) {
	data, code, msg := readExcelUpload(c)
	if code != 0 {
		fail(c, code, msg)
		return
	}
	snap := s.settings.Snapshot()
	stats, err := transfer.ImportRequestsXLSX(s.db, bytes.NewReader(data), snap.DefaultInterval, snap.DefaultTimeout)
	if err != nil {
		if msg, ok := excelClientError(err); ok {
			fail(c, http.StatusBadRequest, msg)
			return
		}
		slog.Error("导入网络请求表格失败", "err", err.Error())
		fail(c, http.StatusInternalServerError, "导入失败")
		return
	}
	ok(c, stats)
}
