package api

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/cng1985/nwatch/internal/metric"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/transfer"
	"github.com/gin-gonic/gin"
)

const maxMonitorExcel = 8 << 20

func (s *Server) exportMonitors(c *gin.Context) {
	loc := s.settings.Location()
	filename := "nmonitor-monitors-template.xlsx"
	var monitors []model.Monitor
	availability := map[uint]string{}
	if c.Query("template") != "1" {
		var total int64
		if err := s.monitorScope(c).Count(&total).Error; err != nil {
			fail(c, http.StatusInternalServerError, "导出失败")
			return
		}
		if total > 10000 {
			fail(c, http.StatusBadRequest, "一次最多导出 10000 条，请先筛选")
			return
		}
		if err := s.monitorScope(c).Preload("Group").Preload("Notifiers").Order("id").Find(&monitors).Error; err != nil {
			fail(c, http.StatusInternalServerError, "导出失败")
			return
		}
		avail, err := metric.AvailabilityByMonitor(s.db, time.Now().Add(-24*time.Hour), model.BucketHour)
		if err != nil {
			fail(c, http.StatusInternalServerError, "导出失败")
			return
		}
		for id, stat := range avail {
			if stat.Total > 0 {
				availability[id] = strconv.FormatFloat(float64(stat.Success)/float64(stat.Total)*100, 'f', 2, 64) + "%"
			}
		}
		filename = "nmonitor-monitors-" + time.Now().In(loc).Format("20060102-150405") + ".xlsx"
	}
	data, err := transfer.MonitorsToXLSX(monitors, availability, loc)
	if err != nil {
		slog.Error("导出监控表格失败", "err", err.Error())
		fail(c, http.StatusInternalServerError, "导出失败")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

func (s *Server) importMonitors(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "请选择 xlsx 文件")
		return
	}
	if file.Size > maxMonitorExcel {
		fail(c, http.StatusBadRequest, "文件不能超过 8MB")
		return
	}
	src, err := file.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, "请选择 xlsx 文件")
		return
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, maxMonitorExcel+1))
	if err != nil {
		fail(c, http.StatusBadRequest, "无法读取文件")
		return
	}
	if len(data) > maxMonitorExcel {
		fail(c, http.StatusBadRequest, "文件不能超过 8MB")
		return
	}
	snap := s.settings.Snapshot()
	stats, err := transfer.ImportMonitorsXLSX(s.db, bytes.NewReader(data), snap.DefaultInterval, snap.DefaultTimeout)
	if err != nil {
		if msg, ok := excelClientError(err); ok {
			fail(c, http.StatusBadRequest, msg)
			return
		}
		slog.Error("导入监控表格失败", "err", err.Error())
		fail(c, http.StatusInternalServerError, "导入失败")
		return
	}
	ok(c, stats)
}

func excelClientError(err error) (string, bool) {
	var rows *transfer.RowErrors
	if errors.As(err, &rows) {
		return rows.Error(), true
	}
	var sheet *transfer.SheetError
	if errors.As(err, &sheet) {
		return sheet.Error(), true
	}
	return "", false
}
