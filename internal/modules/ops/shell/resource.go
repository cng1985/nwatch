package shell

import (
	"context"
	stderrors "errors"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
)

// Resource 编排终端业务：票据签发与命令审计查询。
type Resource struct {
	repo *Repository
	hub  *Hub
}

func NewResource(repo *Repository, hub *Hub) *Resource {
	return &Resource{repo: repo, hub: hub}
}

// Info 返回终端能力信息。
func (r *Resource) Info(ctx context.Context) (response.Response[ShellInfoResponse], error) {
	return response.OK(r.hub.Info()), nil
}

// IssueTicket 为当前用户签发连接票据。
func (r *Resource) IssueTicket(ctx context.Context, username string) (response.Response[ShellTicketResponse], error) {
	token, err := r.hub.Issue(username)
	if err != nil {
		return response.Response[ShellTicketResponse]{}, shellError(err)
	}
	return response.OK(ShellTicketResponse{Ticket: token, ExpiresIn: 30}), nil
}

// SearchCommands 分页查询命令审计。
func (r *Resource) SearchCommands(ctx context.Context, search *ShellCommandSearch) (response.PageResponse[ShellCommandSimple], error) {
	if search == nil {
		return response.PageResponse[ShellCommandSimple]{}, errors.BadRequest("请求格式不正确")
	}
	if search.Page < 1 {
		search.Page = 1
	}
	if search.PageSize < 1 {
		search.PageSize = 20
	}
	if search.PageSize > 200 {
		return response.PageResponse[ShellCommandSimple]{}, errors.BadRequest("每页数量不能超过 200")
	}
	return r.repo.Search(ctx, search)
}

// shellError 把终端错误映射为平台错误分类。
func shellError(err error) error {
	switch {
	case stderrors.Is(err, ErrDisabled):
		return errors.Forbidden(err.Error())
	case stderrors.Is(err, ErrNoShell), stderrors.Is(err, ErrUnsupported):
		return errors.Unavailable(err.Error())
	case stderrors.Is(err, ErrBusy), stderrors.Is(err, ErrTooManyTickets):
		return errors.RateLimited(err.Error())
	case stderrors.Is(err, ErrTicket):
		return errors.Unauthorized(err.Error())
	default:
		return errors.BadRequest(err.Error())
	}
}
