package app

import (
	"time"
)

// MaxDeliveryLogs 投递日志环形上限：只保留最近的记录，避免存储无限膨胀。
const MaxDeliveryLogs = 200

// DeliveryLog 是一次「消息 → 渠道」投递的结果记录，持久化在插件存储中。
type DeliveryLog struct {
	Time     time.Time `json:"time"`
	Token    string    `json:"token"`
	Channel  string    `json:"channel"`
	Title    string    `json:"title"`
	Excerpt  string    `json:"excerpt,omitempty"`
	OK       bool      `json:"ok"`
	Error    string    `json:"error,omitempty"`
	Attempts int       `json:"attempts"`
	CostMS   int64     `json:"cost_ms"`
}

// truncateRunes 按字符数截断，避免日志里塞进整篇消息正文。
func truncateRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}
