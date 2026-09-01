import { For, Show, createSignal } from 'solid-js'
import { testChannel } from './api'
import type { Rule, RuleMatch, TestResult } from './types'
import { BARK_LEVELS, BARK_SOUNDS, CHANNEL_TYPES, matchSummary } from './types'

function fieldOf(channel: Record<string, unknown>, key: string): string {
  const v = channel[key]
  return v === undefined || v === null ? '' : String(v)
}

/** 更新渠道字段；空字符串的可选字段直接从配置中移除，保持 JSON 干净 */
function patchChannel(
  channels: Record<string, Record<string, unknown>>,
  name: string,
  key: string,
  value: unknown,
): Record<string, Record<string, unknown>> {
  const next = { ...channels[name] }
  if (value === '' || value === undefined) delete next[key]
  else next[key] = value
  return { ...channels, [name]: next }
}

export function ChannelsEditor(props: {
  channels: Record<string, Record<string, unknown>>
  onChange: (channels: Record<string, Record<string, unknown>>) => void
}) {
  const [newName, setNewName] = createSignal('')

  const names = () => Object.keys(props.channels)

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
          <ChannelCard
            name={name}
            channel={props.channels[name]}
            onPatch={(key, value) => props.onChange(patchChannel(props.channels, name, key, value))}
            onRemove={() => remove(name)}
          />
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

function ChannelCard(props: {
  name: string
  channel: Record<string, unknown>
  onPatch: (key: string, value: unknown) => void
  onRemove: () => void
}) {
  const [testing, setTesting] = createSignal(false)
  const [result, setResult] = createSignal<TestResult | null>(null)

  const runTest = async () => {
    setTesting(true)
    setResult(null)
    try {
      setResult(await testChannel(props.channel, props.name))
    } catch (e) {
      setResult({
        ok: false,
        error: e instanceof Error ? e.message : String(e),
        cost_ms: 0,
      })
    } finally {
      setTesting(false)
    }
  }

  const archiveValue = () => {
    const v = props.channel['is_archive']
    if (v === true) return '1'
    if (v === false) return '0'
    return ''
  }

  return (
    <div class="item">
      <div class="item-head">
        <div class="item-title">
          <strong>{props.name}</strong>
          <span class="badge">{fieldOf(props.channel, 'type') || CHANNEL_TYPES[0]}</span>
        </div>
        <div class="item-actions">
          <button class="btn small" onClick={runTest} disabled={testing()}>
            {testing() ? '发送中…' : '测试发送'}
          </button>
          <button class="btn danger ghost small" onClick={props.onRemove}>
            删除
          </button>
        </div>
      </div>

      <div class="grid">
        <label class="span-2">
          <span>推送地址</span>
          <input
            type="text"
            placeholder="https://api.day.app/your_token"
            value={fieldOf(props.channel, 'url')}
            onInput={(e) => props.onPatch('url', e.currentTarget.value)}
          />
        </label>
        <label>
          <span>时效性等级</span>
          <select
            value={fieldOf(props.channel, 'level') || 'auto'}
            onChange={(e) => props.onPatch('level', e.currentTarget.value)}
          >
            <For each={[...BARK_LEVELS]}>{(l) => <option value={l.value}>{l.label}</option>}</For>
          </select>
        </label>
        <label>
          <span>铃声</span>
          <input
            type="text"
            list="bark-sounds"
            placeholder="留空使用 App 默认"
            value={fieldOf(props.channel, 'sound')}
            onInput={(e) => props.onPatch('sound', e.currentTarget.value)}
          />
          <datalist id="bark-sounds">
            <For each={[...BARK_SOUNDS]}>{(s) => <option value={s} />}</For>
          </datalist>
        </label>
        <label>
          <span>分组</span>
          <input
            type="text"
            placeholder="如 gotify"
            value={fieldOf(props.channel, 'group')}
            onInput={(e) => props.onPatch('group', e.currentTarget.value)}
          />
        </label>
        <label>
          <span>图标 URL</span>
          <input
            type="text"
            placeholder="https://…/icon.png"
            value={fieldOf(props.channel, 'icon')}
            onInput={(e) => props.onPatch('icon', e.currentTarget.value)}
          />
        </label>
        <label class="span-2">
          <span>点击跳转地址</span>
          <input
            type="text"
            placeholder="https://…（推送被点击时打开）"
            value={fieldOf(props.channel, 'jump_url')}
            onInput={(e) => props.onPatch('jump_url', e.currentTarget.value)}
          />
        </label>
        <label>
          <span>历史列表存档</span>
          <select
            value={archiveValue()}
            onChange={(e) => {
              const v = e.currentTarget.value
              props.onPatch('is_archive', v === '' ? '' : v === '1')
            }}
          >
            <option value="">跟随 App 设置</option>
            <option value="1">存档</option>
            <option value="0">不存档</option>
          </select>
        </label>
        <label>
          <span>AES 密钥（可选，16/24/32 字节）</span>
          <input
            type="text"
            autocomplete="off"
            value={fieldOf(props.channel, 'aes_key')}
            onInput={(e) => props.onPatch('aes_key', e.currentTarget.value)}
          />
        </label>
        <label>
          <span>AES IV（可选，16 字节）</span>
          <input
            type="text"
            autocomplete="off"
            value={fieldOf(props.channel, 'aes_iv')}
            onInput={(e) => props.onPatch('aes_iv', e.currentTarget.value)}
          />
        </label>
      </div>

      <Show when={result()}>
        {(r) => (
          <div class={`test-result ${r().ok ? 'ok' : 'error'}`}>
            {r().ok ? `测试成功，耗时 ${r().cost_ms}ms` : `测试失败：${r().error}`}
          </div>
        )}
      </Show>
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

  const patchToken = (token: string, rules: Rule[]) => {
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
              <button class="btn danger ghost small" onClick={() => remove(token)}>
                删除
              </button>
            </div>

            <div class="stack tight">
              <For
                each={props.rules[token] ?? []}
                fallback={<p class="empty">暂无规则，点击下方按钮添加。</p>}
              >
                {(rule, i) => (
                  <RuleRow
                    rule={rule}
                    channels={props.channels}
                    onChange={(next) => {
                      const rules = [...(props.rules[token] ?? [])]
                      rules[i()] = next
                      patchToken(token, rules)
                    }}
                    onRemove={() => {
                      const rules = (props.rules[token] ?? []).filter((_, j) => j !== i())
                      patchToken(token, rules)
                    }}
                  />
                )}
              </For>
            </div>

            <button
              class="btn small"
              onClick={() => patchToken(token, [...(props.rules[token] ?? []), { channel: '', enabled: true }])}
              disabled={Object.keys(props.channels).length === 0}
            >
              添加规则
            </button>
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
          添加规则组
        </button>
      </div>
    </div>
  )
}

function RuleRow(props: {
  rule: Rule
  channels: Record<string, Record<string, unknown>>
  onChange: (rule: Rule) => void
  onRemove: () => void
}) {
  // 编辑态保存在本地 signal，失焦/变更时归一化写回（空字段剔除）
  const [minV, setMinV] = createSignal(props.rule.match?.priority_min ?? '')
  const [maxV, setMaxV] = createSignal(props.rule.match?.priority_max ?? '')

  const patch = (next: Partial<Rule>) => props.onChange({ ...props.rule, ...next })

  const patchMatch = (next: Partial<RuleMatch>) => {
    const merged = { ...(props.rule.match ?? {}), ...next }
    // 清理空字段与空对象
    for (const k of Object.keys(merged) as (keyof RuleMatch)[]) {
      if (merged[k] === '' || merged[k] === undefined || merged[k] === false) delete merged[k]
    }
    patch(Object.keys(merged).length ? { match: merged } : { match: undefined })
  }

  const priorityValue = (raw: string): number | undefined => {
    if (raw === '') return undefined
    const n = Number(raw)
    return Number.isFinite(n) ? Math.trunc(n) : undefined
  }

  return (
    <div class="rule-row">
      <div class="rule-main">
        <label class="check">
          <input
            type="checkbox"
            checked={props.rule.enabled}
            onChange={(e) => patch({ enabled: e.currentTarget.checked })}
          />
        </label>
        <select
          class="rule-channel"
          value={props.rule.channel}
          onChange={(e) => patch({ channel: e.currentTarget.value })}
        >
          <option value="" disabled>
            选择渠道
          </option>
          <For each={Object.keys(props.channels ?? {})}>{(c) => <option value={c}>{c}</option>}</For>
        </select>
        <span class="chip" title="匹配条件">
          {matchSummary(props.rule.match)}
        </span>
        <button class="btn danger ghost small" onClick={props.onRemove}>
          删除
        </button>
      </div>

      <details class="match-editor">
        <summary>匹配条件（全部留空 = 匹配所有消息）</summary>
        <div class="grid">
          <label>
            <span>标题包含 / 正则</span>
            <input
              type="text"
              value={props.rule.match?.title_contains ?? ''}
              onInput={(e) => patchMatch({ title_contains: e.currentTarget.value })}
            />
          </label>
          <label>
            <span>内容包含 / 正则</span>
            <input
              type="text"
              value={props.rule.match?.message_contains ?? ''}
              onInput={(e) => patchMatch({ message_contains: e.currentTarget.value })}
            />
          </label>
          <label>
            <span>排除词（命中标题或内容即不转发）</span>
            <input
              type="text"
              value={props.rule.match?.exclude_contains ?? ''}
              onInput={(e) => patchMatch({ exclude_contains: e.currentTarget.value })}
            />
          </label>
          <label class="check-label">
            <span>按正则解释</span>
            <input
              type="checkbox"
              checked={props.rule.match?.use_regex ?? false}
              onChange={(e) => patchMatch({ use_regex: e.currentTarget.checked })}
            />
          </label>
          <label>
            <span>优先级 ≥</span>
            <input
              type="number"
              value={minV()}
              onInput={(e) => {
                setMinV(e.currentTarget.value)
                patchMatch({ priority_min: priorityValue(e.currentTarget.value) })
              }}
            />
          </label>
          <label>
            <span>优先级 ≤</span>
            <input
              type="number"
              value={maxV()}
              onInput={(e) => {
                setMaxV(e.currentTarget.value)
                patchMatch({ priority_max: priorityValue(e.currentTarget.value) })
              }}
            />
          </label>
        </div>
      </details>
    </div>
  )
}
