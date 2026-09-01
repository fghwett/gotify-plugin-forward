package channels

import (
	"net/http"
	"time"
)

// HTTPClient 是所有出站请求共用的客户端。
// 必须带超时：转发目标挂起时若无限等待，会拖住整个插件乃至 gotify 的请求处理。
var HTTPClient = &http.Client{Timeout: 10 * time.Second}
