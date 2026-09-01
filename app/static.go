package app

import (
	"embed"
	"mime"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
)

// webFS 内嵌 SolidJS 构建的可视化配置页面（由 Taskfile 的 ui-build 任务生成到 app/web）。
//
//go:embed all:web
var webFS embed.FS

// handleConfigPage 返回可视化配置页面。
// vite 以相对路径（base './'）构建，页面可挂在 gotify 动态分配的插件路径下。
func (a *App) handleConfigPage(ctx *gin.Context) {
	body, err := webFS.ReadFile("web/index.html")
	if err != nil {
		a.abortInternal(ctx, err)
		return
	}
	// index.html 每次都回源校验，保证发版后浏览器立即拿到新页面；
	// 引用的 assets 文件名带内容哈希，可长期缓存
	ctx.Header("Cache-Control", "no-cache")
	ctx.Data(http.StatusOK, "text/html; charset=utf-8", body)
}

// handleConfigAssets 提供页面静态资源（js/css 等）。
func (a *App) handleConfigAssets(ctx *gin.Context) {
	// ctx.Param("filepath") 形如 /index-xxxx.js，规范化后拼接到 web/assets 之下
	name := path.Clean("/" + ctx.Param("filepath"))
	body, err := webFS.ReadFile(path.Join("web", "assets", name))
	if err != nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	ctx.Header("Cache-Control", "public, max-age=31536000, immutable")
	ctx.Data(http.StatusOK, contentType, body)
}
