package channels

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gotify/plugin-api"
)

// BarkConfig 对应可视化编辑 / YAML 配置中的一个 Bark 渠道。
type BarkConfig struct {
	Url    string  `yaml:"url" json:"url"`
	AesKey *string `yaml:"aes_key,omitempty" json:"aes_key,omitempty"`
	AesIV  *string `yaml:"aes_iv,omitempty" json:"aes_iv,omitempty"`
	// Sound 推送铃声，如 bell、alarm、calippo 等，留空用 App 默认
	Sound string `yaml:"sound,omitempty" json:"sound,omitempty"`
	// Level 时效性等级：留空/auto 按消息优先级推导，也可显式指定
	// active / timeSensitive / passive
	Level string `yaml:"level,omitempty" json:"level,omitempty"`
	// Group 消息分组，便于在 Bark App 里折叠管理
	Group string `yaml:"group,omitempty" json:"group,omitempty"`
	// Icon 推送图标 URL
	Icon string `yaml:"icon,omitempty" json:"icon,omitempty"`
	// JumpURL 点击推送时跳转的地址
	JumpURL string `yaml:"jump_url,omitempty" json:"jump_url,omitempty"`
	// Copy 点击推送时复制的内容
	Copy string `yaml:"copy,omitempty" json:"copy,omitempty"`
	// AutoCopy 是否自动复制 Copy 内容
	AutoCopy bool `yaml:"auto_copy,omitempty" json:"auto_copy,omitempty"`
	// IsArchive 是否保存到 Bark 历史列表；nil 表示跟随 App 设置
	IsArchive *bool `yaml:"is_archive,omitempty" json:"is_archive,omitempty"`
}

type BarkBody struct {
	Title      *string `json:"title,omitempty"`
	Body       *string `json:"body,omitempty"`
	Level      *string `json:"level,omitempty"`
	Badge      *int    `json:"badge,omitempty"`
	AutoCopy   *int    `json:"autoCopy,omitempty"`
	Copy       *string `json:"copy,omitempty"`
	Sound      *string `json:"sound,omitempty"`
	Icon       *string `json:"icon,omitempty"`
	Group      *string `json:"group,omitempty"`
	IsArchive  *int    `json:"isArchive,omitempty"`
	Url        *string `json:"url,omitempty"`
	CipherText *string `json:"cipherText,omitempty"`
}

type BarkClient struct {
	channel map[string]interface{}

	logger *slog.Logger
}

func NewBarkClient(channel map[string]interface{}, logger *slog.Logger) *BarkClient {
	client := &BarkClient{
		channel: channel,
		logger:  logger,
	}

	return client
}

func (c *BarkClient) Parse(data map[string]interface{}) (*BarkConfig, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var bc BarkConfig
	if err = json.Unmarshal(body, &bc); err != nil {
		return nil, err
	}
	return &bc, nil
}

// Validate 检查渠道配置的完整性，配置保存与测试发送前调用。
func (c *BarkConfig) Validate(name string) error {
	if c.Url == "" {
		return fmt.Errorf("渠道 %s 缺少推送地址（url）", name)
	}
	if u, err := url.Parse(c.Url); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("渠道 %s 的推送地址（url）不是合法的 http(s) 链接", name)
	}
	hasKey, hasIV := c.AesKey != nil && *c.AesKey != "", c.AesIV != nil && *c.AesIV != ""
	if hasKey != hasIV {
		return fmt.Errorf("渠道 %s 的 aes_key 与 aes_iv 需同时设置", name)
	}
	if hasKey {
		if l := len(*c.AesKey); l != 16 && l != 24 && l != 32 {
			return fmt.Errorf("渠道 %s 的 aes_key 长度必须为 16/24/32 字节，当前 %d 字节", name, l)
		}
		if l := len(*c.AesIV); l != 16 {
			return fmt.Errorf("渠道 %s 的 aes_iv 长度必须为 16 字节，当前 %d 字节", name, l)
		}
	}
	switch c.Level {
	case "", "auto", "active", "timeSensitive", "passive":
	default:
		return fmt.Errorf("渠道 %s 的 level 只能为 auto/active/timeSensitive/passive", name)
	}
	return nil
}

// levelFromPriority 按消息优先级推导时效性等级：
// >=8 重要（timeSensitive），>=1 常规（active），<=0 低优先级（passive，静默投递）。
func levelFromPriority(priority int) string {
	switch {
	case priority >= 8:
		return "timeSensitive"
	case priority >= 1:
		return "active"
	default:
		return "passive"
	}
}

func (c *BarkClient) SendMessage(message plugin.Message) error {
	conf, err := c.Parse(c.channel)
	if err != nil {
		return err
	}
	if conf.Url == "" {
		return errors.New("url is not set")
	}
	barkBody := &BarkBody{
		Title: &message.Title,
		Body:  &message.Message,
	}
	if conf.Sound != "" {
		barkBody.Sound = &conf.Sound
	}
	if conf.Group != "" {
		barkBody.Group = &conf.Group
	}
	if conf.Icon != "" {
		barkBody.Icon = &conf.Icon
	}
	if conf.JumpURL != "" {
		barkBody.Url = &conf.JumpURL
	}
	if conf.Copy != "" {
		barkBody.Copy = &conf.Copy
		if conf.AutoCopy {
			one := 1
			barkBody.AutoCopy = &one
		}
	}
	if conf.IsArchive != nil {
		isArchive := 0
		if *conf.IsArchive {
			isArchive = 1
		}
		barkBody.IsArchive = &isArchive
	}
	// level 默认按优先级推导，显式 passive 等配置优先
	level := conf.Level
	if level == "" || level == "auto" {
		level = levelFromPriority(message.Priority)
	}
	barkBody.Level = &level

	if conf.AesKey != nil && *conf.AesKey != "" && conf.AesIV != nil && *conf.AesIV != "" {
		body, err := json.Marshal(barkBody)
		if err != nil {
			return err
		}
		key := []byte(*conf.AesKey)
		iv := []byte(*conf.AesIV)
		cipherText, err := AesEncrypt(body, key, iv)
		if err != nil {
			return err
		}
		cipherTextStr := EncodeToString(cipherText)
		barkBody = &BarkBody{CipherText: &cipherTextStr}
	}
	body, err := json.Marshal(barkBody)
	if err != nil {
		return err
	}
	resp, err := HTTPClient.Post(conf.Url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer func() {
		if e := resp.Body.Close(); e != nil {
			c.logger.Error("close body failed", "error", e)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("response status is not 200(%d)", resp.StatusCode)
	}
	if body, err = io.ReadAll(resp.Body); err != nil {
		return err
	}
	var response *BarkResponse
	if err = json.Unmarshal(body, &response); err != nil {
		return err
	}
	if response.Code != 200 {
		c.logger.With(slog.String("response", string(body))).Error("response code is not 200")
		return fmt.Errorf("response code is not 200(%d), %s", response.Code, response.Message)
	}

	return nil
}

type BarkResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Timestamp int    `json:"timestamp"`
}

func Encode(src []byte) []byte {
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(src)))
	base64.StdEncoding.Encode(dst, src)
	return dst
}

func EncodeToString(src []byte) string {
	return string(Encode(src))
}

func AesEncrypt(plaintext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	plaintext = PKCS7Padding(plaintext, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, iv[:blockSize])
	ciphertext := make([]byte, len(plaintext))
	blockMode.CryptBlocks(ciphertext, plaintext)
	return ciphertext, nil
}

func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}
