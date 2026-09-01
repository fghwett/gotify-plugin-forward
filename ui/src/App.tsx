import { For, Show, Switch, Match, createEffect, createSignal, onCleanup, onMount } from 'solid-js'
import {
  ApiError,
  getAuthStatus,
  getConfig,
  loginPasskey,
  logout,
  registerPasskey,
  saveConfig,
} from './api'
import type { AuthStatus, Config } from './types'
import { ChannelsEditor, RulesEditor } from './editors'
import { LogsPanel } from './logs'
import { AboutPanel } from './about'

type Phase = 'loading' | 'setup' | 'login' | 'ready'
type Tab = 'channels' | 'rules' | 'logs' | 'about'

const TABS: { key: Tab; label: string }[] = [
  { key: 'channels', label: '渠道' },
  { key: 'rules', label: '规则' },
  { key: 'logs', label: '日志' },
  { key: 'about', label: '关于' },
]

/** 与服务端 Validate 对应的本地校验，保存前先拦一道 */
function validateConfig(conf: Config): string[] {
  const errors: string[] = []
  const channelNames = new Set(Object.keys(conf.channels))

  for (const [name, channel] of Object.entries(conf.channels)) {
    if (!name.trim()) errors.push('存在未命名的渠道')
    const url = String(channel.url ?? '')
    if (!url) errors.push(`渠道「${name}」缺少推送地址`)
    else if (!/^https?:\/\/[^\s/]+/i.test(url)) errors.push(`渠道「${name}」的推送地址不是合法的 http(s) 链接`)

    const key = String(channel.aes_key ?? '')
    const iv = String(channel.aes_iv ?? '')
    if (key || iv) {
      if (!key || !iv) errors.push(`渠道「${name}」的 AES 密钥与 IV 需同时填写`)
      else {
        if (![16, 24, 32].includes(key.length)) errors.push(`渠道「${name}」的 AES 密钥长度须为 16/24/32 字节`)
        if (iv.length !== 16) errors.push(`渠道「${name}」的 AES IV 长度须为 16 字节`)
      }
    }
  }

  for (const [token, rules] of Object.entries(conf.rules)) {
    if (!token.trim()) errors.push('存在空的应用 token')
    rules.forEach((rule, i) => {
      const where = `规则组「${token}」第 ${i + 1} 条`
      if (!rule.channel) errors.push(`${where}未选择渠道`)
      else if (!channelNames.has(rule.channel)) errors.push(`${where}引用了不存在的渠道「${rule.channel}」`)
      const m = rule.match
      if (!m) return
      if (m.use_regex) {
        for (const [field, pattern] of [
          ['标题', m.title_contains],
          ['内容', m.message_contains],
          ['排除', m.exclude_contains],
        ] as const) {
          if (!pattern) continue
          try {
            new RegExp(pattern)
          } catch (e) {
            errors.push(`${where}的${field}正则无效：${e instanceof Error ? e.message : e}`)
          }
        }
      }
      if (m.priority_min !== undefined && m.priority_max !== undefined && m.priority_min > m.priority_max)
        errors.push(`${where}的优先级区间无效（min > max）`)
    })
  }
  return errors
}

export default function App() {
  const [phase, setPhase] = createSignal<Phase>('loading')
  const [tab, setTab] = createSignal<Tab>('channels')
  const [error, setError] = createSignal('')
  const [notice, setNotice] = createSignal('')
  const [config, setConfig] = createSignal<Config | null>(null)
  const [busy, setBusy] = createSignal(false)
  const [snapshot, setSnapshot] = createSignal('')

  const dirty = () => !!config() && JSON.stringify(config()) !== snapshot()
  const validationErrors = () => (config() ? validateConfig(config()!) : [])

  // 有未保存更改时拦截页面关闭/刷新
  createEffect(() => {
    const isDirty = dirty()
    const handler = (e: BeforeUnloadEvent) => {
      if (isDirty) {
        e.preventDefault()
        e.returnValue = ''
      }
    }
    window.addEventListener('beforeunload', handler)
    onCleanup(() => window.removeEventListener('beforeunload', handler))
  })

  const applyStatus = (status: AuthStatus) => {
    if (status.authenticated) {
      setPhase('ready')
      loadConfig()
    } else {
      setPhase(status.configured ? 'login' : 'setup')
    }
  }

  const loadConfig = async () => {
    try {
      const conf = await getConfig()
      setConfig(conf)
      setSnapshot(JSON.stringify(conf))
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e))
    }
  }

  onMount(async () => {
    try {
      applyStatus(await getAuthStatus())
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e))
    }
  })

  const run = async (fn: () => Promise<void>) => {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await fn()
    } catch (e) {
      const message = e instanceof Error ? e.message : String(e)
      setError(
        e instanceof ApiError && e.status === 0 && message.includes('secure')
          ? '当前环境不支持 passkey：请通过 HTTPS 或 localhost 访问本页面'
          : message,
      )
    } finally {
      setBusy(false)
    }
  }

  const handleSetup = () =>
    run(async () => {
      await registerPasskey()
      setPhase('ready')
      await loadConfig()
      setNotice('passkey 设置成功')
    })

  const handleLogin = () =>
    run(async () => {
      await loginPasskey()
      setPhase('ready')
      await loadConfig()
    })

  const handleLogout = () =>
    run(async () => {
      await logout()
      setConfig(null)
      setSnapshot('')
      setPhase('login')
    })

  const handleSave = () =>
    run(async () => {
      const conf = config()
      if (!conf) return
      const problems = validateConfig(conf)
      if (problems.length > 0) {
        setError(problems[0] + (problems.length > 1 ? `（共 ${problems.length} 处问题）` : ''))
        return
      }
      await saveConfig(conf)
      setSnapshot(JSON.stringify(conf))
      setNotice('配置已保存')
    })

  return (
    <main class="page wide">
      <header class="header">
        <h1>Gotify Forward</h1>
        <Show when={phase() === 'ready'}>
          <div class="header-actions">
            <Show when={dirty()}>
              <span class="dirty-dot" title="有未保存的更改">
                未保存
              </span>
            </Show>
            <button class="btn primary" onClick={handleSave} disabled={busy() || !config()}>
              {busy() ? '保存中…' : '保存配置'}
            </button>
            <button class="btn ghost" onClick={handleLogout} disabled={busy()}>
              退出登录
            </button>
          </div>
        </Show>
      </header>

      <Show when={error()}>
        <div class="alert error">{error()}</div>
      </Show>
      <Show when={notice()}>
        <div class="alert ok">{notice()}</div>
      </Show>

      <Switch>
        <Match when={phase() === 'loading'}>
          <div class="card center">加载中…</div>
        </Match>

        <Match when={phase() === 'setup'}>
          <div class="card center">
            <h2>首次使用</h2>
            <p>
              为这个配置页面设置一个 passkey（指纹 / 面容 / 硬件密钥），
              之后每次进入都需要通过它验证身份。
            </p>
            <p class="hint">
              页面地址本身就是私密的（含随机令牌），请妥善保存。passkey 仅在
              HTTPS 或 localhost 下可用。
            </p>
            <button class="btn primary" onClick={handleSetup} disabled={busy()}>
              {busy() ? '请稍候…' : '设置 passkey'}
            </button>
          </div>
        </Match>

        <Match when={phase() === 'login'}>
          <div class="card center">
            <h2>passkey 验证</h2>
            <p>此页面已设置 passkey 保护，请完成验证后编辑配置。</p>
            <button class="btn primary" onClick={handleLogin} disabled={busy()}>
              {busy() ? '请稍候…' : '使用 passkey 登录'}
            </button>
            <p class="hint">
              passkey 丢失？在 gotify 原生插件配置中将 reset_passkey 设为 true
              并保存，即可重新设置。
            </p>
          </div>
        </Match>

        <Match when={phase() === 'ready'}>
          <Show when={config()} fallback={<div class="card center">加载配置…</div>}>
            {(conf) => (
              <>
                <nav class="tabs">
                  <For each={TABS}>
                    {(t) => (
                      <button
                        class={`tab ${tab() === t.key ? 'active' : ''}`}
                        onClick={() => setTab(t.key)}
                      >
                        {t.label}
                      </button>
                    )}
                  </For>
                </nav>

                <Show when={validationErrors().length > 0 && (tab() === 'channels' || tab() === 'rules')}>
                  <div class="alert error">
                    <strong>配置存在 {validationErrors().length} 处问题：</strong>
                    <ul class="problem-list">
                      <For each={validationErrors().slice(0, 5)}>{(p) => <li>{p}</li>}</For>
                      <Show when={validationErrors().length > 5}>
                        <li>… 其余 {validationErrors().length - 5} 处从略</li>
                      </Show>
                    </ul>
                  </div>
                </Show>

                <Switch>
                  <Match when={tab() === 'channels'}>
                    <section class="card">
                      <h2>转发渠道</h2>
                      <p class="hint">
                        支持 Bark 推送；时效性等级默认按消息优先级自动推导（≥8
                        时效性、≥1 常规、≤0 静默），AES 密钥与 IV 为可选的端到端加密。
                        填好后可用「测试发送」即时验证。
                      </p>
                      <ChannelsEditor
                        channels={conf().channels}
                        onChange={(channels) => setConfig({ ...conf(), channels })}
                      />
                    </section>
                  </Match>

                  <Match when={tab() === 'rules'}>
                    <section class="card">
                      <h2>转发规则</h2>
                      <p class="hint">
                        按来源 token 绑定渠道，同一 token 可配多条规则分别匹配不同消息；
                        <code>all</code> 为兜底规则，token 未配置专属规则时生效。
                        匹配条件支持关键字/正则、排除词与优先级区间。
                      </p>
                      <RulesEditor
                        channels={conf().channels}
                        rules={conf().rules}
                        onChange={(rules) => setConfig({ ...conf(), rules })}
                      />
                    </section>
                  </Match>

                  <Match when={tab() === 'logs'}>
                    <LogsPanel channels={conf().channels} />
                  </Match>

                  <Match when={tab() === 'about'}>
                    <AboutPanel
                      config={conf()}
                      onImport={(imported) => setConfig(imported)}
                      onNotice={setNotice}
                      onError={setError}
                    />
                  </Match>
                </Switch>
              </>
            )}
          </Show>
        </Match>
      </Switch>

      <footer class="footer">
        <span>gotify-plugin-forward · 消息转发插件</span>
      </footer>
    </main>
  )
}
