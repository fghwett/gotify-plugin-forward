package channels

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gotify/plugin-api"
	"io"
	"log/slog"
	"net/http"
)

type BarkConfig struct {
	Url    *string `json:"url,omitempty"`
	AesKey *string `json:"aes_key,omitempty"`
	AesIV  *string `json:"aes_iv,omitempty"`
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

func (c *BarkClient) SendMessage(message plugin.Message) error {
	conf, err := c.Parse(c.channel)
	if err != nil {
		return err
	}
	if conf.Url == nil {
		return errors.New("url is not set")
	}
	barkBody := &BarkBody{
		Title: &message.Title,
		Body:  &message.Message,
	}
	if conf.AesKey != nil && conf.AesIV != nil {
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
		barkBody.CipherText = &cipherTextStr
	}
	body, err := json.Marshal(barkBody)
	if err != nil {
		return err
	}
	var resp *http.Response
	if resp, err = http.Post(*conf.Url, "application/json", bytes.NewReader(body)); err != nil {
		return err
	}
	defer func() {
		if e := resp.Body.Close(); e != nil {
			c.logger.Error("close body failed", e)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return errors.New("response status is not 200")
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
