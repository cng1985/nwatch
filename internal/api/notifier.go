package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/notifier"
	"github.com/gin-gonic/gin"
)

type notifierDTO struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	WebhookURL string    `json:"webhookUrl"`
	SecretSet  bool      `json:"secretSet"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func toNotifierDTO(n model.Notifier) notifierDTO {
	return notifierDTO{
		ID: n.ID, Name: n.Name, Type: n.Type,
		WebhookURL: notifier.MaskSecret(n.WebhookURL),
		SecretSet:  n.Secret != "",
		Enabled:    n.Enabled,
		CreatedAt:  n.CreatedAt, UpdatedAt: n.UpdatedAt,
	}
}

type notifierRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	WebhookURL string `json:"webhookUrl"`
	Secret     string `json:"secret"`
	Enabled    *bool  `json:"enabled"`
}

func (s *Server) listNotifiers(c *gin.Context) {
	var rows []model.Notifier
	if err := s.db.Order("id desc").Find(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	items := make([]notifierDTO, 0, len(rows))
	for _, n := range rows {
		items = append(items, toNotifierDTO(n))
	}
	ok(c, items)
}

func (s *Server) createNotifier(c *gin.Context) {
	var req notifierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	n, err := buildNotifier(model.Notifier{}, req, true)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.db.Create(&n).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建失败")
		return
	}
	ok(c, toNotifierDTO(n))
}

func (s *Server) updateNotifier(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var n model.Notifier
	if err := s.db.First(&n, id).Error; err != nil {
		fail(c, http.StatusNotFound, "通知渠道不存在")
		return
	}
	var req notifierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "请求格式不正确")
		return
	}
	updated, err := buildNotifier(n, req, false)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.db.Save(&updated).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存失败")
		return
	}
	ok(c, toNotifierDTO(updated))
}

func (s *Server) deleteNotifier(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var n model.Notifier
	if err := s.db.First(&n, id).Error; err != nil {
		fail(c, http.StatusNotFound, "通知渠道不存在")
		return
	}
	if err := s.db.Exec("DELETE FROM monitor_notifiers WHERE notifier_id = ?", id).Error; err != nil {
		fail(c, http.StatusInternalServerError, "删除绑定失败")
		return
	}
	if err := s.db.Delete(&n).Error; err != nil {
		fail(c, http.StatusInternalServerError, "删除失败")
		return
	}
	ok(c, gin.H{"ok": true})
}

func (s *Server) testNotifier(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var n model.Notifier
	if err := s.db.First(&n, id).Error; err != nil {
		fail(c, http.StatusNotFound, "通知渠道不存在")
		return
	}
	if err := s.alerts.SendTest(c.Request.Context(), &n); err != nil {
		fail(c, http.StatusBadGateway, err.Error())
		return
	}
	ok(c, gin.H{"ok": true, "message": "监控系统通知测试成功"})
}

func buildNotifier(n model.Notifier, req notifierRequest, creating bool) (model.Notifier, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	req.WebhookURL = strings.TrimSpace(req.WebhookURL)
	if req.Name == "" {
		return n, errString("名称不能为空")
	}
	switch req.Type {
	case model.NotifierDingTalk, model.NotifierWeCom, model.NotifierWebhook:
	default:
		return n, errString("通知类型仅支持 dingtalk、wecom、webhook")
	}
	if creating || (req.WebhookURL != "" && !strings.Contains(req.WebhookURL, "****")) {
		if req.WebhookURL == "" {
			return n, errString("Webhook 地址不能为空")
		}
		if !strings.HasPrefix(req.WebhookURL, "http://") && !strings.HasPrefix(req.WebhookURL, "https://") {
			return n, errString("Webhook 地址必须以 http:// 或 https:// 开头")
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
	return n, nil
}

type errString string

func (e errString) Error() string { return string(e) }
