package app

import (
	"encoding/json"
	"fmt"
)

type Config struct {
	Version  string                            `yaml:"version" json:"version"`
	Rules    map[string][]*Rule                `yaml:"rules" json:"rules"`
	Channels map[string]map[string]interface{} `yaml:"channels" json:"channels"`
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
	body, _ := json.Marshal(c)
	if token == nil {
		return
	}
	fmt.Println("解析之后的配置", *token, string(body))
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
}

func (a *App) DefaultConfig() interface{} {
	a.logger.Debug("获取默认配置")

	return defaultConfig
}

func (a *App) ValidateAndSetConfig(conf interface{}) error {
	a.logger.With("config", conf).Info("保存配置")

	a.config = conf.(*Config)
	return nil
}
