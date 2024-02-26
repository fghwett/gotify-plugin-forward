package main

import "github.com/gotify/plugin-api"

func (c *MyPlugin) SetStorageHandler(h plugin.StorageHandler) {
	c.logger.Info("set storage handler")
	c.storageHandler = h
}
