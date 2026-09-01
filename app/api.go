package app

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gotify/plugin-api"

	"github.com/fghwett/gotify-plugin-forward/channels"
	"github.com/fghwett/gotify-plugin-forward/consts"
)

// handleAuthStatus 返回 passkey 设置与登录状态，供页面决定展示哪个视图。
func (a *App) handleAuthStatus(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"configured":    a.store.Passkey() != nil,
		"authenticated": a.authenticated(ctx),
	})
}

// handleRegisterBegin 发起 passkey 注册，仅在尚未设置 passkey 时允许（首次打开设置）。
func (a *App) handleRegisterBegin(ctx *gin.Context) {
	if a.store.Passkey() != nil {
		ctx.AbortWithStatusJSON(http.StatusConflict, Result{Code: http.StatusConflict, Message: "passkey 已设置，如需重置请使用 reset_passkey"})
		return
	}

	instance, err := a.webauthn.webauthnFor(ctx)
	if err != nil {
		a.abortInternal(ctx, err)
		return
	}
	// user handle 首次生成后固定，随凭据一起持久化
	handle := randomBytes(32)
	options, session, err := instance.BeginRegistration(&passkeyUser{name: a.user.Name, handle: handle})
	if err != nil {
		a.abortInternal(ctx, err)
		return
	}
	sessionID, err := a.webauthn.putChallenge(*session)
	if err != nil {
		a.abortInternal(ctx, err)
		return
	}
	a.registeringHandle = handle

	ctx.JSON(http.StatusOK, gin.H{"session_id": sessionID, "options": options})
}

// handleRegisterFinish 校验注册结果并保存凭据，成功即视为登录。
func (a *App) handleRegisterFinish(ctx *gin.Context) {
	if a.store.Passkey() != nil {
		ctx.AbortWithStatusJSON(http.StatusConflict, Result{Code: http.StatusConflict, Message: "passkey 已设置"})
		return
	}
	session, ok := a.popChallengeFromRequest(ctx)
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, Result{Code: http.StatusBadRequest, Message: "注册会话不存在或已过期"})
		return
	}
	instance, err := a.webauthn.webauthnFor(ctx)
	if err != nil {
		a.abortInternal(ctx, err)
		return
	}
	user := &passkeyUser{name: a.user.Name, handle: a.registeringHandle}
	credential, err := instance.FinishRegistration(user, session, ctx.Request)
	if err != nil {
		a.logger.Error("passkey 注册校验失败", "error", err)
		ctx.AbortWithStatusJSON(http.StatusBadRequest, Result{Code: http.StatusBadRequest, Message: ceremonyError(err)})
		return
	}
	if err = a.store.SavePasskey(&PasskeyData{
		UserID:      user.WebAuthnID(),
		Credentials: []webauthn.Credential{*credential},
	}); err != nil {
		a.abortInternal(ctx, err)
		return
	}
	a.registeringHandle = nil
	a.issueSession(ctx)
	ctx.JSON(http.StatusOK, Result{Code: 0, Message: "success"})
}

// handleLoginBegin 发起 passkey 认证。
func (a *App) handleLoginBegin(ctx *gin.Context) {
	data := a.store.Passkey()
	if data == nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, Result{Code: http.StatusNotFound, Message: "尚未设置 passkey"})
		return
	}
	instance, err := a.webauthn.webauthnFor(ctx)
	if err != nil {
		a.abortInternal(ctx, err)
		return
	}
	options, session, err := instance.BeginLogin(&passkeyUser{data: data, name: a.user.Name})
	if err != nil {
		a.abortInternal(ctx, err)
		return
	}
	sessionID, err := a.webauthn.putChallenge(*session)
	if err != nil {
		a.abortInternal(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"session_id": sessionID, "options": options})
}

// handleLoginFinish 校验签名，成功后签发会话 cookie。
func (a *App) handleLoginFinish(ctx *gin.Context) {
	data := a.store.Passkey()
	if data == nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, Result{Code: http.StatusNotFound, Message: "尚未设置 passkey"})
		return
	}
	session, ok := a.popChallengeFromRequest(ctx)
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, Result{Code: http.StatusBadRequest, Message: "登录会话不存在或已过期"})
		return
	}
	instance, err := a.webauthn.webauthnFor(ctx)
	if err != nil {
		a.abortInternal(ctx, err)
		return
	}
	user := &passkeyUser{data: data, name: a.user.Name}
	credential, err := instance.FinishLogin(user, session, ctx.Request)
	if err != nil {
		a.logger.Error("passkey 登录校验失败", "error", err)
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, Result{Code: http.StatusUnauthorized, Message: ceremonyError(err)})
		return
	}
	// 回写凭据（含更新后的签名计数器）
	if err = a.store.SavePasskey(&PasskeyData{UserID: user.WebAuthnID(), Credentials: []webauthn.Credential{*credential}}); err != nil {
		a.abortInternal(ctx, err)
		return
	}
	a.issueSession(ctx)
	ctx.JSON(http.StatusOK, Result{Code: 0, Message: "success"})
}

func (a *App) handleLogout(ctx *gin.Context) {
	// 浏览器可能持有多个同名 cookie（历史 Path 变体），全部失效
	for _, cookie := range ctx.Request.Cookies() {
		if cookie.Name == sessionCookie {
			a.webauthn.dropSession(cookie.Value)
		}
	}
	a.setSessionCookie(ctx, "")
	ctx.JSON(http.StatusOK, Result{Code: 0, Message: "success"})
}

// handleConfigGet 返回当前生效的完整配置（需登录）。
func (a *App) handleConfigGet(ctx *gin.Context) {
	conf := a.effectiveConfig()
	if conf == nil {
		conf = &Config{Version: ConfigVersion}
	}
	ctx.JSON(http.StatusOK, conf)
}

// handleConfigSave 保存可视化编辑的配置（需登录）。
func (a *App) handleConfigSave(ctx *gin.Context) {
	conf := &Config{}
	if err := ctx.BindJSON(conf); err != nil {
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if err := conf.Validate(); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, Result{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	if err := a.store.SaveConfig(conf); err != nil {
		a.abortInternal(ctx, err)
		return
	}
	a.logger.Info("可视化配置已保存")
	ctx.JSON(http.StatusOK, Result{Code: 0, Message: "success"})
}

// requireSession 是配置读写接口的登录校验中间件。
func (a *App) requireSession(ctx *gin.Context) {
	if !a.authenticated(ctx) {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, Result{Code: http.StatusUnauthorized, Message: "未登录或会话已过期"})
		return
	}
	ctx.Next()
}

// ceremonyError 提取 WebAuthn 仪式失败的可读信息：
// protocol.Error 的 DevInfo 经常为空，回落到完整错误描述，避免出现「401 空消息」。
func ceremonyError(err error) string {
	var protocolErr *protocol.Error
	if errors.As(err, &protocolErr) && protocolErr.DevInfo != "" {
		return protocolErr.DevInfo
	}
	return err.Error()
}

// handleTestSend 用提交的渠道配置（无需先保存）发送一条测试消息，
// 便于在配置页即时验证推送地址与参数是否可用。
func (a *App) handleTestSend(ctx *gin.Context) {
	var body struct {
		Channel map[string]interface{} `json:"channel" binding:"required"`
	}
	if err := ctx.BindJSON(&body); err != nil {
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	name := ctx.Query("name")
	if name == "" {
		name = "未命名渠道"
	}
	typeVal, _ := body.Channel[RuleTypeKey].(string)
	if RuleType(typeVal) != RuleTypeBark {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, Result{Code: http.StatusBadRequest, Message: "仅支持测试 bark 类型渠道"})
		return
	}
	client := channels.NewBarkClient(body.Channel, a.logger)
	conf, err := client.Parse(body.Channel)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, Result{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}
	if err = conf.Validate(name); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, Result{Code: http.StatusBadRequest, Message: err.Error()})
		return
	}

	begin := time.Now()
	err = a.sendToChannel(body.Channel, testMessage())
	ctx.JSON(http.StatusOK, gin.H{
		"ok":      err == nil,
		"error":   errText(err),
		"cost_ms": time.Since(begin).Milliseconds(),
	})
}

// handleLogsGet 返回投递日志（新→旧），供配置页日志标签展示。
func (a *App) handleLogsGet(ctx *gin.Context) {
	logs := a.store.Logs()
	out := make([]DeliveryLog, 0, len(logs))
	for i := len(logs) - 1; i >= 0; i-- {
		out = append(out, logs[i])
	}
	ctx.JSON(http.StatusOK, gin.H{"logs": out, "max": MaxDeliveryLogs})
}

// handleLogsClear 清空投递日志。
func (a *App) handleLogsClear(ctx *gin.Context) {
	if err := a.store.ClearLogs(); err != nil {
		a.abortInternal(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, Result{Code: 0, Message: "success"})
}

// handleMeta 返回插件元信息，展示在「关于」标签。
func (a *App) handleMeta(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"version": consts.PluginVersion,
		"name":    consts.PluginName,
	})
}

func testMessage() plugin.Message {
	return plugin.Message{
		Title:    "gotify-plugin-forward 测试消息",
		Message:  "如果你看到这条推送，说明该渠道配置可用。",
		Priority: 5,
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// authenticated 校验会话 cookie。
// 历史版本曾以带尾斜杠的 Path 写入同名 cookie，浏览器会按「长路径优先」
// 同时发送新旧两个值，而 Go 的 Cookie() 只取第一个（旧值），因此这里
// 逐个校验，任一有效即通过，让存量浏览器无需手动清 cookie 即可自愈。
func (a *App) authenticated(ctx *gin.Context) bool {
	for _, cookie := range ctx.Request.Cookies() {
		if cookie.Name == sessionCookie && a.webauthn.validSession(cookie.Value) {
			return true
		}
	}
	return false
}

func (a *App) issueSession(ctx *gin.Context) {
	token, err := a.webauthn.newSession()
	if err != nil {
		a.abortInternal(ctx, err)
		return
	}
	a.setSessionCookie(ctx, token)
}

func (a *App) setSessionCookie(ctx *gin.Context, value string) {
	base := strings.TrimSuffix(pageBasePath(ctx), "/")
	if base == "" {
		base = "/"
	}
	maxAge := int(sessionTTL.Seconds())
	if value == "" {
		maxAge = -1
	}
	secure := requestSchemeOf(ctx) == "https"
	ctx.SetCookie(sessionCookie, value, maxAge, base, "", secure, true)
	// 顺手清除历史尾斜杠 Path 的同名 cookie，避免新旧值并存干扰
	if base != "/" {
		ctx.SetCookie(sessionCookie, "", -1, base+"/", "", secure, true)
	}
}

func (a *App) popChallengeFromRequest(ctx *gin.Context) (webauthn.SessionData, bool) {
	return a.webauthn.popChallenge(ctx.GetHeader("X-Session-Id"))
}

func (a *App) abortInternal(ctx *gin.Context, err error) {
	_ = ctx.AbortWithError(http.StatusInternalServerError, err)
}
