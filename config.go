package main

import (
	"encoding/json"
	"fmt"
)

const (
	ConfigVersion = "1"
)

type Config struct {
	Version  string                            `yaml:"version"`
	Rules    map[string][]*Rule                `yaml:"rules"`
	Channels map[string]map[string]interface{} `yaml:"channels"`
}

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

type Rule struct {
	Channel string `yaml:"channel"`
	Enabled bool   `yaml:"enabled"`
}

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

func (c *MyPlugin) DefaultConfig() interface{} {
	c.logger.Info("get default config")

	return defaultConfig
}

func (c *MyPlugin) ValidateAndSetConfig(conf interface{}) error {
	c.logger.With("config", conf).Info("validate and set config")

	c.config = conf.(*Config)
	return nil
}
