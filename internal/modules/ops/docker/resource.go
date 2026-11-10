// Package docker 通过本机 docker 客户端管理容器和镜像。
package docker

import (
	"context"
	stderrors "errors"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
)

// DockerContainerSearch 是容器列表查询参数。
type DockerContainerSearch struct {
	State string `form:"state"`
	Query string `form:"q"`
}

// DockerContainerListResponse 是容器列表结果。
type DockerContainerListResponse = ContainerList

// DockerImageSearch 是镜像列表查询参数。
type DockerImageSearch struct {
	Query string `form:"q"`
}

// DockerImageListResponse 是镜像列表结果。
type DockerImageListResponse = ImageList

// DockerActionRequest 是容器操作参数。
type DockerActionRequest struct {
	Name   string `json:"name"`
	Action string `json:"action"`
}

// DockerActionResponse 是容器操作结果。
type DockerActionResponse struct {
	OK     bool   `json:"ok"`
	Name   string `json:"name"`
	Action string `json:"action"`
}

// DockerLogsRequest 是容器日志查询参数。
type DockerLogsRequest struct {
	Name  string `form:"name"`
	Lines int    `form:"lines"`
}

// DockerLogsResponse 是容器日志结果。
type DockerLogsResponse = Logs

// Resource 编排 Docker 管理业务。
type Resource struct {
	manager *Manager
}

func NewResource(manager *Manager) *Resource {
	return &Resource{manager: manager}
}

// Containers 查询容器列表。状态筛选白名单校验。
func (r *Resource) Containers(ctx context.Context, search *DockerContainerSearch) (response.Response[DockerContainerListResponse], error) {
	if search == nil {
		return response.Response[DockerContainerListResponse]{}, errors.BadRequest("请求格式不正确")
	}
	switch strings.ToLower(strings.TrimSpace(search.State)) {
	case "", "all", "running", "exited", "paused", "created", "restarting", "dead", "removing":
	default:
		return response.Response[DockerContainerListResponse]{}, errors.BadRequest("状态筛选不合法")
	}
	listCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	res, err := r.manager.Containers(listCtx, search.Query, strings.ToLower(strings.TrimSpace(search.State)))
	if err != nil {
		return response.Response[DockerContainerListResponse]{}, dockerError(err)
	}
	return response.OK(res), nil
}

// Images 查询镜像列表。
func (r *Resource) Images(ctx context.Context, search *DockerImageSearch) (response.Response[DockerImageListResponse], error) {
	if search == nil {
		return response.Response[DockerImageListResponse]{}, errors.BadRequest("请求格式不正确")
	}
	listCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	res, err := r.manager.Images(listCtx, search.Query)
	if err != nil {
		return response.Response[DockerImageListResponse]{}, dockerError(err)
	}
	return response.OK(res), nil
}

// Action 执行容器启动、停止或重启。
func (r *Resource) Action(ctx context.Context, request *DockerActionRequest) (response.Response[DockerActionResponse], error) {
	if request == nil || request.Name == "" || request.Action == "" {
		return response.Response[DockerActionResponse]{}, errors.BadRequest("无效的容器操作")
	}
	actionCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	name, err := r.manager.Action(actionCtx, request.Name, request.Action)
	if err != nil {
		return response.Response[DockerActionResponse]{}, dockerError(err)
	}
	return response.OK(DockerActionResponse{OK: true, Name: name, Action: request.Action}), nil
}

// Logs 查询容器日志。
func (r *Resource) Logs(ctx context.Context, request *DockerLogsRequest) (response.Response[DockerLogsResponse], error) {
	if request == nil || request.Name == "" {
		return response.Response[DockerLogsResponse]{}, errors.BadRequest("无效的容器名")
	}
	lines := request.Lines
	if lines <= 0 {
		lines = 200
	}
	logsCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	logs, err := r.manager.Logs(logsCtx, request.Name, lines)
	if err != nil {
		return response.Response[DockerLogsResponse]{}, dockerError(err)
	}
	return response.OK(logs), nil
}

func dockerError(err error) error {
	switch {
	case stderrors.Is(err, ErrDisabled):
		return errors.Forbidden(err.Error())
	case stderrors.Is(err, ErrUnavailable):
		return errors.Unavailable(err.Error())
	case stderrors.Is(err, ErrBadName):
		return errors.BadRequest(err.Error())
	default:
		return errors.Unavailable(err.Error())
	}
}
