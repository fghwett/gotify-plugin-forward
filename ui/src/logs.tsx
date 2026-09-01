import { For, Show, createMemo, createSignal, onCleanup, onMount } from 'solid-js'
import { clearLogs, getLogs } from './api'
import type { DeliveryLog } from './types'

function formatTime(iso: string): string {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

export function LogsPanel(props: { channels: Record<string, Record<string, unknown>> }) {
  const [logs, setLogs] = createSignal<DeliveryLog[]>([])
  const [error, setError] = createSignal('')
  const [busy, setBusy] = createSignal(false)
  const [statusFilter, setStatusFilter] = createSignal<'all' | 'ok' | 'fail'>('all')
  const [channelFilter, setChannelFilter] = createSignal('')
  const [autoRefresh, setAutoRefresh] = createSignal(true)
  const [expanded, setExpanded] = createSignal<string | null>(null)

  const refresh = async () => {
    setBusy(true)
    try {
      const data = await getLogs()
      setLogs(data.logs ?? [])
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  onMount(refresh)

  // 自动刷新：投递是异步发生的，日志页保持新鲜
  let timer: ReturnType<typeof setInterval> | undefined
  onMount(() => {
    timer = setInterval(() => {
      if (autoRefresh() && !busy()) void refresh()
    }, 10_000)
  })
  onCleanup(() => clearInterval(timer))

  const channelNames = createMemo(() => Object.keys(props.channels))

  const filtered = createMemo(() =>
    logs().filter((l) => {
      if (statusFilter() === 'ok' && !l.ok) return false
      if (statusFilter() === 'fail' && l.ok) return false
      if (channelFilter() && l.channel !== channelFilter()) return false
      return true
    }),
  )

  const stats = createMemo(() => {
    const total = logs().length
    const failed = logs().filter((l) => !l.ok).length
    return { total, failed }
  })

  const handleClear = async () => {
    if (!window.confirm(`确定清空全部 ${stats().total} 条投递日志？`)) return
    try {
      await clearLogs()
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  const rowKey = (l: DeliveryLog, i: number) => `${l.time}-${i}`

  return (
    <div class="stack">
      <div class="toolbar">
        <label class="inline">
          <span>状态</span>
          <select value={statusFilter()} onChange={(e) => setStatusFilter(e.currentTarget.value as 'all' | 'ok' | 'fail')}>
            <option value="all">全部</option>
            <option value="ok">成功</option>
            <option value="fail">失败</option>
          </select>
        </label>
        <label class="inline">
          <span>渠道</span>
          <select value={channelFilter()} onChange={(e) => setChannelFilter(e.currentTarget.value)}>
            <option value="">全部</option>
            <For each={channelNames()}>{(c) => <option value={c}>{c}</option>}</For>
          </select>
        </label>
        <label class="check inline">
          <input
            type="checkbox"
            checked={autoRefresh()}
            onChange={(e) => setAutoRefresh(e.currentTarget.checked)}
          />
          <span>自动刷新</span>
        </label>
        <span class="toolbar-spacer" />
        <span class="hint">
          共 {stats().total} 条，失败 {stats().failed} 条
        </span>
        <button class="btn small" onClick={() => void refresh()} disabled={busy()}>
          {busy() ? '刷新中…' : '刷新'}
        </button>
        <button class="btn danger small" onClick={() => void handleClear()} disabled={stats().total === 0}>
          清空
        </button>
      </div>

      <Show when={error()}>
        <div class="alert error">{error()}</div>
      </Show>

      <Show
        when={filtered().length > 0}
        fallback={<div class="card center">暂无投递记录。发送一条消息或用「测试发送」试试。</div>}
      >
        <div class="table-wrap">
          <table class="logs">
            <thead>
              <tr>
                <th>时间</th>
                <th>标题</th>
                <th>来源</th>
                <th>渠道</th>
                <th>状态</th>
                <th>次数</th>
                <th>耗时</th>
              </tr>
            </thead>
            <tbody>
              <For each={filtered()}>
                {(l, i) => {
                  const key = rowKey(l, i())
                  const open = () => expanded() === key
                  return (
                    <>
                      <tr
                        class={l.ok ? '' : 'row-fail'}
                        onClick={() => setExpanded(open() ? null : key)}
                        title={l.error ? l.error : l.excerpt}
                      >
                        <td class="mono dim">{formatTime(l.time)}</td>
                        <td class="title-cell">{l.title || '（无标题）'}</td>
                        <td class="mono dim">{l.token}</td>
                        <td>{l.channel}</td>
                        <td>
                          <span class={`badge ${l.ok ? 'ok' : 'error'}`}>{l.ok ? '成功' : '失败'}</span>
                        </td>
                        <td class="dim">{l.attempts > 1 ? `${l.attempts} 次` : '—'}</td>
                        <td class="dim">{l.cost_ms}ms</td>
                      </tr>
                      <Show when={open()}>
                        <tr class="detail-row">
                          <td colspan="7">
                            <Show when={l.error} fallback={<span class="dim">无错误信息</span>}>
                              <div class="detail error">错误：{l.error}</div>
                            </Show>
                            <Show when={l.excerpt}>
                              <div class="detail">内容：{l.excerpt}</div>
                            </Show>
                          </td>
                        </tr>
                      </Show>
                    </>
                  )
                }}
              </For>
            </tbody>
          </table>
        </div>
        <p class="hint">点击行可展开错误详情与消息摘要。日志保存在插件存储中，仅保留最近 200 条。</p>
      </Show>
    </div>
  )
}
