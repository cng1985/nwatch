package group

import (
	"context"
	"strings"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
)

// Resource 编排分组业务规则。
type Resource struct {
	repo *Repository
}

func NewResource(repo *Repository) *Resource {
	return &Resource{repo: repo}
}

// List 列出全部分组。
func (r *Resource) List(ctx context.Context) (response.Response[GroupListResponse], error) {
	return r.repo.List(ctx)
}

// Create 创建分组。
func (r *Resource) Create(ctx context.Context, request *GroupRequest) (response.Response[GroupResponse], error) {
	if err := validate(request); err != nil {
		return response.Response[GroupResponse]{}, err
	}
	return r.repo.Create(ctx, request)
}

// Update 修改分组。
func (r *Resource) Update(ctx context.Context, request *GroupRequest) (response.Response[GroupResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[GroupResponse]{}, errors.BadRequest("无效的分组编号")
	}
	if err := validate(request); err != nil {
		return response.Response[GroupResponse]{}, err
	}
	return r.repo.Update(ctx, request)
}

// Delete 删除分组。
func (r *Resource) Delete(ctx context.Context, request *GroupRequest) (response.Response[GroupOKResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[GroupOKResponse]{}, errors.BadRequest("无效的分组编号")
	}
	return r.repo.Delete(ctx, request.ID)
}

func validate(request *GroupRequest) error {
	if request == nil {
		return errors.BadRequest("请求格式不正确")
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		return errors.BadRequest("名称不能为空")
	}
	request.Description = strings.TrimSpace(request.Description)
	return nil
}
