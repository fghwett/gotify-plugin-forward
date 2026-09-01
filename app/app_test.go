package app

import (
	"net/http"
	"net/http/httptest"
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
