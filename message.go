package main

import (
	"github.com/gotify/plugin-api"
)

func (c *MyPlugin) SetMessageHandler(h plugin.MessageHandler) {
	c.logger.Info("set message handler")
	c.messageHandler = h
}
