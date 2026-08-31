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
		loc := &url.URL{Path: a.basePath + "/config"}
		if location != nil {
			// 从当前访问地址推断完整 URL，兼容反向代理部署
			loc.Scheme = location.Scheme
			loc.Host = location.Host
		}
		page = loc.String()
	}

	return fmt.Sprintf(`### 可视化配置

打开 [配置页面](%s) 编辑转发渠道与规则，首次打开时需要设置 passkey（需要 HTTPS 或 localhost 访问）。

- 消息推送地址与原版 gotify 兼容：`+"`%s`"+`?token=应用token
- passkey 丢失时，可在下方原生配置中将 `+"`reset_passkey`"+` 设为 `+"`true`"+` 并保存，即可重新设置（完成后请改回 `+"`false`"+`）`, page, a.basePath+"/message")
}
