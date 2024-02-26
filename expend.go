package main

import (
	"github.com/gotify/plugin-api"
	"github.com/gotify/plugin-template/channels"
)

func (c *MyPlugin) sendExtraMessage(token *string, msg plugin.Message) (err error) {
	cs := c.config.GetChannels(token)

	for _, channel := range cs {
		if err = c.sendToChannel(channel, msg); err != nil {
			c.logger.Error("send to channel failed", err)
		}
	}

	return
}

func (c *MyPlugin) sendToChannel(channel map[string]interface{}, msg plugin.Message) (err error) {
	handler := c.getExpendMessageHandler(channel)

	return handler.SendMessage(msg)
}

func (c *MyPlugin) getExpendMessageHandler(channel map[string]interface{}) plugin.MessageHandler {
	ruleType, ok := channel[RuleTypeKey]
	if !ok {
		c.logger.Error("找不到类型")
		return channels.NewNoneClient()
	}
	rt, ok := ruleType.(string)
	if !ok {
		c.logger.Error("类型转换失败")
		return channels.NewNoneClient()
	}
	switch RuleType(rt) {
	case RuleTypeBark:
		return channels.NewBarkClient(channel, c.logger)
	default:
		return channels.NewNoneClient()
	}
}
