// Package domain 定义平台公共模型：可信 Scope 与公共分页请求。
// Scope 不依赖 Gin，经中间件认证后写入标准 context。
package domain

import "context"

// Scope 是一次请求或任务的可信身份上下文，按不可变方式使用。
type Scope struct {
	ActorID   uint
	Username  string
	RequestID string
}

type scopeKey struct{}

// WithScope 把可信 Scope 写入标准 context。
func WithScope(ctx context.Context, scope *Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

// FromContext 读取可信 Scope；缺失时返回 nil，调用方必须按未认证处理。
func FromContext(ctx context.Context) *Scope {
	if ctx == nil {
		return nil
	}
	scope, _ := ctx.Value(scopeKey{}).(*Scope)
	return scope
}

// PageRequest 是公共分页请求的默认边界：page 从 1 开始，page_size 默认 20、上限 200。
const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 200
	MaxNonPageRows  = 1000
	MaxOffsetRows   = 100000
)

// NormalizePage 补全分页默认值并校验边界，非法值返回 false。
func NormalizePage(page, pageSize int) (int, int, bool) {
	if page < 1 {
		page = DefaultPage
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		return 0, 0, false
	}
	// 先限制乘法操作数，避免极大 page 导致 offset 溢出。
	if int64(page-1) > int64(MaxOffsetRows)/int64(pageSize) {
		return 0, 0, false
	}
	return page, pageSize, true
}
