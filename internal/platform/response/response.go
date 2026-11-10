// Package response 定义泛型结果与 Gin 直接输出函数。
// 普通业务 JSON API 默认 POST + JSON，成功与应用错误均返回 HTTP 200；
// 分页结果原样输出 PageResponse，不额外嵌套 Response。
package response

import (
	"net/http"

	"github.com/cng1985/nwatch/internal/platform/errors"
	"github.com/gin-gonic/gin"
)

// Response 是单条结果的统一外壳。Code=200 表示成功，Msg 默认 success。
type Response[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data,omitempty"`
}

// PageResponse 是分页结果的统一外壳，原样输出，不再嵌套 Response。
type PageResponse[T any] struct {
	Data     []T   `json:"data"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// OK 构造单条成功结果。
func OK[T any](data T) Response[T] {
	return Response[T]{Code: 200, Msg: "success", Data: data}
}

// Page 构造分页成功结果，空列表初始化为 []，不返回 null。
func Page[T any](data []T, total int64, page, pageSize int) PageResponse[T] {
	if data == nil {
		data = []T{}
	}
	return PageResponse[T]{Data: data, Total: total, Page: page, PageSize: pageSize}
}

// SuccessResponse 直接输出已有 Response。
func SuccessResponse[T any](c *gin.Context, data Response[T]) {
	c.JSON(http.StatusOK, data)
}

// SuccessPage 直接输出分页结果。
func SuccessPage[T any](c *gin.Context, data PageResponse[T]) {
	c.JSON(http.StatusOK, data)
}

// HandleError 统一分类错误映射。原始错误细节写服务端日志，不拼接给客户端。
func HandleError(c *gin.Context, err error) {
	appErr := errors.From(err)
	if appErr.Kind == errors.KindInternal {
		c.Error(err) // gin.Context.Error 供日志中间件在请求边界统一记录一次。
	}
	c.JSON(http.StatusOK, gin.H{"code": appErr.Code, "msg": appErr.Message})
}
