package main

import (
	"github.com/fghwett/gotify-plugin-forward/app"
	"github.com/fghwett/gotify-plugin-forward/consts"
	"github.com/gotify/plugin-api"
)

// GetGotifyPluginInfo 返回插件信息
func GetGotifyPluginInfo() plugin.Info {
	return plugin.Info{
		Version:     consts.PluginVersion,
		Author:      consts.PluginAuthor,
		Name:        consts.PluginName,
		Website:     consts.PluginWebsite,
		Description: consts.PluginDescription,
		License:     consts.PluginLicense,
		ModulePath:  consts.PluginModulePath,
	}
}

// NewGotifyPluginInstance 为单个用户创建一个插件实例
func NewGotifyPluginInstance(ctx plugin.UserContext) plugin.Plugin {
	a := app.New(ctx)

	return a
}

func main() {
	panic("this should be built as go plugin")
}
