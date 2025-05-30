package app

import (
	"github.com/gotify/plugin-api"
)

func (a *App) SetMessageHandler(h plugin.MessageHandler) {
	a.logger.Info("初始化消息处理器")
	a.messageHandler = h
}
