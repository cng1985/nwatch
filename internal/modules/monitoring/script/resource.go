package script

import (
	"context"
	"time"

	"github.com/cng1985/nwatch/internal/modules/monitoring/checker"
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
)

// Resource 编排手动脚本执行：校验、执行、落库与记录清理。
type Resource struct {
	repo *Repository
}

func NewResource(repo *Repository) *Resource {
	return &Resource{repo: repo}
}

// Run 执行一次脚本并保存记录。
func (r *Resource) Run(ctx context.Context, request *ScriptRunRequest) (response.Response[ScriptRunResponse], error) {
	if request == nil {
		return response.Response[ScriptRunResponse]{}, errors.BadRequest("请求格式不正确")
	}
	timeout, err := checker.ValidateScript(request.Command, request.WorkDir, request.Timeout, true)
	if err != nil {
		return response.Response[ScriptRunResponse]{}, errors.BadRequest(err.Error())
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout+time.Second)
	defer cancel()
	out := checker.Execute(runCtx, request.Command, request.WorkDir, timeout)
	row := out.Model(trimSpace(request.Name), trimSpace(request.Command), trimSpace(request.WorkDir), nil)
	if err := r.repo.Create(ctx, &row); err != nil {
		return response.Response[ScriptRunResponse]{}, errors.Internal("保存执行记录失败")
	}
	r.repo.Prune(ctx)
	return response.OK(row), nil
}

// Search 分页查询执行记录。
func (r *Resource) Search(ctx context.Context, search *ScriptRunSearch) (response.PageResponse[ScriptRunSimple], error) {
	if search == nil {
		return response.PageResponse[ScriptRunSimple]{}, errors.BadRequest("请求格式不正确")
	}
	if search.Page < 1 {
		search.Page = 1
	}
	if search.PageSize < 1 {
		search.PageSize = 20
	}
	if search.PageSize > 200 {
		return response.PageResponse[ScriptRunSimple]{}, errors.BadRequest("每页数量不能超过 200")
	}
	return r.repo.Search(ctx, search)
}

// One 查询单条执行记录。
func (r *Resource) One(ctx context.Context, request *ScriptRunOneRequest) (response.Response[ScriptRunResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[ScriptRunResponse]{}, errors.BadRequest("无效的编号")
	}
	return r.repo.One(ctx, request.ID)
}
