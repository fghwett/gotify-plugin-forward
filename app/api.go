package app

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
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
		var protocolErr *protocol.Error
		if errors.As(err, &protocolErr) {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, Result{Code: http.StatusBadRequest, Message: protocolErr.DevInfo})
			return
		}
		a.abortInternal(ctx, err)
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
		var protocolErr *protocol.Error
		if errors.As(err, &protocolErr) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, Result{Code: http.StatusUnauthorized, Message: protocolErr.DevInfo})
			return
		}
		a.abortInternal(ctx, err)
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
	if token, err := ctx.Cookie(sessionCookie); err == nil {
		a.webauthn.dropSession(token)
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

func (a *App) authenticated(ctx *gin.Context) bool {
	token, err := ctx.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return a.webauthn.validSession(token)
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
	maxAge := int(sessionTTL.Seconds())
	if value == "" {
		maxAge = -1
	}
	ctx.SetCookie(sessionCookie, value, maxAge, pageBasePath(ctx), "", requestSchemeOf(ctx) == "https", true)
}

func (a *App) popChallengeFromRequest(ctx *gin.Context) (webauthn.SessionData, bool) {
	return a.webauthn.popChallenge(ctx.GetHeader("X-Session-Id"))
}

func (a *App) abortInternal(ctx *gin.Context, err error) {
	_ = ctx.AbortWithError(http.StatusInternalServerError, err)
}
