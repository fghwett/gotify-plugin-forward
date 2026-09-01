package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gotify/plugin-api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memStorage struct{ data []byte }

func (m *memStorage) Save(b []byte) error   { m.data = b; return nil }
func (m *memStorage) Load() ([]byte, error) { return m.data, nil }

func newTestApp(t *testing.T, storage *memStorage) *App {
	t.Helper()
	gin.SetMode(gin.TestMode)
	a := New(plugin.UserContext{ID: 1, Name: "tester", Admin: true})
	a.SetStorageHandler(storage)
	return a
}

func TestStoreRoundtrip(t *testing.T) {
	storage := &memStorage{}
	a := newTestApp(t, storage)

	require.Nil(t, a.store.Config())
	require.Nil(t, a.store.Passkey())

	conf := &Config{Version: ConfigVersion, Channels: map[string]map[string]interface{}{
		"b1": {"type": "bark", "url": "https://example.com/t"},
	}}
	require.NoError(t, a.store.SaveConfig(conf))
	require.NoError(t, a.store.SavePasskey(&PasskeyData{UserID: []byte("u1")}))

	// 用同一存储重建实例，验证数据确实持久化
	b := newTestApp(t, storage)
	require.NotNil(t, b.store.Config())
	assert.Equal(t, conf.Channels["b1"]["url"], b.store.Config().Channels["b1"]["url"])
	require.NotNil(t, b.store.Passkey())
	assert.Equal(t, []byte("u1"), b.store.Passkey().UserID)

	require.NoError(t, b.store.ClearPasskey())
	assert.Nil(t, b.store.Passkey())
}

func TestEffectiveConfigPriority(t *testing.T) {
	a := newTestApp(t, &memStorage{})

	// 存储与原生配置都为空
	assert.Nil(t, a.effectiveConfig())

	// 仅有原生配置
	a.config = &Config{Version: "1"}
	assert.Equal(t, a.config, a.effectiveConfig())

	// 存储配置优先于原生配置
	stored := &Config{Version: "1"}
	require.NoError(t, a.store.SaveConfig(stored))
	assert.Equal(t, stored, a.effectiveConfig())
}

func TestConfigValidate(t *testing.T) {
	conf := &Config{
		Channels: map[string]map[string]interface{}{"b1": {"type": "bark", "url": "https://example.com/t"}},
		Rules:    map[string][]*Rule{"all": {{Channel: "missing", Enabled: true}}},
	}
	assert.ErrorContains(t, conf.Validate(), "不存在的渠道")

	conf.Rules["all"] = []*Rule{{Channel: "b1", Enabled: true}}
	assert.NoError(t, conf.Validate())
}

func TestResetPasskeyTrigger(t *testing.T) {
	a := newTestApp(t, &memStorage{})
	require.NoError(t, a.store.SavePasskey(&PasskeyData{UserID: []byte("u1")}))

	require.NoError(t, a.ValidateAndSetConfig(&Config{Version: "1", ResetPasskey: true}))
	assert.Nil(t, a.store.Passkey(), "reset_passkey 应清除已保存的凭据")
}

func newTestRouter(a *App) (*gin.Engine, func() string) {
	r := gin.New()
	base := "/plugin/1/custom/testtoken"
	a.RegisterWebhook(base, r.Group(base))
	return r, func() string { return base }
}

func doReq(t *testing.T, method, url string, headers map[string]string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, url, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestAuthStatusAndConfigGuard(t *testing.T) {
	a := newTestApp(t, &memStorage{})
	r, base := newTestRouter(a)
	server := httptest.NewServer(r)
	defer server.Close()
	url := server.URL + base()

	// 未设置 passkey 的初始状态
	resp := doReq(t, http.MethodGet, url+"/api/auth/status", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// 未登录时配置接口必须拒绝
	resp = doReq(t, http.MethodGet, url+"/api/config", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// 未设置 passkey 时不允许发起登录
	resp = doReq(t, http.MethodPost, url+"/api/auth/login/begin", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// 已设置 passkey 后不允许重复注册
	require.NoError(t, a.store.SavePasskey(&PasskeyData{UserID: []byte("u1")}))
	resp = doReq(t, http.MethodPost, url+"/api/auth/register/begin", nil)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestRegisterFinishRejectsBadSession(t *testing.T) {
	a := newTestApp(t, &memStorage{})
	r, base := newTestRouter(a)
	server := httptest.NewServer(r)
	defer server.Close()

	resp := doReq(t, http.MethodPost, server.URL+base()+"/api/auth/register/finish",
		map[string]string{"X-Session-Id": "not-exists"})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// basePath 带尾斜杠注册时应规整为无尾斜杠形式，避免拼出 "//config" 双斜杠地址
func TestBasePathNormalization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	a := New(plugin.UserContext{ID: 1, Name: "tester", Admin: true})
	a.SetStorageHandler(&memStorage{})

	r := gin.New()
	a.RegisterWebhook("/plugin/1/custom/tok/", r.Group("/plugin/1/custom/tok/"))
	assert.Equal(t, "/plugin/1/custom/tok", a.basePath)

	display := a.GetDisplay(nil)
	assert.NotContains(t, display, "//", "展示链接不应包含双斜杠")
}

func TestRootRouting(t *testing.T) {
	a := newTestApp(t, &memStorage{})
	r, base := newTestRouter(a)
	server := httptest.NewServer(r)
	defer server.Close()
	url := server.URL + base()

	// GET 裸路径且无 message 参数 → 配置页面（源码环境缺 index.html 时为 500，但不应 404）
	resp := doReq(t, http.MethodGet, url, nil)
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)

	// GET /config 已按需求移除
	resp = doReq(t, http.MethodGet, url+"/config", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// POST 裸路径仍是消息接口：缺 message 参数应 400
	resp = doReq(t, http.MethodPost, url, nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// 登录构造的 passkeyUser（仅 data）必须返回持久化的 UserID，
// 否则 go-webauthn 的 userHandle 比对永远失败（登录必 401）
func TestPasskeyUserWebAuthnID(t *testing.T) {
	saved := []byte("0123456789abcdef0123456789abcdef")

	login := &passkeyUser{data: &PasskeyData{UserID: saved}, name: "fghwett"}
	assert.Equal(t, saved, login.WebAuthnID(), "登录场景应返回存储的 UserID 而非用户名")

	registering := &passkeyUser{handle: saved, name: "fghwett"}
	assert.Equal(t, saved, registering.WebAuthnID())

	legacy := &passkeyUser{name: "fghwett"}
	assert.Equal(t, []byte("fghwett"), legacy.WebAuthnID())
}

// 浏览器对同名不同 Path 的 cookie 按「长路径优先」排序发送，Go 的
// Cookie() 只取第一个（历史尾斜杠 Path 的旧值），认证必须逐个校验
func TestAuthenticatedWithDuplicateCookies(t *testing.T) {
	a := newTestApp(t, &memStorage{})
	r, base := newTestRouter(a)
	server := httptest.NewServer(r)
	defer server.Close()

	valid, err := a.webauthn.newSession()
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodGet, server.URL+base()+"/api/config", nil)
	req.Header.Set("Cookie", sessionCookie+"=stale-token; "+sessionCookie+"="+valid)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "任一有效 cookie 都应通过认证")
}

func TestPageBasePathCanonical(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mk := func(p string) *gin.Context {
		return &gin.Context{Request: &http.Request{URL: &url.URL{Path: p}}}
	}
	// 双斜杠历史形态必须规整为无尾斜杠，避免签出第二个 Path 变体
	assert.Equal(t, "/p/t", pageBasePath(mk("/p/t//api/auth/login/finish")))
	assert.Equal(t, "/p/t", pageBasePath(mk("/p/t/api/config")))
	assert.Equal(t, "/", pageBasePath(mk("/")))
}
