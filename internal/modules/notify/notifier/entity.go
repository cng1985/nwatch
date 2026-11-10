// Package notifier 提供通知渠道的实体、发送适配与管理业务。
// 发送适配（dingtalk/wecom/webhook）是底层能力；渠道 CRUD 与测试是本模块业务。
package notifier

import "time"

// Notifier 是一个通知渠道。WebhookURL 与 Secret 不对外输出。
type Notifier struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"size:128;not null" json:"name"`
	Type       string    `gorm:"size:32;not null;index" json:"type"`
	WebhookURL string    `gorm:"size:2048" json:"-"`
	Secret     string    `gorm:"size:512" json:"-"`
	Enabled    bool      `gorm:"not null" json:"enabled"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (Notifier) TableName() string { return "notifiers" }

// NotificationLog 是一次通知发送的记录。
type NotificationLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	MonitorID    uint      `gorm:"index" json:"monitorId"`
	NotifierID   uint      `gorm:"index" json:"notifierId"`
	NotifierName string    `gorm:"size:128" json:"notifierName"`
	EventID      uint      `gorm:"index" json:"eventId"`
	EventType    string    `gorm:"size:32" json:"eventType"`
	Success      bool      `json:"success"`
	Response     string    `gorm:"type:text" json:"response"`
	ErrorMessage string    `gorm:"size:2048" json:"errorMessage"`
	SentAt       time.Time `gorm:"index" json:"sentAt"`
}

func (NotificationLog) TableName() string { return "notification_logs" }
