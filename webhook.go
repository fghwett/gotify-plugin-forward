package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/gotify/plugin-api"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// RegisterWebhook implements plugin.Webhooker.
func (c *MyPlugin) RegisterWebhook(basePath string, g *gin.RouterGroup) {
	c.logger.With("base_path", basePath).Info("register webhook")
	c.basePath = basePath

	g.Match([]string{http.MethodGet, http.MethodPost}, "/", c.Message)
	g.Match([]string{http.MethodGet, http.MethodPost}, "/message", c.Message)
}

type MessageExternal struct {
	ID            uint                   `json:"id"`
	ApplicationID uint                   `json:"appid"`
	Message       string                 `form:"message" query:"message" json:"message" binding:"required"`
	Title         string                 `form:"title" query:"title" json:"title"`
	Priority      int                    `form:"priority" query:"priority" json:"priority"`
	Extras        map[string]interface{} `form:"-" query:"-" json:"extras,omitempty"`
	Date          time.Time              `json:"date"`
}

func (c *MyPlugin) Message(ctx *gin.Context) {
	message, err := c.getMessage(ctx)
	if err != nil {
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	messageHandler := c.getSendMessageHandler(ctx)

	if err = messageHandler.SendMessage(*message); err != nil {
		if errors.Is(err, &Result{}) {
			_ = ctx.AbortWithError(err.(*Result).Code, err)
			return
		}
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// 额外推送消息
	if err = c.sendExtraMessage(c.getToken(ctx), *message); err != nil {
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, Result{
		Code:    0,
		Message: "success",
	})
}

func (c *MyPlugin) getMessage(ctx *gin.Context) (*plugin.Message, error) {
	// 获取消息本体
	message := &MessageExternal{}
	if err := ctx.Bind(message); err != nil {
		return nil, err
	}
	if message == nil {
		return nil, errors.New("message is nil")
	}

	if message.Title == "" && message.Message == "" {
		return nil, errors.New("title and message are empty")
	}

	if message.Title == "" {
		message.Title = "Empty Title"
	}
	if message.Message == "" {
		message.Message = message.Title
	}

	m := &plugin.Message{
		Message:  message.Message,
		Title:    message.Title,
		Priority: message.Priority,
		Extras:   message.Extras,
	}

	return m, nil
}

func (c *MyPlugin) getSendMessageHandler(ctx *gin.Context) plugin.MessageHandler {
	if c.getToken(ctx) == nil {
		return c.messageHandler
	}

	return NewMessageHandler(ctx, c.logger)
}

func (c *MyPlugin) getToken(ctx *gin.Context) *string {
	token, ok := ctx.GetQuery("token")
	if !ok || token == "" {
		return nil
	}
	return &token
}

type MessageHandler struct {
	ctx    *gin.Context
	logger *slog.Logger
}

func NewMessageHandler(ctx *gin.Context, logger *slog.Logger) *MessageHandler {
	return &MessageHandler{
		ctx:    ctx,
		logger: logger,
	}
}

func (c *MessageHandler) SendMessage(message plugin.Message) error {
	body, err := json.Marshal(&message)
	if err != nil {
		return err
	}

	source := c.ctx.Request.URL
	to, _ := url.Parse("http://127.0.0.1:80")
	to.Path = "/message"
	to.RawQuery = source.RawQuery

	var resp *http.Response
	if resp, err = http.Post(to.String(), `application/json`, bytes.NewReader(body)); err != nil {
		return err
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			c.logger.Error("close response error:", err)
		}
	}()
	if body, err = io.ReadAll(resp.Body); err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("response error:", string(body))
		return &Result{
			Code:    resp.StatusCode,
			Message: string(body),
		}
	}

	return nil
}
