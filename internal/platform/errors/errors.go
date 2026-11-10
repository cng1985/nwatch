// Package errors 定义平台统一的错误分类、应用码与可公开消息。
// 本包不依赖 Gin、GORM 或具体业务。
package errors

import "errors"

// Kind 是错误分类。应用码与 HTTP 状态码分开，但默认值保持一致。
type Kind int

const (
	KindUnknown      Kind = iota
	KindBadRequest        // 参数错误，应用码 400
	KindUnauthorized      // 未认证，应用码 401
	KindForbidden         // 无权限，应用码 403
	KindNotFound          // 不存在，应用码 404
	KindConflict          // 冲突，应用码 409
	KindTooLarge          // 请求过大，应用码 413
	KindRateLimited       // 限流，应用码 429
	KindInternal          // 内部错误，应用码 500
	KindUnavailable       // 依赖暂不可用，应用码 503
)

func (k Kind) code() int {
	switch k {
	case KindBadRequest:
		return 400
	case KindUnauthorized:
		return 401
	case KindForbidden:
		return 403
	case KindNotFound:
		return 404
	case KindConflict:
		return 409
	case KindTooLarge:
		return 413
	case KindRateLimited:
		return 429
	case KindUnavailable:
		return 503
	default:
		return 500
	}
}

// Error 是平台统一错误：分类 + 应用码 + 可公开消息 + 原始 Cause。
type Error struct {
	Kind    Kind
	Code    int
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

// New 构造指定分类的错误，应用码取该分类默认值。
func New(kind Kind, message string) *Error {
	return &Error{Kind: kind, Code: kind.code(), Message: message}
}

// Wrap 构造带原始错误链的错误，供错误链检查。
func Wrap(kind Kind, message string, cause error) *Error {
	return &Error{Kind: kind, Code: kind.code(), Message: message, Cause: cause}
}

func BadRequest(message string) *Error   { return New(KindBadRequest, message) }
func Unauthorized(message string) *Error { return New(KindUnauthorized, message) }
func Forbidden(message string) *Error    { return New(KindForbidden, message) }
func NotFound(message string) *Error     { return New(KindNotFound, message) }
func Conflict(message string) *Error     { return New(KindConflict, message) }
func TooLarge(message string) *Error     { return New(KindTooLarge, message) }
func RateLimited(message string) *Error  { return New(KindRateLimited, message) }
func Internal(message string) *Error     { return New(KindInternal, message) }
func Unavailable(message string) *Error  { return New(KindUnavailable, message) }

// From 把任意 error 归类为平台错误。未知错误隐藏细节，仅返回通用内部错误。
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Kind: KindInternal, Code: 500, Message: "服务器内部错误", Cause: err}
}

// Is 报告 err 链上是否存在指定分类的错误。
func Is(err error, kind Kind) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind == kind
	}
	return false
}
