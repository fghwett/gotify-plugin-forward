package app

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/fghwett/gotify-plugin-forward/channels"
	"github.com/gotify/plugin-api"
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

// RuleMatch 是一条转发规则的匹配条件，多个条件之间是「与」的关系，
// 全部留空表示匹配所有消息。
type RuleMatch struct {
	// TitleContains 标题包含的关键字；UseRegex 时为作用于标题的正则
	TitleContains string `yaml:"title_contains,omitempty" json:"title_contains,omitempty"`
	// MessageContains 内容包含的关键字；UseRegex 时为作用于内容的正则
	MessageContains string `yaml:"message_contains,omitempty" json:"message_contains,omitempty"`
	// ExcludeContains 命中即不转发（检查标题+内容）
	ExcludeContains string `yaml:"exclude_contains,omitempty" json:"exclude_contains,omitempty"`
	// UseRegex 开启后上述包含条件按正则解释
	UseRegex bool `yaml:"use_regex,omitempty" json:"use_regex,omitempty"`
	// PriorityMin / PriorityMax 消息优先级区间（闭区间）
	PriorityMin *int `yaml:"priority_min,omitempty" json:"priority_min,omitempty"`
	PriorityMax *int `yaml:"priority_max,omitempty" json:"priority_max,omitempty"`
}

type Rule struct {
	Channel string `yaml:"channel" json:"channel"`
	Enabled bool   `yaml:"enabled" json:"enabled"`
	// Match 为 nil 时匹配所有消息
	Match *RuleMatch `yaml:"match,omitempty" json:"match,omitempty"`
}

const (
	ConfigVersion = "1"
)

// MatchedRule 带渠道名的命中结果，投递日志需要知道消息发给了哪个渠道。
type MatchedRule struct {
	ChannelName string
	Channel     map[string]interface{}
	Rule        *Rule
}

// MatchedRules 按来源 token 与消息内容筛选出需要投递的渠道。
// token 配置过规则列表（无论启用状态）则完全由其决定走向，
// 否则回落到 all 兜底规则。
func (c *Config) MatchedRules(token *string, msg plugin.Message) []MatchedRule {
	key := RuleTokenAll
	if token != nil {
		if _, ok := c.Rules[*token]; ok {
			key = *token
		}
	}

	var matched []MatchedRule
	for _, rule := range c.Rules[key] {
		if rule == nil || !rule.Enabled {
			continue
		}
		if !rule.Matches(msg) {
			continue
		}
		channel, ok := c.Channels[rule.Channel]
		if !ok {
			continue
		}
		matched = append(matched, MatchedRule{ChannelName: rule.Channel, Channel: channel, Rule: rule})
	}
	return matched
}

// Matches 判断消息是否命中规则：条件全留空则匹配一切。
func (r *Rule) Matches(msg plugin.Message) bool {
	if r.Match == nil {
		return true
	}
	return r.Match.Matches(msg)
}

func (m *RuleMatch) Matches(msg plugin.Message) bool {
	if m.UseRegex {
		for field, pattern := range map[string]string{
			msg.Title:   m.TitleContains,
			msg.Message: m.MessageContains,
		} {
			if pattern == "" {
				continue
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				// 正则无效视为不匹配，保存时 Validate 会拦下这类配置
				return false
			}
			if !re.MatchString(field) {
				return false
			}
		}
	} else {
		if m.TitleContains != "" && !strings.Contains(msg.Title, m.TitleContains) {
			return false
		}
		if m.MessageContains != "" && !strings.Contains(msg.Message, m.MessageContains) {
			return false
		}
	}
	if m.ExcludeContains != "" {
		combined := msg.Title + "\n" + msg.Message
		if m.UseRegex {
			if re, err := regexp.Compile(m.ExcludeContains); err == nil && re.MatchString(combined) {
				return false
			}
		} else if strings.Contains(combined, m.ExcludeContains) {
			return false
		}
	}
	if m.PriorityMin != nil && msg.Priority < *m.PriorityMin {
		return false
	}
	if m.PriorityMax != nil && msg.Priority > *m.PriorityMax {
		return false
	}
	return true
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

// Validate 校验可视化编辑提交的配置：
// 规则引用的渠道必须存在、渠道自身参数完整、匹配条件可用。
func (c *Config) Validate() error {
	for name, channel := range c.Channels {
		if name == "" {
			return errors.New("渠道名不能为空")
		}
		typeVal, ok := channel[RuleTypeKey]
		if !ok {
			return fmt.Errorf("渠道 %s 缺少类型（type）", name)
		}
		ruleType, ok := typeVal.(string)
		if !ok {
			return fmt.Errorf("渠道 %s 的类型（type）无效", name)
		}
		switch RuleType(ruleType) {
		case RuleTypeBark:
			client := channels.NewBarkClient(channel, nil)
			conf, err := client.Parse(channel)
			if err != nil {
				return fmt.Errorf("渠道 %s 配置解析失败: %w", name, err)
			}
			if err = conf.Validate(name); err != nil {
				return err
			}
		default:
			return fmt.Errorf("渠道 %s 的类型（%s）暂不支持", name, ruleType)
		}
	}

	for token, rules := range c.Rules {
		if token == "" {
			return errors.New("规则 token 不能为空")
		}
		for _, rule := range rules {
			if rule == nil {
				return fmt.Errorf("规则组 %s 存在空规则", token)
			}
			if rule.Channel == "" {
				return fmt.Errorf("规则组 %s 存在未选择渠道的规则", token)
			}
			if _, ok := c.Channels[rule.Channel]; !ok {
				return fmt.Errorf("规则引用了不存在的渠道: %s", rule.Channel)
			}
			if err := rule.validateMatch(token); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Rule) validateMatch(token string) error {
	m := r.Match
	if m == nil {
		return nil
	}
	if m.UseRegex {
		for field, pattern := range map[string]string{"标题": m.TitleContains, "内容": m.MessageContains, "排除": m.ExcludeContains} {
			if pattern == "" {
				continue
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("规则组 %s 的%s正则无效: %w", token, field, err)
			}
		}
	}
	if m.PriorityMin != nil && m.PriorityMax != nil && *m.PriorityMin > *m.PriorityMax {
		return fmt.Errorf("规则组 %s 的优先级区间无效：min %d 大于 max %d", token, *m.PriorityMin, *m.PriorityMax)
	}
	return nil
}

// PageURL 返回可视化配置页地址，供原生配置页展示。
func (c *Config) PageURL(basePath string) string {
	return basePath + "/config"
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
