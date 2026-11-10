package notifier

import "time"

// NotifierRequest 是通知渠道的创建与修改参数。
type NotifierRequest struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	WebhookURL string `json:"webhookUrl"`
	Secret     string `json:"secret"`
	Enabled    *bool  `json:"enabled"`
}

// NotifierSimple 是通知渠道列表行，敏感信息已打码。
type NotifierSimple struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	WebhookURL string    `json:"webhookUrl"`
	SecretSet  bool      `json:"secretSet"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// NotifierResponse 是通知渠道详情与操作结果。
type NotifierResponse = NotifierSimple

// NotifierListResponse 是通知渠道列表结果。
type NotifierListResponse struct {
	Items []NotifierSimple `json:"items"`
}

// NotifierTestRequest 是通知渠道测试参数。
type NotifierTestRequest struct {
	ID uint `json:"id"`
}

// NotifierTestResponse 是测试发送结果。
type NotifierTestResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// NotifierOKResponse 是删除动作回执。
type NotifierOKResponse struct {
	OK bool `json:"ok"`
}

// NotifyLogSearch 是通知日志分页查询参数。
type NotifyLogSearch struct {
	NotifierID string `form:"notifierId"`
	Success    string `form:"success"`
	Page       int    `form:"page"`
	PageSize   int    `form:"pageSize"`
}

// NotifyLogSimple 是通知日志列表行。
type NotifyLogSimple = NotificationLog
