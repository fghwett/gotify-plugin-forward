// 页面挂在 gotify 动态分配的路径前缀下（…/config），
// 相对路径 api/... 会解析到同一前缀的 /api/...
const BASE = 'api'

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

async function request(path: string, init: RequestInit = {}, sessionId?: string): Promise<any> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (sessionId) headers['X-Session-Id'] = sessionId
  const resp = await fetch(`${BASE}/${path}`, { ...init, headers })
  const data = await resp.json().catch(() => ({}))
  if (!resp.ok) throw new ApiError(resp.status, data.message ?? `请求失败（HTTP ${resp.status}）`)
  return data
}

export const getAuthStatus = () => request('auth/status')
export const logout = () => request('auth/logout', { method: 'POST' })
export const getConfig = () => request('config')
export const saveConfig = (config: unknown) =>
  request('config', { method: 'POST', body: JSON.stringify(config) })
export const testChannel = (channel: unknown, name?: string) =>
  request(`test${name ? `?name=${encodeURIComponent(name)}` : ''}`, {
    method: 'POST',
    body: JSON.stringify({ channel }),
  })
export const getLogs = () => request('logs')
export const clearLogs = () => request('logs', { method: 'DELETE' })
export const getMeta = () => request('meta')

// ---------- WebAuthn ----------

function b64urlToBuffer(value: string): Uint8Array {
  const pad = '='.repeat((4 - (value.length % 4)) % 4)
  const b64 = (value + pad).replace(/-/g, '+').replace(/_/g, '/')
  const raw = atob(b64)
  const out = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i)
  return out
}

function bufferToB64url(value: ArrayBuffer | Uint8Array): string {
  const bytes = value instanceof Uint8Array ? value : new Uint8Array(value)
  let str = ''
  for (const b of bytes) str += String.fromCharCode(b)
  return btoa(str).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function toCreateOptions(options: any): PublicKeyCredentialCreationOptions {
  return {
    ...options,
    challenge: b64urlToBuffer(options.challenge),
    user: { ...options.user, id: b64urlToBuffer(options.user.id) },
    excludeCredentials: (options.excludeCredentials ?? []).map((c: any) => ({
      ...c,
      id: b64urlToBuffer(c.id),
    })),
  }
}

function toRequestOptions(options: any): PublicKeyCredentialRequestOptions {
  return {
    ...options,
    challenge: b64urlToBuffer(options.challenge),
    allowCredentials: (options.allowCredentials ?? []).map((c: any) => ({
      ...c,
      id: b64urlToBuffer(c.id),
    })),
  }
}

async function beginCeremony(kind: 'register' | 'login') {
  const data = await request(`auth/${kind}/begin`, { method: 'POST' })
  return { sessionId: data.session_id as string, options: data.options }
}

function finishCeremony(kind: 'register' | 'login', sessionId: string, credential: PublicKeyCredential) {
  const response = credential.response as AuthenticatorAttestationResponse &
    AuthenticatorAssertionResponse
  const body: Record<string, unknown> = {
    id: credential.id,
    rawId: bufferToB64url(credential.rawId),
    type: credential.type,
    clientExtensionResults: {},
    response: {
      clientDataJSON: bufferToB64url(response.clientDataJSON),
    },
  }
  if (kind === 'register') {
    body.response = {
      ...(body.response as Record<string, unknown>),
      attestationObject: bufferToB64url(response.attestationObject),
      transports: ['internal', 'hybrid', 'usb'],
    }
  } else {
    body.response = {
      ...(body.response as Record<string, unknown>),
      authenticatorData: bufferToB64url(response.authenticatorData),
      signature: bufferToB64url(response.signature),
      userHandle: response.userHandle ? bufferToB64url(response.userHandle) : null,
    }
  }
  return request(`auth/${kind}/finish`, {
    method: 'POST',
    body: JSON.stringify(body),
  }, sessionId)
}

/** 首次设置 passkey，成功后自动进入已登录状态 */
export async function registerPasskey(): Promise<void> {
  const { sessionId, options } = await beginCeremony('register')
  const credential = (await navigator.credentials.create({
    publicKey: toCreateOptions(options.publicKey),
  })) as PublicKeyCredential
  if (!credential) throw new Error('未完成 passkey 创建')
  await finishCeremony('register', sessionId, credential)
}

/** 使用 passkey 登录 */
export async function loginPasskey(): Promise<void> {
  const { sessionId, options } = await beginCeremony('login')
  const credential = (await navigator.credentials.get({
    publicKey: toRequestOptions(options.publicKey),
  })) as PublicKeyCredential
  if (!credential) throw new Error('未完成 passkey 验证')
  await finishCeremony('login', sessionId, credential)
}
