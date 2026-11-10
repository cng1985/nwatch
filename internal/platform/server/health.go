package server

import (
	"net/http"
	"time"

	"github.com/cng1985/nwatch/internal/platform/response"
	"github.com/cng1985/nwatch/internal/platform/version"
	"github.com/gin-gonic/gin"
)

// healthLive 基本存活，直接 HTTP 200/503，不被外部依赖拖垮。
func (s *Server) healthLive(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "UP"})
}

// healthReady 必需依赖（数据库）就绪、接受流量，直接 HTTP 200/503。
func (s *Server) healthReady(c *gin.Context) {
	if !s.dbUp() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "DOWN"})
		return
	}
	for _, reporter := range s.reporters {
		if report := reporter.HealthReport(); !report.Up {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "DOWN", "component": report.Name})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "UP"})
}

// healthDetail 保留既有运维视图：数据库、调度器与队列状态。
func (s *Server) healthDetail(c *gin.Context) {
	dbStatus := "UP"
	if !s.dbUp() {
		dbStatus = "DOWN"
	}
	detail := gin.H{
		"status":   "UP",
		"database": dbStatus,
		"version":  version.Version,
		"uptime":   time.Since(s.started).Truncate(time.Second).String(),
	}
	components := gin.H{}
	status := "UP"
	if dbStatus != "UP" {
		status = "DOWN"
	}
	for _, reporter := range s.reporters {
		report := reporter.HealthReport()
		state := "UP"
		if !report.Up {
			state = "DOWN"
			status = "DOWN"
		}
		components[report.Name] = state
	}
	detail["status"] = status
	detail["components"] = components
	code := http.StatusOK
	if status != "UP" {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, detail)
}

func (s *Server) dbUp() bool {
	sqlDB, err := s.db.DB()
	if err != nil {
		return false
	}
	return sqlDB.Ping() == nil
}

// version 输出进程版本与运行时长。
func (s *Server) version(c *gin.Context) {
	response.SuccessResponse(c, response.OK(gin.H{
		"version": version.Version,
		"uptime":  time.Since(s.started).Truncate(time.Second).String(),
	}))
}
