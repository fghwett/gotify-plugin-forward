package app

import (
	"github.com/fghwett/gotify-plugin-forward/channels"
	"github.com/gotify/plugin-api"
)

func (a *App) sendExtraMessage(token *string, msg plugin.Message) (err error) {
	conf := a.effectiveConfig()
	if conf == nil {
		return
	}
	cs := conf.GetChannels(token)

	for _, channel := range cs {
		if err = a.sendToChannel(channel, msg); err != nil {
			a.logger.Error("send to channel failed", "error", err)
		}
	}

	return
}

func (a *App) sendToChannel(channel map[string]interface{}, msg plugin.Message) (err error) {
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
