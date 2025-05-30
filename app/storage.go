package app

import "github.com/gotify/plugin-api"

func (a *App) SetStorageHandler(h plugin.StorageHandler) {
	a.logger.Info("初始化存储处理器")
	a.storageHandler = h
}
