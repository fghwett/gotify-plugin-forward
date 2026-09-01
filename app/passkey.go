package app

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	challengeTTL  = 90 * time.Second
	sessionTTL    = 7 * 24 * time.Hour
	sessionCookie = "forward_session"
)

// passkeyUser 适配 webauthn.User 接口，数据来自 store 中的 PasskeyData。
type passkeyUser struct {
	data   *PasskeyData
	name   string
	handle []byte
}

func (u *passkeyUser) WebAuthnID() []byte {
	if len(u.handle) != 0 {
		return u.handle
	}
	// 登录时 handle 为空，必须返回持久化保存的 UserID：
	// go-webauthn 会把它与认证器返回的 userHandle 严格比对，回落到用户名会导致校验永远失败
	if u.data != nil && len(u.data.UserID) != 0 {
		return u.data.UserID
	}
	// 兼容历史数据缺 handle 的场景，回落到用户名
	return []byte(u.name)
}

func (u *passkeyUser) WebAuthnName() string        { return u.name }
func (u *passkeyUser) WebAuthnDisplayName() string { return u.name }
func (u *passkeyUser) WebAuthnCredentials() []webauthn.Credential {
	if u.data == nil {
		return nil
	}
	return u.data.Credentials
}

// webauthnState 汇总一个插件实例的认证运行时状态。
type webauthnState struct {
	mu         sync.Mutex
	instances  map[string]*webauthn.WebAuthn // key: rpid+"\x00"+origin
	challenges map[string]challengeEntry
	sessions   map[string]time.Time
}

type challengeEntry struct {
	data   webauthn.SessionData
	expire time.Time
}

func newWebauthnState() *webauthnState {
	return &webauthnState{
		instances:  map[string]*webauthn.WebAuthn{},
		challenges: map[string]challengeEntry{},
		sessions:   map[string]time.Time{},
	}
}

// webauthnFor 按请求推断 RP 信息并返回（带缓存的）WebAuthn 实例。
// 插件页面运行在 gotify 域名下，RP ID 即请求域名。
func (w *webauthnState) webauthnFor(ctx *gin.Context) (*webauthn.WebAuthn, error) {
	scheme := requestSchemeOf(ctx)
	host := forwardedHostOf(ctx)

	rpid := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		rpid = h
	}
	origin := scheme + "://" + host

	w.mu.Lock()
	defer w.mu.Unlock()

	key := rpid + "\x00" + origin
	if instance, ok := w.instances[key]; ok {
		return instance, nil
	}
	instance, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Gotify Forward",
		RPID:          rpid,
		RPOrigins:     []string{origin},
	})
	if err != nil {
		return nil, err
	}
	w.instances[key] = instance
	return instance, nil
}

func (w *webauthnState) putChallenge(data webauthn.SessionData) (string, error) {
	id, err := randomID()
	if err != nil {
		return "", err
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	w.cleanupLocked()
	w.challenges[id] = challengeEntry{data: data, expire: time.Now().Add(challengeTTL)}
	return id, nil
}

func (w *webauthnState) popChallenge(id string) (webauthn.SessionData, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry, ok := w.challenges[id]
	if !ok {
		return webauthn.SessionData{}, false
	}
	delete(w.challenges, id)
	if time.Now().After(entry.expire) {
		return webauthn.SessionData{}, false
	}
	return entry.data, true
}

func (w *webauthnState) newSession() (string, error) {
	token, err := randomID()
	if err != nil {
		return "", err
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	w.cleanupLocked()
	w.sessions[token] = time.Now().Add(sessionTTL)
	return token, nil
}

func (w *webauthnState) validSession(token string) bool {
	if token == "" {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	expire, ok := w.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(expire) {
		delete(w.sessions, token)
		return false
	}
	return true
}

func (w *webauthnState) dropSession(token string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.sessions, token)
}

func (w *webauthnState) cleanupLocked() {
	now := time.Now()
	for id, entry := range w.challenges {
		if now.After(entry.expire) {
			delete(w.challenges, id)
		}
	}
	for token, expire := range w.sessions {
		if now.After(expire) {
			delete(w.sessions, token)
		}
	}
}

func randomID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomBytes(n int) []byte {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return nil
	}
	return buf
}

// requestSchemeOf 与 forwardedHostOf 是 MessageHandler.requestScheme 的公共版本，
// 认证与转发共用同一套反代推断逻辑。
func requestSchemeOf(ctx *gin.Context) string {
	if proto := ctx.GetHeader("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if ctx.Request.TLS != nil {
		return "https"
	}
	return "http"
}

func forwardedHostOf(ctx *gin.Context) string {
	if host := ctx.GetHeader("X-Forwarded-Host"); host != "" {
		return host
	}
	return ctx.Request.Host
}

// pageBasePath 从请求 URL 推断插件页面所在的根路径，
// 用于设置 cookie 的作用范围，避免会话 cookie 泄露给 gotify 其他路由。
func pageBasePath(ctx *gin.Context) string {
	path := ctx.Request.URL.Path
	if idx := lastIndex(path, "/api/"); idx >= 0 {
		return path[:idx]
	}
	return "/"
}

func lastIndex(s, sub string) int {
	for i := len(s) - len(sub); i >= 0; i-- {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
