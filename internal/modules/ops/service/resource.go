// Package service 通过本机 systemctl 管理系统服务。
package service

import (
	"context"
	stderrors "errors"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
)

// ServiceListRequest 是系统服务列表查询参数。
type ServiceListRequest struct {
	State string `form:"state"`
	Query string `form:"q"`
}

// ServiceListResponse 是系统服务列表结果。
type ServiceListResponse = ListResult

// ServiceActionRequest 是服务操作参数。
type ServiceActionRequest struct {
	Name   string `json:"name"`
	Action string `json:"action"`
}

// ServiceActionResponse 是服务操作结果。
type ServiceActionResponse struct {
	OK     bool   `json:"ok"`
	Name   string `json:"name"`
	Action string `json:"action"`
}

// ServiceLogsRequest 是服务日志查询参数。
type ServiceLogsRequest struct {
	Name  string `form:"name"`
	Lines int    `form:"lines"`
}

// ServiceLogsResponse 是服务日志结果。
type ServiceLogsResponse = Logs

// Resource 编排系统服务业务。
type Resource struct {
	manager *Manager
}

func NewResource(manager *Manager) *Resource {
	return &Resource{manager: manager}
}

// List 查询系统服务列表。状态筛选白名单校验。
func (r *Resource) List(ctx context.Context, request *ServiceListRequest) (response.Response[ServiceListResponse], error) {
	if request == nil {
		return response.Response[ServiceListResponse]{}, errors.BadRequest("请求格式不正确")
	}
	switch strings.ToLower(strings.TrimSpace(request.State)) {
	case "", "all", "active", "inactive", "failed":
	default:
		return response.Response[ServiceListResponse]{}, errors.BadRequest("状态筛选不合法")
	}
	listCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	res, err := r.manager.List(listCtx, request.Query, strings.ToLower(strings.TrimSpace(request.State)))
	if err != nil {
		return response.Response[ServiceListResponse]{}, serviceError(err)
	}
	return response.OK(res), nil
}

// Action 执行启动、停止或重启。
func (r *Resource) Action(ctx context.Context, request *ServiceActionRequest) (response.Response[ServiceActionResponse], error) {
	if request == nil || request.Name == "" || request.Action == "" {
		return response.Response[ServiceActionResponse]{}, errors.BadRequest("无效的服务操作")
	}
	actionCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	unit, err := r.manager.Action(actionCtx, request.Name, request.Action)
	if err != nil {
		return response.Response[ServiceActionResponse]{}, serviceError(err)
	}
	return response.OK(ServiceActionResponse{OK: true, Name: unit, Action: request.Action}), nil
}

// Logs 查询服务日志。
func (r *Resource) Logs(ctx context.Context, request *ServiceLogsRequest) (response.Response[ServiceLogsResponse], error) {
	if request == nil || request.Name == "" {
		return response.Response[ServiceLogsResponse]{}, errors.BadRequest("无效的服务名")
	}
	lines := request.Lines
	if lines <= 0 {
		lines = 200
	}
	logsCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	logs, err := r.manager.Logs(logsCtx, request.Name, lines)
	if err != nil {
		return response.Response[ServiceLogsResponse]{}, serviceError(err)
	}
	return response.OK(logs), nil
}

func serviceError(err error) error {
	switch {
	case stderrors.Is(err, ErrDisabled):
		return errors.Forbidden(err.Error())
	case stderrors.Is(err, ErrUnavailable), stderrors.Is(err, ErrNoJournal):
		return errors.Unavailable(err.Error())
	case stderrors.Is(err, ErrBadName), stderrors.Is(err, ErrProtected):
		return errors.BadRequest(err.Error())
	default:
		return errors.Unavailable(err.Error())
	}
}
