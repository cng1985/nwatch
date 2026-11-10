package cronjob

import (
	"context"
	"strings"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/cng1985/nwatch/internal/platform/response"
	"gorm.io/gorm"
)

// Repository 负责定时任务的持久化与事务。
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Search 分页查询定时任务。
func (r *Repository) Search(ctx context.Context, search *CronJobSearch) (response.PageResponse[CronJobSimple], error) {
	var result response.PageResponse[CronJobSimple]
	q := r.db.WithContext(ctx).Model(&CronJob{})
	if kw := strings.TrimSpace(search.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR command LIKE ? OR cron LIKE ? OR url LIKE ?", like, like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return result, err
	}
	items := make([]CronJob, 0)
	if err := q.Order("id desc").Offset((search.Page - 1) * search.PageSize).Limit(search.PageSize).Find(&items).Error; err != nil {
		return result, err
	}
	for i := range items {
		Decorate(&items[i])
	}
	return response.Page(items, total, search.Page, search.PageSize), nil
}

// One 按 ID 读取定时任务。
func (r *Repository) One(ctx context.Context, id uint) (response.Response[CronJobResponse], error) {
	var job CronJob
	if err := r.db.WithContext(ctx).First(&job, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.Response[CronJobResponse]{}, errors.NotFound("定时任务不存在")
		}
		return response.Response[CronJobResponse]{}, err
	}
	Decorate(&job)
	return response.OK(job), nil
}

// Create 创建定时任务。
func (r *Repository) Create(ctx context.Context, job *CronJob) (response.Response[CronJobResponse], error) {
	if err := r.db.WithContext(ctx).Create(job).Error; err != nil {
		if isUnique(err) {
			return response.Response[CronJobResponse]{}, errors.BadRequest("名称已存在")
		}
		return response.Response[CronJobResponse]{}, err
	}
	Decorate(job)
	return response.OK(*job), nil
}

// Update 保存定时任务修改。
func (r *Repository) Update(ctx context.Context, job *CronJob) (response.Response[CronJobResponse], error) {
	if err := r.db.WithContext(ctx).Save(job).Error; err != nil {
		if isUnique(err) {
			return response.Response[CronJobResponse]{}, errors.BadRequest("名称已存在")
		}
		return response.Response[CronJobResponse]{}, err
	}
	Decorate(job)
	return response.OK(*job), nil
}

// Delete 删除定时任务及其执行记录，一次事务完成。
func (r *Repository) Delete(ctx context.Context, id uint) (response.Response[CronJobOKResponse], error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job CronJob
		if err := tx.First(&job, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errors.NotFound("定时任务不存在")
			}
			return err
		}
		if err := tx.Where("job_id = ?", id).Delete(&CronRun{}).Error; err != nil {
			return err
		}
		return tx.Delete(&job).Error
	})
	if err != nil {
		return response.Response[CronJobOKResponse]{}, err
	}
	return response.OK(CronJobOKResponse{OK: true}), nil
}

// SearchRuns 分页查询执行记录。
func (r *Repository) SearchRuns(ctx context.Context, search *CronRunSearch) (response.PageResponse[CronRunSimple], error) {
	var result response.PageResponse[CronRunSimple]
	q := r.db.WithContext(ctx).Model(&CronRun{})
	if search.JobID != "" {
		q = q.Where("job_id = ?", search.JobID)
	}
	if kw := strings.TrimSpace(search.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR command LIKE ? OR stdout LIKE ? OR stderr LIKE ? OR error_message LIKE ?", like, like, like, like, like)
	}
	switch search.Success {
	case "true":
		q = q.Where("success = ?", true)
	case "false":
		q = q.Where("success = ?", false)
	}
	if trigger := strings.TrimSpace(search.Trigger); trigger != "" {
		q = q.Where("trigger = ?", trigger)
	}
	if !search.FromTime.IsZero() {
		q = q.Where("started_at >= ?", search.FromTime)
	}
	if !search.ToTime.IsZero() {
		q = q.Where("started_at < ?", search.ToTime)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return result, err
	}
	items := make([]CronRun, 0)
	if err := q.Order("started_at desc, id desc").Offset((search.Page - 1) * search.PageSize).Limit(search.PageSize).Find(&items).Error; err != nil {
		return result, err
	}
	return response.Page(items, total, search.Page, search.PageSize), nil
}

// OneEntity 按 ID 读取实体，供修改流程复用。
func (r *Repository) OneEntity(ctx context.Context, id uint) (*CronJob, error) {
	var job CronJob
	if err := r.db.WithContext(ctx).First(&job, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.NotFound("定时任务不存在")
		}
		return nil, err
	}
	return &job, nil
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}
