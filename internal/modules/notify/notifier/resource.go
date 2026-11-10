package notifier

import (
	"context"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
)

// Resource 编排通知渠道业务：校验、CRUD、测试发送与日志查询。
type Resource struct {
	repo     *Repository
	registry *Registry
}

func NewResource(repo *Repository, registry *Registry) *Resource {
	return &Resource{repo: repo, registry: registry}
}

// List 列出全部通知渠道。
func (r *Resource) List(ctx context.Context) (response.Response[NotifierListResponse], error) {
	return r.repo.List(ctx)
}

// Create 创建通知渠道。
func (r *Resource) Create(ctx context.Context, request *NotifierRequest) (response.Response[NotifierResponse], error) {
	if request == nil {
		return response.Response[NotifierResponse]{}, errors.BadRequest("请求格式不正确")
	}
	n, err := buildNotifier(Notifier{}, request, true)
	if err != nil {
		return response.Response[NotifierResponse]{}, err
	}
	return r.repo.Create(ctx, n)
}

// Update 修改通知渠道。Webhook 与密钥提交打码值时保持原值。
func (r *Resource) Update(ctx context.Context, request *NotifierRequest) (response.Response[NotifierResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[NotifierResponse]{}, errors.BadRequest("无效的编号")
	}
	current, err := r.repo.One(ctx, request.ID)
	if err != nil {
		return response.Response[NotifierResponse]{}, err
	}
	n, err := buildNotifier(*current, request, false)
	if err != nil {
		return response.Response[NotifierResponse]{}, err
	}
	return r.repo.Update(ctx, n)
}

// Delete 删除通知渠道。
func (r *Resource) Delete(ctx context.Context, request *NotifierTestRequest) (response.Response[NotifierOKResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[NotifierOKResponse]{}, errors.BadRequest("无效的编号")
	}
	return r.repo.Delete(ctx, request.ID)
}

// Test 向指定渠道发送一条测试通知并记录日志。
func (r *Resource) Test(ctx context.Context, request *NotifierTestRequest) (response.Response[NotifierTestResponse], error) {
	if request == nil || request.ID == 0 {
		return response.Response[NotifierTestResponse]{}, errors.BadRequest("无效的编号")
	}
	n, err := r.repo.One(ctx, request.ID)
	if err != nil {
		return response.Response[NotifierTestResponse]{}, err
	}
	sender, err := r.registry.Get(n.Type)
	if err != nil {
		r.writeTestLog(ctx, n, false, "", err.Error())
		return response.Response[NotifierTestResponse]{}, errors.Unavailable(err.Error())
	}
	event := &Event{
		Kind:        KindTest,
		EventType:   "TEST",
		MonitorName: "NMonitor",
		MonitorType: "system",
		NewStatus:   "UP",
		Target:      "nmonitor",
		Message:     "监控系统通知测试成功",
		OccurredAt:  time.Now(),
	}
	body, err := sender.Send(ctx, n, event)
	if err != nil {
		r.writeTestLog(ctx, n, false, body, err.Error())
		return response.Response[NotifierTestResponse]{}, errors.Unavailable(err.Error())
	}
	r.writeTestLog(ctx, n, true, body, "")
	return response.OK(NotifierTestResponse{OK: true, Message: "监控系统通知测试成功"}), nil
}

// SearchLogs 分页查询通知日志。
func (r *Resource) SearchLogs(ctx context.Context, search *NotifyLogSearch) (response.PageResponse[NotifyLogSimple], error) {
	if search == nil {
		return response.PageResponse[NotifyLogSimple]{}, errors.BadRequest("请求格式不正确")
	}
	if search.Page < 1 {
		search.Page = 1
	}
	if search.PageSize < 1 {
		search.PageSize = 20
	}
	if search.PageSize > 200 {
		return response.PageResponse[NotifyLogSimple]{}, errors.BadRequest("每页数量不能超过 200")
	}
	return r.repo.SearchLogs(ctx, search)
}

func (r *Resource) writeTestLog(ctx context.Context, n *Notifier, ok bool, body, errMsg string) {
	clip := func(s string, max int) string {
		if len(s) > max {
			return s[:max]
		}
		return s
	}
	_ = r.repo.WriteLog(ctx, &NotificationLog{
		NotifierID:   n.ID,
		NotifierName: n.Name,
		EventType:    "TEST",
		Success:      ok,
		Response:     clip(body, 2000),
		ErrorMessage: clip(errMsg, 2000),
		SentAt:       time.Now(),
	})
}

// buildNotifier 校验请求并应用到实体。creating 区分创建与修改的必填规则。
func buildNotifier(n Notifier, req *NotifierRequest, creating bool) (*Notifier, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	req.WebhookURL = strings.TrimSpace(req.WebhookURL)
	if req.Name == "" {
		return nil, errors.BadRequest("名称不能为空")
	}
	switch req.Type {
	case NotifierDingTalk, NotifierWeCom, NotifierWebhook:
	default:
		return nil, errors.BadRequest("通知类型仅支持 dingtalk、wecom、webhook")
	}
	if creating || (req.WebhookURL != "" && !strings.Contains(req.WebhookURL, "****")) {
		if req.WebhookURL == "" {
			return nil, errors.BadRequest("Webhook 地址不能为空")
		}
		if !strings.HasPrefix(req.WebhookURL, "http://") && !strings.HasPrefix(req.WebhookURL, "https://") {
			return nil, errors.BadRequest("Webhook 地址必须以 http:// 或 https:// 开头")
		}
		n.WebhookURL = req.WebhookURL
	}
	if req.Secret != "" && !strings.Contains(req.Secret, "****") {
		n.Secret = strings.TrimSpace(req.Secret)
	}
	n.Name = req.Name
	n.Type = req.Type
	if req.Enabled == nil {
		n.Enabled = true
	} else {
		n.Enabled = *req.Enabled
	}
	return &n, nil
}
