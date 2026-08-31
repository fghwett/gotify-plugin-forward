import { For, createSignal } from 'solid-js'
import type { Rule } from './types'
import { CHANNEL_TYPES } from './types'

function fieldOf(channel: Record<string, unknown>, key: string): string {
  const v = channel[key]
  return v === undefined || v === null ? '' : String(v)
}

export function ChannelsEditor(props: {
  channels: Record<string, Record<string, unknown>>
  onChange: (channels: Record<string, Record<string, unknown>>) => void
}) {
  const [newName, setNewName] = createSignal('')

  const names = () => Object.keys(props.channels)

  const patch = (name: string, key: string, value: string) => {
    const next = { ...props.channels, [name]: { ...props.channels[name], [key]: value } }
    props.onChange(next)
  }

  const remove = (name: string) => {
    const next = { ...props.channels }
    delete next[name]
    props.onChange(next)
  }

  const add = () => {
    const name = newName().trim()
    if (!name || props.channels[name]) return
    setNewName('')
    props.onChange({ ...props.channels, [name]: { type: CHANNEL_TYPES[0], url: '' } })
  }

  return (
    <div class="stack">
      <For each={names()} fallback={<p class="empty">还没有渠道，先添加一个吧。</p>}>
        {(name) => (
          <div class="item">
            <div class="item-head">
              <strong>{name}</strong>
              <button class="btn danger ghost" onClick={() => remove(name)}>
                删除
              </button>
            </div>
            <div class="grid">
              <label>
                <span>类型</span>
                <select
                  value={fieldOf(props.channels[name], 'type') || CHANNEL_TYPES[0]}
                  onChange={(e) => patch(name, 'type', e.currentTarget.value)}
                >
                  <For each={[...CHANNEL_TYPES]}>{(t) => <option value={t}>{t}</option>}</For>
                </select>
              </label>
              <label>
                <span>推送地址</span>
                <input
                  type="text"
                  placeholder="https://api.day.app/your_token"
                  value={fieldOf(props.channels[name], 'url')}
                  onInput={(e) => patch(name, 'url', e.currentTarget.value)}
                />
              </label>
              <label>
                <span>AES 密钥（可选）</span>
                <input
                  type="text"
                  autocomplete="off"
                  value={fieldOf(props.channels[name], 'aes_key')}
                  onInput={(e) => patch(name, 'aes_key', e.currentTarget.value)}
                />
              </label>
              <label>
                <span>AES IV（可选）</span>
                <input
                  type="text"
                  autocomplete="off"
                  value={fieldOf(props.channels[name], 'aes_iv')}
                  onInput={(e) => patch(name, 'aes_iv', e.currentTarget.value)}
                />
              </label>
            </div>
          </div>
        )}
      </For>

      <div class="inline-add">
        <input
          type="text"
          placeholder="新渠道名称，如 myBark"
          value={newName()}
          onInput={(e) => setNewName(e.currentTarget.value)}
          onKeyDown={(e) => e.key === 'Enter' && add()}
        />
        <button class="btn" onClick={add} disabled={!newName().trim()}>
          添加渠道
        </button>
      </div>
    </div>
  )
}

export function RulesEditor(props: {
  channels: Record<string, Record<string, unknown>>
  rules: Record<string, Rule[]>
  onChange: (rules: Record<string, Rule[]>) => void
}) {
  const [newToken, setNewToken] = createSignal('')

  const tokens = () => Object.keys(props.rules)

  const toggle = (token: string, channel: string, enabled: boolean) => {
    const current = props.rules[token] ?? []
    let rules = current.filter((r) => r.channel !== channel)
    if (enabled) rules = [...rules, { channel, enabled: true }]
    props.onChange({ ...props.rules, [token]: rules })
  }

  const remove = (token: string) => {
    const next = { ...props.rules }
    delete next[token]
    props.onChange(next)
  }

  const add = () => {
    const token = newToken().trim()
    if (!token || props.rules[token]) return
    setNewToken('')
    props.onChange({ ...props.rules, [token]: [] })
  }

  return (
    <div class="stack">
      <For each={tokens()} fallback={<p class="empty">还没有规则。添加 token 或使用 all 兜底。</p>}>
        {(token) => (
          <div class="item">
            <div class="item-head">
              <strong class="mono">{token === 'all' ? 'all（兜底）' : token}</strong>
              <button class="btn danger ghost" onClick={() => remove(token)}>
                删除
              </button>
            </div>
            <div class="checks">
              <For each={Object.keys(props.channels)} fallback={<span class="hint">请先创建渠道</span>}>
                {(channel) => {
                  const checked = () =>
                    (props.rules[token] ?? []).some((r) => r.channel === channel && r.enabled)
                  return (
                    <label class="check">
                      <input
                        type="checkbox"
                        checked={checked()}
                        onChange={(e) => toggle(token, channel, e.currentTarget.checked)}
                      />
                      <span>{channel}</span>
                    </label>
                  )
                }}
              </For>
            </div>
          </div>
        )}
      </For>

      <div class="inline-add">
        <input
          type="text"
          class="mono"
          placeholder="应用 token（或 all）"
          value={newToken()}
          onInput={(e) => setNewToken(e.currentTarget.value)}
          onKeyDown={(e) => e.key === 'Enter' && add()}
        />
        <button class="btn" onClick={add} disabled={!newToken().trim()}>
          添加规则
        </button>
      </div>
    </div>
  )
}
