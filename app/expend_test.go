package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gotify/plugin-api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleMatch(t *testing.T) {
	min2, max8 := 2, 8
	cases := []struct {
		name string
		rule *Rule
		msg  plugin.Message
		want bool
	}{
		{"无条件匹配一切", &Rule{Channel: "c"}, plugin.Message{Title: "t", Message: "m"}, true},
		{"标题包含命中", &Rule{Match: &RuleMatch{TitleContains: "告警"}}, plugin.Message{Title: "服务告警", Message: "x"}, true},
		{"标题包含未命中", &Rule{Match: &RuleMatch{TitleContains: "告警"}}, plugin.Message{Title: "日常", Message: "告警"}, false},
		{"内容包含命中", &Rule{Match: &RuleMatch{MessageContains: "error"}}, plugin.Message{Title: "t", Message: "got error"}, true},
		{"标题与内容同时要求", &Rule{Match: &RuleMatch{TitleContains: "a", MessageContains: "b"}}, plugin.Message{Title: "a", Message: "x"}, false},
		{"排除词命中标题", &Rule{Match: &RuleMatch{ExcludeContains: "心跳"}}, plugin.Message{Title: "心跳", Message: "ok"}, false},
		{"正则命中", &Rule{Match: &RuleMatch{UseRegex: true, TitleContains: `^\d{4}-`}}, plugin.Message{Title: "2024-01-01"}, true},
		{"正则未命中", &Rule{Match: &RuleMatch{UseRegex: true, TitleContains: `^x+`}}, plugin.Message{Title: "2024"}, false},
		{"优先级下界", &Rule{Match: &RuleMatch{PriorityMin: &min2}}, plugin.Message{Priority: 1}, false},
		{"优先级上界", &Rule{Match: &RuleMatch{PriorityMax: &max8}}, plugin.Message{Priority: 9}, false},
		{"优先级区间内", &Rule{Match: &RuleMatch{PriorityMin: &min2, PriorityMax: &max8}}, plugin.Message{Priority: 5}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.rule.Matches(c.msg))
		})
	}
}

func TestMatchedRulesTokenFallback(t *testing.T) {
	conf := &Config{
		Version: ConfigVersion,
		Channels: map[string]map[string]interface{}{
			"c1": {"type": "bark", "url": "https://example.com/1"},
			"c2": {"type": "bark", "url": "https://example.com/2"},
		},
		Rules: map[string][]*Rule{
			"all": {{Channel: "c1", Enabled: true}},
			"tok": {{Channel: "c2", Enabled: false}},
		},
	}
	msg := plugin.Message{Title: "t", Message: "m"}
	token := "tok"

	// token 配置过规则列表则完全由它决定（即使规则被停用），不再回落 all
	got := conf.MatchedRules(&token, msg)
	require.Len(t, got, 0)

	// 未配置过规则列表的 token 回落 all
	other := "other"
	got = conf.MatchedRules(&other, msg)
	require.Len(t, got, 1)
	assert.Equal(t, "c1", got[0].ChannelName)

	// 匹配条件不满足时不投递
	conf.Rules["all"] = []*Rule{{Channel: "c1", Enabled: true, Match: &RuleMatch{TitleContains: "missing"}}}
	got = conf.MatchedRules(nil, msg)
	require.Len(t, got, 0)
}

func TestValidateChannelAndMatch(t *testing.T) {
	build := func(mutate func(c *Config)) *Config {
		c := &Config{
			Version: ConfigVersion,
			Channels: map[string]map[string]interface{}{
				"b1": {"type": "bark", "url": "https://example.com/t"},
			},
			Rules: map[string][]*Rule{"all": {{Channel: "b1", Enabled: true}}},
		}
		mutate(c)
		return c
	}

	assert.ErrorContains(t, build(func(c *Config) { c.Channels["b1"]["url"] = "" }).Validate(), "推送地址")
	assert.ErrorContains(t, build(func(c *Config) { c.Channels["b1"]["url"] = "ftp://x" }).Validate(), "http(s)")
	assert.ErrorContains(t, build(func(c *Config) { c.Channels["b1"]["aes_key"] = "short" }).Validate(), "aes_key")
	assert.ErrorContains(t, build(func(c *Config) { c.Channels["b1"]["aes_key"] = "1234567890123456" }).Validate(), "aes_iv")
	assert.ErrorContains(t, build(func(c *Config) { c.Channels["b1"]["type"] = "telegram" }).Validate(), "暂不支持")
	assert.ErrorContains(t, build(func(c *Config) {
		c.Rules["all"] = []*Rule{{Channel: "b1", Enabled: true, Match: &RuleMatch{UseRegex: true, TitleContains: "("}}}
	}).Validate(), "正则无效")
	assert.ErrorContains(t, build(func(c *Config) {
		c.Rules["all"] = []*Rule{{Channel: "b1", Enabled: true, Match: &RuleMatch{PriorityMin: &[]int{5}[0], PriorityMax: &[]int{1}[0]}}}
	}).Validate(), "优先级区间")
	assert.NoError(t, build(func(c *Config) {}).Validate())
}

// barkStub 模拟 Bark 服务端，可控制失败次数并记录收到的请求体。
type barkStub struct {
	server   *httptest.Server
	URL      string
	failN    int32
	requests chan map[string]interface{}
}

func newBarkStub(t *testing.T, failN int32) *barkStub {
	t.Helper()
	stub := &barkStub{requests: make(chan map[string]interface{}, 32), failN: failN}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		stub.requests <- body
		if atomic.AddInt32(&stub.failN, -1) >= 0 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":500,"message":"boom"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"success"}`))
	}))
	t.Cleanup(stub.server.Close)
	stub.URL = stub.server.URL
	return stub
}

func newForwardApp(t *testing.T, stub *barkStub) *App {
	t.Helper()
	a := newTestApp(t, &memStorage{})
	require.NoError(t, a.store.SaveConfig(&Config{
		Version: ConfigVersion,
		Channels: map[string]map[string]interface{}{
			"b1": {"type": "bark", "url": stub.URL},
		},
		Rules: map[string][]*Rule{RuleTokenAll: {{Channel: "b1", Enabled: true}}},
	}))
	return a
}

func TestForwardSuccessAndParams(t *testing.T) {
	stub := newBarkStub(t, 0)
	a := newForwardApp(t, stub)

	a.forward(nil, plugin.Message{Title: "标题", Message: "内容", Priority: 9})

	var body map[string]interface{}
	select {
	case body = <-stub.requests:
	default:
		t.Fatal("bark 未收到请求")
	}
	assert.Equal(t, "标题", body["title"])
	assert.Equal(t, "内容", body["body"])
	// 优先级 9 应推导为 timeSensitive
	assert.Equal(t, "timeSensitive", body["level"])

	logs := a.store.Logs()
	require.Len(t, logs, 1)
	assert.True(t, logs[0].OK)
	assert.Equal(t, "b1", logs[0].Channel)
	assert.Equal(t, 1, logs[0].Attempts)
}

func TestForwardRetryThenSuccess(t *testing.T) {
	stub := newBarkStub(t, 1) // 首次 500，重试成功
	a := newForwardApp(t, stub)

	a.forward(nil, plugin.Message{Title: "t", Message: "m", Priority: 1})

	logs := a.store.Logs()
	require.Len(t, logs, 1)
	assert.True(t, logs[0].OK)
	assert.Equal(t, 2, logs[0].Attempts)
}

func TestForwardRetryExhausted(t *testing.T) {
	stub := newBarkStub(t, 99) // 永远失败
	a := newForwardApp(t, stub)

	begin := time.Now()
	a.forward(nil, plugin.Message{Title: "t", Message: "m"})
	elapsed := time.Since(begin)

	logs := a.store.Logs()
	require.Len(t, logs, 1)
	assert.False(t, logs[0].OK)
	assert.Equal(t, deliverAttempts, logs[0].Attempts)
	assert.Contains(t, logs[0].Error, "500")
	// 3 次尝试 + 两次退避（500ms + 1s）至少要等 1.5s
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(1500))
}

func TestLogsRingAndPersistence(t *testing.T) {
	storage := &memStorage{}
	a := newTestApp(t, storage)

	for i := 0; i < MaxDeliveryLogs+20; i++ {
		require.NoError(t, a.store.AppendLog(DeliveryLog{Time: time.Now(), Channel: "c", Title: strconv.Itoa(i), OK: true}))
	}
	logs := a.store.Logs()
	assert.Len(t, logs, MaxDeliveryLogs)
	assert.Equal(t, "20", logs[0].Title, "最旧的记录应被淘汰")

	// 重建实例验证日志确实落盘
	b := newTestApp(t, storage)
	assert.Len(t, b.store.Logs(), MaxDeliveryLogs)

	require.NoError(t, b.store.ClearLogs())
	assert.Empty(t, b.store.Logs())
}

func TestLogsEndpointGuard(t *testing.T) {
	a := newTestApp(t, &memStorage{})
	r, base := newTestRouter(a)
	server := httptest.NewServer(r)
	defer server.Close()
	url := server.URL + base()

	for _, target := range []string{"/api/logs", "/api/test", "/api/meta"} {
		method := http.MethodGet
		if target == "/api/test" {
			method = http.MethodPost
		}
		resp := doReq(t, method, url+target, nil)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, target)
	}
}

func TestTestSendEndpoint(t *testing.T) {
	stub := newBarkStub(t, 0)
	a := newTestApp(t, &memStorage{})
	r, base := newTestRouter(a)
	server := httptest.NewServer(r)
	defer server.Close()

	// 直接构造会话，绕过 passkey 流程
	token, err := a.webauthn.newSession()
	require.NoError(t, err)

	payload, err := json.Marshal(map[string]interface{}{
		"channel": map[string]interface{}{"type": "bark", "url": stub.URL},
	})
	require.NoError(t, err)
	req, _ := http.NewRequest(http.MethodPost, server.URL+base()+"/api/test?name=b1", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		OK     bool  `json:"ok"`
		CostMS int64 `json:"cost_ms"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.True(t, result.OK)
}
