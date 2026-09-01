package app

import (
	"fmt"
	"net/url"
)

// GetDisplay 在插件详情页展示可视化配置页面的入口地址与使用说明。
func (a *App) GetDisplay(location *url.URL) string {
	a.logger.Debug("get display")

	page := "（插件尚未完成注册，请重启 gotify 后查看）"
	if a.basePath != "" {
		loc := &url.URL{Path: a.basePath}
		if location != nil {
			// 从当前访问地址推断完整 URL，兼容反向代理部署
			loc.Scheme = location.Scheme
			loc.Host = location.Host
		}
		page = loc.String()
	}

	return fmt.Sprintf(`### 可视化配置

打开 [配置页面](%s) 管理转发渠道、规则与投递日志，首次打开时需要设置 passkey（需要 HTTPS 或 localhost 访问）。

- 页面分为四个标签：**渠道**（Bark 参数、测试发送）、**规则**（按 token 绑定渠道，支持关键字/正则/优先级匹配）、**日志**（最近投递记录与失败原因）、**关于**（版本、配置导入导出）
- 消息推送地址与原版 gotify 兼容：`+"`%s`"+`?token=应用token
- passkey 丢失时，可在下方原生配置中将 `+"`reset_passkey`"+` 设为 `+"`true`"+` 并保存，即可重新设置（完成后请改回 `+"`false`"+`）`, page, a.basePath+"/message")
}
