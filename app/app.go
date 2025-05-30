package app

import (
	"github.com/fghwett/gotify-plugin-forward/consts"
	"github.com/gotify/plugin-api"
	"log/slog"
	"os"
)

type App struct {
	basePath string
	config   *Config

	user           plugin.UserContext
	messageHandler plugin.MessageHandler
	storageHandler plugin.StorageHandler

	logger *slog.Logger
}

func New(ctx plugin.UserContext) *App {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil)).
		With(slog.String("plugin", consts.PluginName)).
		With(slog.Uint64("user_id", uint64(ctx.ID))).
		With(slog.String("user_name", ctx.Name)).
		With(slog.Bool("is_admin", ctx.Admin))

	a := &App{
		logger: logger,

		user: ctx,
	}

	return a
}

// Enable 启用插件
func (a *App) Enable() error {
	a.logger.Info("插件已启用")
	return nil
}

// Disable 禁用插件
func (a *App) Disable() error {
	a.logger.Warn("插件已禁用")
	return nil
}
