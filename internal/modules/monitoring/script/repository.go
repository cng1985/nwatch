package script

import (
	"context"

	"github.com/cng1985/nwatch/internal/modules/monitoring/checker"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"gorm.io/gorm"
)

// Repository 负责执行记录的持久化与查询。
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create 保存一次执行记录。
func (r *Repository) Create(ctx context.Context, row *shared.ScriptRun) error {
	return r.db.WithContext(ctx).Create(row).Error
}

// Prune 清理过量的历史执行记录。
func (r *Repository) Prune(ctx context.Context) {
	checker.PruneScriptRuns(r.db.WithContext(ctx))
}

// Search 分页查询执行记录。
func (r *Repository) Search(ctx context.Context, search *ScriptRunSearch) (response.PageResponse[ScriptRunSimple], error) {
	var result response.PageResponse[ScriptRunSimple]
	q := r.db.WithContext(ctx).Model(&shared.ScriptRun{})
	if kw := trimSpace(search.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR command LIKE ?", like, like)
	}
	if id := search.MonitorID; id != "" {
		q = q.Where("monitor_id = ?", id)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return result, err
	}
	items := make([]ScriptRunSimple, 0)
	if err := q.Order("started_at desc").Offset((search.Page - 1) * search.PageSize).Limit(search.PageSize).Find(&items).Error; err != nil {
		return result, err
	}
	return response.Page(items, total, search.Page, search.PageSize), nil
}

// One 按 ID 读取执行记录。
func (r *Repository) One(ctx context.Context, id uint) (response.Response[ScriptRunResponse], error) {
	var row shared.ScriptRun
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.Response[ScriptRunResponse]{}, errors.NotFound("执行记录不存在")
		}
		return response.Response[ScriptRunResponse]{}, err
	}
	return response.OK(row), nil
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
