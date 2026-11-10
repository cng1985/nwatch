package group

import (
	"context"

	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"gorm.io/gorm"
)

// Repository 负责分组的持久化与事务。
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// List 列出全部分组并统计组内监控数量。
func (r *Repository) List(ctx context.Context) (response.Response[GroupListResponse], error) {
	var rows []shared.Group
	if err := r.db.WithContext(ctx).Order("sort asc, id asc").Find(&rows).Error; err != nil {
		return response.Response[GroupListResponse]{}, err
	}
	out := make([]GroupItem, 0, len(rows))
	for _, g := range rows {
		var n int64
		if err := r.db.WithContext(ctx).Model(&shared.Monitor{}).Where("group_id = ?", g.ID).Count(&n).Error; err != nil {
			return response.Response[GroupListResponse]{}, err
		}
		out = append(out, GroupItem{Group: g, MonitorCount: n})
	}
	return response.OK(GroupListResponse{Items: out}), nil
}

// Create 创建分组。名称唯一冲突映射为参数错误。
func (r *Repository) Create(ctx context.Context, request *GroupRequest) (response.Response[GroupResponse], error) {
	g := shared.Group{Name: request.Name, Description: request.Description, Sort: request.Sort}
	if err := r.db.WithContext(ctx).Create(&g).Error; err != nil {
		return response.Response[GroupResponse]{}, errors.BadRequest("分组名称已存在")
	}
	return response.OK(g), nil
}

// Update 修改分组。
func (r *Repository) Update(ctx context.Context, request *GroupRequest) (response.Response[GroupResponse], error) {
	var g shared.Group
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&g, request.ID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.NotFound("分组不存在")
			}
			return err
		}
		g.Name = request.Name
		g.Description = request.Description
		g.Sort = request.Sort
		return tx.Save(&g).Error
	})
	if err != nil {
		return response.Response[GroupResponse]{}, err
	}
	return response.OK(g), nil
}

// Delete 删除分组，并把组内监控的分组引用置空，一次事务完成。
func (r *Repository) Delete(ctx context.Context, id uint) (response.Response[GroupOKResponse], error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var g shared.Group
		if err := tx.First(&g, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.NotFound("分组不存在")
			}
			return err
		}
		if err := tx.Model(&shared.Monitor{}).Where("group_id = ?", id).Update("group_id", gorm.Expr("NULL")).Error; err != nil {
			return err
		}
		return tx.Delete(&shared.Group{}, id).Error
	})
	if err != nil {
		return response.Response[GroupOKResponse]{}, err
	}
	return response.OK(GroupOKResponse{OK: true}), nil
}
