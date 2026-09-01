export interface RuleMatch {
  title_contains?: string
  message_contains?: string
  exclude_contains?: string
  use_regex?: boolean
  priority_min?: number
  priority_max?: number
}

export interface Rule {
  channel: string
  enabled: boolean
  match?: RuleMatch
}

export interface Config {
  version: string
  channels: Record<string, Record<string, unknown>>
  rules: Record<string, Rule[]>
  reset_passkey?: boolean
}

export interface AuthStatus {
  configured: boolean
  authenticated: boolean
}

export interface DeliveryLog {
  time: string
  token: string
  channel: string
  title: string
  excerpt?: string
  ok: boolean
  error?: string
  attempts: number
  cost_ms: number
}

export interface Meta {
  name: string
  version: string
}

export interface TestResult {
  ok: boolean
  error: string
  cost_ms: number
}

export const CHANNEL_TYPES = ['bark'] as const

export const BARK_SOUNDS = [
  'bell',
  'birdsong',
  'bloom',
  'calippo',
  'chime',
  'chiptune',
  'descent',
  'electronic',
  'fanfare',
  'glass',
  'goscompatible',
  'healthnotification',
  'horn',
  'ladder',
  'mailsent',
  'minuet',
  'multiinvitation',
  'multitouch_alarm',
  'nightingale',
  'pineapple',
  'plot twist',
  'presto',
  'silk',
  'telegraph',
  'tiptoe',
  'typewriters',
  'update',
]

export const BARK_LEVELS = [
  { value: 'auto', label: '自动（按消息优先级）' },
  { value: 'active', label: 'active（常规提醒）' },
  { value: 'timeSensitive', label: 'timeSensitive（时效性）' },
  { value: 'passive', label: 'passive（静默）' },
] as const

/** 规则匹配条件的摘要文案，展示在规则行上 */
export function matchSummary(match?: RuleMatch): string {
  if (!match || Object.keys(match).length === 0) return '全部消息'
  const parts: string[] = []
  if (match.title_contains) parts.push(`标题${match.use_regex ? '匹配' : '含'}「${match.title_contains}」`)
  if (match.message_contains) parts.push(`内容${match.use_regex ? '匹配' : '含'}「${match.message_contains}」`)
  if (match.exclude_contains) parts.push(`排除「${match.exclude_contains}」`)
  if (match.priority_min !== undefined || match.priority_max !== undefined) {
    const min = match.priority_min ?? '-∞'
    const max = match.priority_max ?? '+∞'
    parts.push(`优先级 ${min}~${max}`)
  }
  return parts.join('，') || '全部消息'
}
