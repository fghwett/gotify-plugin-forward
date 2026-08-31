import { For, Show, Switch, Match, createSignal, onMount } from 'solid-js'
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

type Phase = 'loading' | 'setup' | 'login' | 'ready'

export default function App() {
  const [phase, setPhase] = createSignal<Phase>('loading')
  const [error, setError] = createSignal('')
  const [notice, setNotice] = createSignal('')
  const [config, setConfig] = createSignal<Config | null>(null)
  const [busy, setBusy] = createSignal(false)

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
      setConfig(await getConfig())
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
      setPhase('login')
    })

  const handleSave = () =>
    run(async () => {
      const conf = config()
      if (!conf) return
      await saveConfig(conf)
      setNotice('配置已保存')
    })

  return (
    <main class="page">
      <header class="header">
        <h1>Gotify Forward</h1>
        <Show when={phase() === 'ready'}>
          <button class="btn ghost" onClick={handleLogout} disabled={busy()}>
            退出登录
          </button>
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
              <div class="stack">
                <section class="card">
                  <h2>转发渠道</h2>
                  <p class="hint">支持 Bark 推送；AES 密钥与 IV 均为可选（用于端到端加密推送）。</p>
                  <ChannelsEditor
                    channels={conf().channels}
                    onChange={(channels) => setConfig({ ...conf(), channels })}
                  />
                </section>

                <section class="card">
                  <h2>转发规则</h2>
                  <p class="hint">
                    按来源 token 绑定渠道；<code>all</code> 为兜底规则，匹配不到
                    token 专属规则时生效。
                  </p>
                  <RulesEditor
                    channels={conf().channels}
                    rules={conf().rules}
                    onChange={(rules) => setConfig({ ...conf(), rules })}
                  />
                </section>

                <div class="actions">
                  <button class="btn primary" onClick={handleSave} disabled={busy()}>
                    {busy() ? '保存中…' : '保存配置'}
                  </button>
                </div>
              </div>
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
