package app

import (
	"encoding/json"
	"sync"

	"github.com/go-webauthn/webauthn/webauthn"
)

// StoredData 是写入插件持久化存储（plugin.StorageHandler）的全部数据，
// 包含可视化编辑的配置和 passkey 凭据，整体序列化为一份 JSON。
type StoredData struct {
	Config  *Config      `json:"config,omitempty"`
	Passkey *PasskeyData `json:"passkey,omitempty"`
}

// PasskeyData 保存用户 handle 与凭据公钥部分，私钥永远留在用户设备上。
type PasskeyData struct {
	UserID      []byte                `json:"user_id"`
	Credentials []webauthn.Credential `json:"credentials"`
}

// store 管理 StoredData 的内存态与持久化，插件实例（每用户）各持有一份。
type store struct {
	mu      sync.Mutex
	data    StoredData
	handler StorageHandler
}

// StorageHandler 与 plugin.StorageHandler 签名一致，便于测试替身。
type StorageHandler interface {
	Save(b []byte) error
	Load() ([]byte, error)
}

func newStore() *store {
	return &store{}
}

// attach 绑定持久化处理器并加载已有数据，gotify 保证它在配置初始化之前调用。
func (s *store) attach(handler StorageHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.handler = handler
	if handler == nil {
		return
	}
	body, err := handler.Load()
	if err != nil || len(body) == 0 {
		return
	}
	_ = json.Unmarshal(body, &s.data)
}

func (s *store) persist() error {
	if s.handler == nil {
		return nil
	}
	body, err := json.Marshal(&s.data)
	if err != nil {
		return err
	}
	return s.handler.Save(body)
}

// Config 返回可视化编辑保存的配置，未保存过时返回 nil。
func (s *store) Config() *Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Config
}

func (s *store) SaveConfig(conf *Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Config = conf
	return s.persist()
}

func (s *store) Passkey() *PasskeyData {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Passkey
}

func (s *store) SavePasskey(data *PasskeyData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Passkey = data
	return s.persist()
}

// ClearPasskey 用于 reset_passkey 兜底开关：凭据丢失时通过原生 YAML 配置触发重置。
func (s *store) ClearPasskey() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data.Passkey == nil {
		return nil
	}
	s.data.Passkey = nil
	return s.persist()
}
