// Package web 暴露前端构建产物并注册静态资源。
package web

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterStatic 注册 SPA 静态资源回退。只注册处理器，不启动服务；注册失败使初始化失败。
func RegisterStatic(engine *gin.Engine) error {
	engine.NoRoute(func(c *gin.Context) {
		// 未知 API 返回接口错误，不回退 HTML；SPA 回退只用于页面路由。
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "接口不存在"})
			return
		}
		sub, err := fs.Sub(Dist, "dist")
		if err != nil {
			c.String(http.StatusInternalServerError, "前端资源缺失")
			return
		}
		rel := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
		if rel == "" || rel == "." {
			rel = "index.html"
		}
		if _, err := fs.Stat(sub, rel); err != nil {
			rel = "index.html"
		}
		data, err := fs.ReadFile(sub, rel)
		if err != nil {
			c.String(http.StatusNotFound, "页面不存在")
			return
		}
		ctype := mime.TypeByExtension(path.Ext(rel))
		switch path.Ext(rel) {
		case ".js":
			ctype = "text/javascript; charset=utf-8"
		case ".css":
			ctype = "text/css; charset=utf-8"
		case ".html":
			ctype = "text/html; charset=utf-8"
		}
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		if rel == "index.html" {
			c.Header("Cache-Control", "no-cache")
		} else {
			c.Header("Cache-Control", "public, max-age=86400")
		}
		c.Data(http.StatusOK, ctype, data)
	})
	return nil
}
