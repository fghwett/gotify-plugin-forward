package app

import (
	"errors"
	"fmt"
)

type Config struct {
	Version  string                            `yaml:"version" json:"version"`
	Rules    map[string][]*Rule                `yaml:"rules" json:"rules"`
	Channels map[string]map[string]interface{} `yaml:"channels" json:"channels"`
	// ResetPasskey 是 passkey 丢失时的兜底开关：在 gotify 原生配置中将其
	// 置为 true 并保存，插件会清除已设置的 passkey，重新打开页面即可重设。
	// 重设完成后请改回 false。
	ResetPasskey bool `yaml:"reset_passkey,omitempty" json:"reset_passkey,omitempty"`
}

type Rule struct {
	Channel string `yaml:"channel" json:"channel"`
	Enabled bool   `yaml:"enabled" json:"enabled"`
}

const (
	ConfigVersion = "1"
)

func (c *Config) GetChannels(token *string) []map[string]interface{} {
	if token == nil {
		token = c.PtrString(RuleTokenAll)
	} else {
		if channels := c.getSpecialChannels(token); len(channels) != 0 {
			return channels
		}
		token = c.PtrString(RuleTokenAll)
	}

	return c.getSpecialChannels(token)
}

func (c *Config) PtrString(s string) *string {
	return &s
}

func (c *Config) getSpecialChannels(token *string) (channels []map[string]interface{}) {
	if token == nil {
		return
	}
	rules, ok := c.Rules[*token]
	if !ok {
		return
	}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		channel, o := c.Channels[rule.Channel]
		if !o {
			continue
		}
		channels = append(channels, channel)
	}
	return
}

type RuleType string

const (
	RuleTokenAll = "all"

	RuleTypeKey           = "type"
	RuleTypeBark RuleType = "bark"
)

var defaultConfig = &Config{
	Version: ConfigVersion,
	Channels: map[string]map[string]interface{}{
		"exampleBark": {
			RuleTypeKey: RuleTypeBark,
			"url":       "https://api.day.app/token",
		},
	},
	Rules: map[string][]*Rule{
		RuleTokenAll: {
			{
				Channel: "exampleBark",
				Enabled: false,
			},
		},
	},
	ResetPasskey: false,
}

// Validate 校验可视化编辑提交的配置：规则引用的渠道必须存在。
func (c *Config) Validate() error {
	for _, rules := range c.Rules {
		for _, rule := range rules {
			if rule == nil || rule.Channel == "" {
				continue
			}
			if _, ok := c.Channels[rule.Channel]; !ok {
				return fmt.Errorf("规则引用了不存在的渠道: %s", rule.Channel)
			}
		}
	}
	return nil
}

func (a *App) DefaultConfig() interface{} {
	a.logger.Debug("获取默认配置")

	return defaultConfig
}

func (a *App) ValidateAndSetConfig(conf interface{}) error {
	a.logger.Info("保存配置")

	c, ok := conf.(*Config)
	if !ok {
		return errors.New("config type error")
	}
	a.config = c

	if c.ResetPasskey {
		if err := a.store.ClearPasskey(); err != nil {
			return err
		}
		a.logger.Warn("reset_passkey 已触发，passkey 凭据已清除，请尽快重新设置")
	}
	return nil
}
