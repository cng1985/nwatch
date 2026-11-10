package dashboard

import (
	"context"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"gorm.io/gorm"
)

// DashboardRequest 是总览查询参数。
type DashboardRequest struct {
	Range string `form:"range"`
}

// DashboardResponse 是总览业务结果。结构由查询引擎定义并作为发布契约。
type DashboardResponse = Summary

// Resource 编排总览查询。
type Resource struct {
	db *gorm.DB
}

func NewResource(db *gorm.DB) *Resource {
	return &Resource{db: db}
}

// Summary 返回总览数据。
func (r *Resource) Summary(ctx context.Context, request *DashboardRequest) (response.Response[DashboardResponse], error) {
	if request == nil {
		return response.Response[DashboardResponse]{}, errors.BadRequest("请求格式不正确")
	}
	name := request.Range
	if name == "" {
		name = "24h"
	}
	sum, err := Query(r.db.WithContext(ctx), name)
	if err != nil {
		return response.Response[DashboardResponse]{}, err
	}
	return response.OK(sum), nil
}
