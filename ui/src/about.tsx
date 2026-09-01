import { createSignal, onMount } from 'solid-js'
import { getMeta } from './api'
import type { Config, Meta } from './types'

export function AboutPanel(props: {
  config: Config | null
  onImport: (config: Config) => void
  onNotice: (message: string) => void
  onError: (message: string) => void
}) {
  const [meta, setMeta] = createSignal<Meta | null>(null)

  onMount(async () => {
    try {
      setMeta(await getMeta())
    } catch {
      // 元信息加载失败不阻塞页面
    }
  })

  const exportConfig = () => {
    if (!props.config) return
    const blob = new Blob([JSON.stringify(props.config, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'gotify-forward-config.json'
    a.click()
    URL.revokeObjectURL(url)
  }

  const importConfig = async (file: File) => {
    try {
      const text = await file.text()
      const parsed = JSON.parse(text) as Config
      if (typeof parsed !== 'object' || parsed === null || typeof parsed.channels !== 'object' || typeof parsed.rules !== 'object') {
        throw new Error('文件结构不像一份转发配置（缺少 channels / rules）')
      }
      props.onImport({ ...parsed, version: parsed.version || '1' })
      props.onNotice('配置已导入（尚未保存，检查无误后请点击保存）')
    } catch (e) {
      props.onError(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <div class="stack">
      <section class="card">
        <h2>插件信息</h2>
        <dl class="kv">
          <dt>名称</dt>
          <dd>{meta()?.name ?? 'gotify-plugin-forward'}</dd>
          <dt>版本</dt>
          <dd class="mono">{meta()?.version ?? '…'}</dd>
          <dt>项目主页</dt>
          <dd>
            <a href="https://github.com/fghwett/gotify-plugin-forward" target="_blank" rel="noreferrer">
              github.com/fghwett/gotify-plugin-forward
            </a>
          </dd>
        </dl>
        <p class="hint">
          转发在消息入库后异步执行：单渠道最多尝试 3 次（退避 0.5s / 1s），多渠道并行投递，
          结果记录在「日志」标签。
        </p>
      </section>

      <section class="card">
        <h2>配置导入 / 导出</h2>
        <p class="hint">
          导出当前配置为 JSON 文件，或从文件导入（导入后需检查并手动保存才会生效）。
        </p>
        <div class="actions">
          <button class="btn" onClick={exportConfig} disabled={!props.config}>
            导出配置
          </button>
          <label class="btn import-label">
            导入配置
            <input
              type="file"
              accept="application/json,.json"
              onChange={(e) => {
                const f = e.currentTarget.files?.[0]
                if (f) void importConfig(f)
                e.currentTarget.value = ''
              }}
            />
          </label>
        </div>
      </section>

      <section class="card">
        <h2>passkey</h2>
        <p class="hint">
          页面通过 passkey（指纹 / 面容 / 硬件密钥）验证身份，会话有效期 7 天。
          passkey 丢失时：在 gotify 原生插件配置中将 <code>reset_passkey</code> 设为{' '}
          <code>true</code> 并保存，重新打开本页即可重设（完成后请改回 <code>false</code>）。
        </p>
      </section>
    </div>
  )
}
