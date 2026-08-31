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

	// store 管理可视化配置与 passkey 凭据的持久化
	store *store
	// webauthn 管理 passkey 挑战与登录会话（实例内存态，重启后需重新登录）
	webauthn *webauthnState
	// registeringHandle 保存注册进行中的 user handle，完成后随凭据持久化
	registeringHandle []byte

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

		store:    newStore(),
		webauthn: newWebauthnState(),
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

// effectiveConfig 返回当前生效的配置：可视化编辑保存的配置优先，
// 未保存过则回落到 gotify 原生 YAML 配置，均无时返回 nil。
func (a *App) effectiveConfig() *Config {
	if conf := a.store.Config(); conf != nil {
		return conf
	}
	return a.config
}
