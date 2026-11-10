package notifier

import (
	"context"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"gorm.io/gorm"
)

// Repository 负责通知渠道与通知日志的持久化。
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// List 列出全部通知渠道。
func (r *Repository) List(ctx context.Context) (response.Response[NotifierListResponse], error) {
	var rows []Notifier
	if err := r.db.WithContext(ctx).Order("id desc").Find(&rows).Error; err != nil {
		return response.Response[NotifierListResponse]{}, err
	}
	items := make([]NotifierSimple, 0, len(rows))
	for _, n := range rows {
		items = append(items, toSimple(n))
	}
	return response.OK(NotifierListResponse{Items: items}), nil
}

// Create 创建通知渠道。
func (r *Repository) Create(ctx context.Context, n *Notifier) (response.Response[NotifierResponse], error) {
	if err := r.db.WithContext(ctx).Create(n).Error; err != nil {
		return response.Response[NotifierResponse]{}, err
	}
	return response.OK(toSimple(*n)), nil
}

// Update 保存通知渠道修改。
func (r *Repository) Update(ctx context.Context, n *Notifier) (response.Response[NotifierResponse], error) {
	if err := r.db.WithContext(ctx).Save(n).Error; err != nil {
		return response.Response[NotifierResponse]{}, err
	}
	return response.OK(toSimple(*n)), nil
}

// Delete 删除通知渠道并解除监控绑定，一次事务完成。
func (r *Repository) Delete(ctx context.Context, id uint) (response.Response[NotifierOKResponse], error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var n Notifier
		if err := tx.First(&n, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.NotFound("通知渠道不存在")
			}
			return err
		}
		if err := tx.Exec("DELETE FROM monitor_notifiers WHERE notifier_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&n).Error
	})
	if err != nil {
		return response.Response[NotifierOKResponse]{}, err
	}
	return response.OK(NotifierOKResponse{OK: true}), nil
}

// One 按 ID 读取通知渠道。
func (r *Repository) One(ctx context.Context, id uint) (*Notifier, error) {
	var n Notifier
	if err := r.db.WithContext(ctx).First(&n, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.NotFound("通知渠道不存在")
		}
		return nil, err
	}
	return &n, nil
}

// SearchLogs 分页查询通知日志。
func (r *Repository) SearchLogs(ctx context.Context, search *NotifyLogSearch) (response.PageResponse[NotifyLogSimple], error) {
	var result response.PageResponse[NotifyLogSimple]
	q := r.db.WithContext(ctx).Model(&NotificationLog{})
	if v := search.NotifierID; v != "" {
		q = q.Where("notifier_id = ?", v)
	}
	if search.Success == "true" || search.Success == "false" {
		q = q.Where("success = ?", search.Success == "true")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return result, err
	}
	items := make([]NotificationLog, 0)
	if err := q.Order("sent_at desc").Offset((search.Page - 1) * search.PageSize).Limit(search.PageSize).Find(&items).Error; err != nil {
		return result, err
	}
	return response.Page(items, total, search.Page, search.PageSize), nil
}

// WriteLog 写入一条通知日志，供测试发送记录结果。
func (r *Repository) WriteLog(ctx context.Context, row *NotificationLog) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func toSimple(n Notifier) NotifierSimple {
	return NotifierSimple{
		ID: n.ID, Name: n.Name, Type: n.Type,
		WebhookURL: MaskSecret(n.WebhookURL),
		SecretSet:  n.Secret != "",
		Enabled:    n.Enabled,
		CreatedAt:  n.CreatedAt, UpdatedAt: n.UpdatedAt,
	}
}
