// Package kube 通过本机 kubectl 管理 Kubernetes 资源。
package kube

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
)

// KubeNamespaceListResponse 是命名空间列表结果。
type KubeNamespaceListResponse = NamespaceList

// KubeResourceSearch 是资源列表查询参数。
type KubeResourceSearch struct {
	Kind      string `form:"kind"`
	Namespace string `form:"namespace"`
	Query     string `form:"q"`
}

// KubeResourceListResponse 是资源列表结果。
type KubeResourceListResponse = ListResult

// KubeWorkloadRequest 是工作负载操作参数。
type KubeWorkloadRequest struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// KubeWorkloadResponse 是工作负载操作结果。
type KubeWorkloadResponse struct {
	OK        bool   `json:"ok"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Action    string `json:"action"`
}

// KubeScaleRequest 是副本数调整参数。
type KubeScaleRequest struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Replicas  *int   `json:"replicas"`
}

// KubeScaleResponse 是副本数调整结果。
type KubeScaleResponse struct {
	OK        bool   `json:"ok"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Replicas  int    `json:"replicas"`
}

// KubePodRequest 是 Pod 操作参数。
type KubePodRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// KubePodResponse 是 Pod 操作结果。
type KubePodResponse struct {
	OK        bool   `json:"ok"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Action    string `json:"action"`
}

// KubePodLogsRequest 是 Pod 日志查询参数。
type KubePodLogsRequest struct {
	Namespace string `form:"namespace"`
	Name      string `form:"name"`
	Container string `form:"container"`
	Lines     int    `form:"lines"`
}

// KubePodLogsResponse 是 Pod 日志结果。
type KubePodLogsResponse = Logs

// Resource 编排 Kubernetes 管理业务。
type Resource struct {
	manager *Manager
}

func NewResource(manager *Manager) *Resource {
	return &Resource{manager: manager}
}

// Namespaces 查询命名空间。
func (r *Resource) Namespaces(ctx context.Context) (response.Response[KubeNamespaceListResponse], error) {
	listCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	res, err := r.manager.Namespaces(listCtx)
	if err != nil {
		return response.Response[KubeNamespaceListResponse]{}, kubeError(err)
	}
	return response.OK(res), nil
}

// List 查询资源列表。
func (r *Resource) List(ctx context.Context, search *KubeResourceSearch) (response.Response[KubeResourceListResponse], error) {
	if search == nil {
		return response.Response[KubeResourceListResponse]{}, errors.BadRequest("请求格式不正确")
	}
	kind := search.Kind
	if kind == "" {
		kind = "pods"
	}
	listCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	res, err := r.manager.List(listCtx, kind, search.Namespace, search.Query)
	if err != nil {
		return response.Response[KubeResourceListResponse]{}, kubeError(err)
	}
	return response.OK(res), nil
}

// Restart 滚动重启工作负载。
func (r *Resource) Restart(ctx context.Context, request *KubeWorkloadRequest) (response.Response[KubeWorkloadResponse], error) {
	actionCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	ns, name, err := r.manager.Restart(actionCtx, request.Kind, request.Namespace, request.Name)
	if err != nil {
		return response.Response[KubeWorkloadResponse]{}, kubeError(err)
	}
	return response.OK(KubeWorkloadResponse{OK: true, Namespace: ns, Name: name, Action: "restart"}), nil
}

// Scale 调整工作负载副本数。
func (r *Resource) Scale(ctx context.Context, request *KubeScaleRequest) (response.Response[KubeScaleResponse], error) {
	if request == nil || request.Replicas == nil || *request.Replicas < 0 {
		return response.Response[KubeScaleResponse]{}, errors.BadRequest("副本数不合法")
	}
	actionCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	ns, name, err := r.manager.Scale(actionCtx, request.Kind, request.Namespace, request.Name, *request.Replicas)
	if err != nil {
		return response.Response[KubeScaleResponse]{}, kubeError(err)
	}
	return response.OK(KubeScaleResponse{OK: true, Namespace: ns, Name: name, Replicas: *request.Replicas}), nil
}

// DeletePod 删除 Pod。
func (r *Resource) DeletePod(ctx context.Context, request *KubePodRequest) (response.Response[KubePodResponse], error) {
	actionCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	ns, name, err := r.manager.DeletePod(actionCtx, request.Namespace, request.Name)
	if err != nil {
		return response.Response[KubePodResponse]{}, kubeError(err)
	}
	return response.OK(KubePodResponse{OK: true, Namespace: ns, Name: name, Action: "delete"}), nil
}

// PodLogs 查询 Pod 日志。
func (r *Resource) PodLogs(ctx context.Context, request *KubePodLogsRequest) (response.Response[KubePodLogsResponse], error) {
	if request == nil || request.Name == "" {
		return response.Response[KubePodLogsResponse]{}, errors.BadRequest("无效的 Pod 名")
	}
	lines := request.Lines
	if lines <= 0 {
		lines = 200
	}
	logsCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	logs, err := r.manager.Logs(logsCtx, request.Namespace, request.Name, request.Container, lines)
	if err != nil {
		return response.Response[KubePodLogsResponse]{}, kubeError(err)
	}
	return response.OK(logs), nil
}

func kubeError(err error) error {
	switch {
	case stderrors.Is(err, ErrDisabled):
		return errors.Forbidden(err.Error())
	case stderrors.Is(err, ErrUnavailable):
		return errors.Unavailable(err.Error())
	case stderrors.Is(err, ErrBadName), stderrors.Is(err, ErrBadKind), stderrors.Is(err, ErrBadScale), stderrors.Is(err, ErrBadConfig):
		return errors.BadRequest(err.Error())
	default:
		return errors.Unavailable(err.Error())
	}
}
