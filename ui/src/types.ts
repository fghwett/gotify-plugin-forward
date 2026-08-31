export interface Rule {
  channel: string
  enabled: boolean
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

export const CHANNEL_TYPES = ['bark'] as const
