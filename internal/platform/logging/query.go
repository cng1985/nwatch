package logging

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// LogQuery 是日志查询的筛选条件。
type LogQuery struct {
	Level    string
	Keyword  string
	Page     int
	PageSize int
}

// LogPage 是一页日志与总数。
type LogPage struct {
	Items []AppLog
	Total int64
}

// QueryLogs 按级别与关键字分页查询进程日志。参数绑定由调用方保证。
func QueryLogs(db *gorm.DB, q LogQuery) (LogPage, error) {
	query := db.WithContext(db.Statement.Context).Model(&AppLog{})
	if level := strings.ToUpper(strings.TrimSpace(q.Level)); level != "" {
		query = query.Where("level = ?", level)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("message LIKE ? OR attrs LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return LogPage{}, err
	}
	items := make([]AppLog, 0)
	err := query.Order("logged_at desc, id desc").
		Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error
	return LogPage{Items: items, Total: total}, err
}

// PruneBefore 清理指定时间之前的日志，返回删除行数。
func PruneBefore(db *gorm.DB, cutoff time.Time) (int64, error) {
	res := db.Where("logged_at < ?", cutoff).Delete(&AppLog{})
	return res.RowsAffected, res.Error
}
