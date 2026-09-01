package app

import (
	"sync"
	"time"

	"github.com/fghwett/gotify-plugin-forward/channels"
	"github.com/gotify/plugin-api"
)

const (
	// deliverAttempts 单渠道最大尝试次数（含首次）
	deliverAttempts = 3
	// deliverBackoff 重试退避基准，实际等待为 base * 2^(已试次数-1)
	deliverBackoff = 500 * time.Millisecond
)

// sendExtraMessage 把消息投递到命中的渠道。
// 异步执行：转发是消息主流程之外的附加动作，不应拖慢 webhook 响应；
// 失败会重试并记录到投递日志，可在配置页面查看。
func (a *App) sendExtraMessage(token *string, msg plugin.Message) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.logger.Error("转发协程 panic", "recover", r)
			}
		}()
		a.forward(token, msg)
	}()
}

// forward 是转发的同步实现（便于测试）：筛选命中渠道后并行投递。
func (a *App) forward(token *string, msg plugin.Message) {
	conf := a.effectiveConfig()
	if conf == nil {
		return
	}

	tokenValue := RuleTokenAll
	if token != nil {
		tokenValue = *token
	}

	var wg sync.WaitGroup
	for _, target := range conf.MatchedRules(token, msg) {
		wg.Add(1)
		go func(t MatchedRule) {
			defer wg.Done()
			a.deliver(t, tokenValue, msg)
		}(target)
	}
	wg.Wait()
}

// deliver 带退避重试地投递到单个渠道，并把最终结果写入投递日志。
func (a *App) deliver(target MatchedRule, token string, msg plugin.Message) {
	begin := time.Now()
	var lastErr error
	attempts := 0

	for attempts < deliverAttempts {
		attempts++
		err := a.sendToChannel(target.Channel, msg)
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
		if attempts < deliverAttempts {
			time.Sleep(deliverBackoff * time.Duration(1<<uint(attempts-1)))
		}
	}

	cost := time.Since(begin).Milliseconds()
	if lastErr != nil {
		a.logger.Error("转发到渠道失败", "channel", target.ChannelName, "token", token, "attempts", attempts, "error", lastErr)
	} else {
		a.logger.Debug("转发到渠道成功", "channel", target.ChannelName, "token", token, "attempts", attempts, "cost_ms", cost)
	}

	entry := DeliveryLog{
		Time:     time.Now(),
		Token:    token,
		Channel:  target.ChannelName,
		Title:    truncateRunes(msg.Title, 64),
		Excerpt:  truncateRunes(msg.Message, 160),
		OK:       lastErr == nil,
		Attempts: attempts,
		CostMS:   cost,
	}
	if lastErr != nil {
		entry.Error = truncateRunes(lastErr.Error(), 300)
	}
	if err := a.store.AppendLog(entry); err != nil {
		a.logger.Error("写入投递日志失败", "error", err)
	}
}

func (a *App) sendToChannel(channel map[string]interface{}, msg plugin.Message) error {
	handler := a.getExpendMessageHandler(channel)

	return handler.SendMessage(msg)
}

func (a *App) getExpendMessageHandler(channel map[string]interface{}) plugin.MessageHandler {
	ruleType, ok := channel[RuleTypeKey]
	if !ok {
		a.logger.Error("找不到类型")
		return channels.NewNoneClient()
	}
	rt, ok := ruleType.(string)
	if !ok {
		a.logger.Error("类型转换失败")
		return channels.NewNoneClient()
	}
	switch RuleType(rt) {
	case RuleTypeBark:
		return channels.NewBarkClient(channel, a.logger)
	default:
		return channels.NewNoneClient()
	}
}
