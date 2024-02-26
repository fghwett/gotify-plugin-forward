package channels

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gotify/plugin-api"
	"io"
	"log/slog"
	"net/http"
)

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

func (c *BarkClient) SendMessage(message plugin.Message) error {
	url, ok := c.channel["url"]
	if !ok {
		return errors.New("url is not set")
	}
	u, o := url.(string)
	if !o {
		return errors.New("url is not a string")
	}
	body, err := json.Marshal(&BarkBody{
		Title: &message.Title,
		Body:  &message.Message,
	})
	if err != nil {
		return err
	}
	var resp *http.Response
	if resp, err = http.Post(u, "application/json", bytes.NewReader(body)); err != nil {
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
