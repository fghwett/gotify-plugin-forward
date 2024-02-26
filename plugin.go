package main

import (
	"github.com/gotify/plugin-api"
	"log/slog"
)

const (
	PluginName = "gotify-plugin-forward"
)

// GetGotifyPluginInfo returns gotify plugin info.
func GetGotifyPluginInfo() plugin.Info {
	return plugin.Info{
		ModulePath:  "github.com/fghwett/gotify-plugin-forward",
		Version:     "0.0.1",
		Author:      "FGHWETT",
		Website:     "https://github.com/fghwett/gotify-plugin-forward",
		Description: "Forward gotify message to Bark or DingTalk etc.",
		License:     "MIT",
		Name:        PluginName,
	}
}

// MyPlugin is the gotify plugin instance.
type MyPlugin struct {
	basePath string
	config   *Config
	logger   *slog.Logger

	user           plugin.UserContext
	messageHandler plugin.MessageHandler
	storageHandler plugin.StorageHandler
}

// Enable enables the plugin.
func (c *MyPlugin) Enable() error {
	c.logger.Info("Plugin enabled")
	return nil
}

// Disable disables the plugin.
func (c *MyPlugin) Disable() error {
	c.logger.Info("Plugin disabled")
	return nil
}

// NewGotifyPluginInstance creates a plugin instance for a user context.
func NewGotifyPluginInstance(ctx plugin.UserContext) plugin.Plugin {
	logger := NewLogger().With(slog.String("user", ctx.Name))
	logger.Info("Creating plugin instance")

	return &MyPlugin{
		user:   ctx,
		logger: logger,
	}
}

func main() {
	panic("this should be built as go plugin")
}
