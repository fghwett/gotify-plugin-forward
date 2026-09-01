package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fghwett/gotify-plugin-forward/channels"
	"github.com/gin-gonic/gin"
	"github.com/gotify/plugin-api"
)

// RegisterWebhook implements plugin.Webhooker.
func (a *App) RegisterWebhook(basePath string, g *gin.RouterGroup) {
	a.logger.With("base_path", basePath).Info("register webhook")
	// gotify 传入的 basePath 带尾部斜杠，规整为无尾斜杠形式，
	// 避免拼接出 "//config" 这类双斜杠地址（相对路径请求会跟着解析错）
	a.basePath = "/" + strings.Trim(basePath, "/")

	g.Match([]string{http.MethodGet, http.MethodPost}, "/message", a.Message)
	g.POST("/", a.Message)
	// 裸路径 GET：兼容原版「GET /?message=…」推送；无消息参数时展示配置页面
	g.GET("/", a.handleRoot)

	// 可视化配置页面与配套 API（passkey 保护）
	g.GET("/config", a.handleConfigPage)
	g.GET("/assets/*filepath", a.handleConfigAssets)
	api := g.Group("/api")
	{
		api.GET("/auth/status", a.handleAuthStatus)
		api.POST("/auth/register/begin", a.handleRegisterBegin)
		api.POST("/auth/register/finish", a.handleRegisterFinish)
		api.POST("/auth/login/begin", a.handleLoginBegin)
		api.POST("/auth/login/finish", a.handleLoginFinish)
		api.POST("/auth/logout", a.handleLogout)
		api.GET("/config", a.requireSession, a.handleConfigGet)
		api.POST("/config", a.requireSession, a.handleConfigSave)
		api.POST("/test", a.requireSession, a.handleTestSend)
		api.GET("/logs", a.requireSession, a.handleLogsGet)
		api.DELETE("/logs", a.requireSession, a.handleLogsClear)
		api.GET("/meta", a.requireSession, a.handleMeta)
	}
}

// handleRoot 区分「GET / 推送消息」与「打开配置页面」两种用法。
func (a *App) handleRoot(ctx *gin.Context) {
	if ctx.Query("message") != "" {
		a.Message(ctx)
		return
	}
	a.handleConfigPage(ctx)
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

func (a *App) Message(ctx *gin.Context) {
	message, err := a.getMessage(ctx)
	if err != nil {
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	messageHandler := a.getSendMessageHandler(ctx)

	if err = messageHandler.SendMessage(*message); err != nil {
		var result *Result
		if errors.As(err, &result) {
			_ = ctx.AbortWithError(result.Code, err)
			return
		}
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// 额外推送：异步执行，失败重试并记录投递日志，不影响本接口响应
	a.sendExtraMessage(a.getToken(ctx), *message)

	ctx.JSON(http.StatusOK, Result{
		Code:    0,
		Message: "success",
	})
}

func (a *App) getMessage(ctx *gin.Context) (*plugin.Message, error) {
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

func (a *App) getSendMessageHandler(ctx *gin.Context) plugin.MessageHandler {
	if a.getToken(ctx) == nil {
		return a.messageHandler
	}

	return NewMessageHandler(ctx, a.logger)
}

func (a *App) getToken(ctx *gin.Context) *string {
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

// requestScheme 从请求推断 gotify 对外访问协议，优先信任反代传递的 X-Forwarded-Proto
func (c *MessageHandler) requestScheme() string {
	if proto := c.ctx.GetHeader("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if c.ctx.Request.TLS != nil {
		return "https"
	}
	return "http"
}

func (c *MessageHandler) SendMessage(message plugin.Message) error {
	body, err := json.Marshal(&message)
	if err != nil {
		return err
	}

	source := c.ctx.Request.URL
	to := &url.URL{
		Scheme:   c.requestScheme(),
		Host:     c.ctx.Request.Host,
		Path:     "/message",
		RawQuery: source.RawQuery,
	}

	resp, err := channels.HTTPClient.Post(to.String(), `application/json`, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			c.logger.Error("close response error", "error", err)
		}
	}()
	if body, err = io.ReadAll(resp.Body); err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("response error", "body", string(body))
		return &Result{
			Code:    resp.StatusCode,
			Message: string(body),
		}
	}

	return nil
}
