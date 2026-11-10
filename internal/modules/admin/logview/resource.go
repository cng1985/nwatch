// Package logview 提供进程运行日志的查询业务。
package logview

import (
	"context"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/logging"
	"github.com/cng1985/nwatch/internal/platform/response"
	"gorm.io/gorm"
)

// LogSearch 是运行日志分页查询参数。
type LogSearch struct {
	Level    string `form:"level"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
}

// LogSimple 是运行日志列表行。
type LogSimple = logging.AppLog

// Resource 编排运行日志查询。
type Resource struct {
	db *gorm.DB
}

func NewResource(db *gorm.DB) *Resource {
	return &Resource{db: db}
}

// Search 分页查询运行日志。
func (r *Resource) Search(ctx context.Context, search *LogSearch) (response.PageResponse[LogSimple], error) {
	if search == nil {
		return response.PageResponse[LogSimple]{}, errors.BadRequest("请求格式不正确")
	}
	if search.Page < 1 {
		search.Page = 1
	}
	if search.PageSize < 1 {
		search.PageSize = 20
	}
	if search.PageSize > 200 {
		return response.PageResponse[LogSimple]{}, errors.BadRequest("每页数量不能超过 200")
	}
	page, err := logging.QueryLogs(r.db.WithContext(ctx), logging.LogQuery{
		Level:    search.Level,
		Keyword:  search.Keyword,
		Page:     search.Page,
		PageSize: search.PageSize,
	})
	if err != nil {
		return response.PageResponse[LogSimple]{}, err
	}
	return response.Page(page.Items, page.Total, search.Page, search.PageSize), nil
}
