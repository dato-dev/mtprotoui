export type Server = {
  id: string
  name: string
  host: string
  ssh_port: number
  ssh_user: string
  ssh_auth_type: string
  proxy_type: string
  fake_tls: boolean
  sni_domain: string
  mtproto_port: number
  secret: string
  proxy_link: string
  container_name: string
  tags: string[]
  status: string
  ping_status: string
  tcp_status: string
  ping_rtt_ms?: number
  deploy_status: string
  operation: string
  operation_step: string
  operation_message: string
  operation_progress: number
  last_check_at?: string
  last_error?: string
  created_at: string
}

export type HealthResult = {
  status: string
  ping_status: string
  tcp_status: string
  ping_rtt_ms?: number
  last_check_at: string
}

export type ContainerStats = {
  cpu_perc: string
  mem_usage: string
  mem_perc: string
  net_io: string
  block_io: string
  pids: string
}

export type AuditEntry = {
  id: string
  created_at: string
  actor: string
  action: string
  server_id?: string
  server_name?: string
  details?: string
}

export type EditPayload = {
  name?: string
  host?: string
  ssh_port?: number
  ssh_user?: string
  ssh_auth_type?: 'password' | 'key'
  password?: string
  private_key?: string
  passphrase?: string
  mtproto_port?: number
  tags?: string[]
}

export type AuthSession = {
  username: string
  must_change_password: boolean
}

export type SSHPayload = {
  name?: string
  host: string
  ssh_port?: number
  ssh_user: string
  ssh_auth_type: 'password' | 'key'
  password?: string
  private_key?: string
  passphrase?: string
  proxy_type?: 'mtg' | 'tg-ws-proxy'
  mtproto_port?: number
  fake_tls?: boolean
  tags?: string[]
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
    ...init,
  })

  if (!res.ok) {
    let message = res.statusText
    try {
      const body = await res.json()
      if (body?.error) message = body.error
    } catch {
      // ignore
    }
    throw new Error(message)
  }

  if (res.status === 204) {
    return undefined as T
  }

  return res.json() as Promise<T>
}

export const api = {
  login: (username: string, password: string) =>
    request<AuthSession>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  logout: () => request<void>('/api/auth/logout', { method: 'POST' }),

  me: () => request<AuthSession>('/api/auth/me'),

  changePassword: (currentPassword: string, newPassword: string) =>
    request<AuthSession>('/api/auth/change-password', {
      method: 'POST',
      body: JSON.stringify({
        current_password: currentPassword,
        new_password: newPassword,
      }),
    }),

  listServers: () => request<Server[]>('/api/servers'),

  createServer: (payload: SSHPayload) =>
    request<{ id: string; deploy_status: string }>('/api/servers', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),

  deleteServer: (id: string) =>
    request<{ id: string; deploy_status: string }>(`/api/servers/${id}`, { method: 'DELETE' }),

  recreateServer: (id: string) =>
    request<{ id: string; deploy_status: string }>(`/api/servers/${id}/recreate`, {
      method: 'POST',
    }),

  checkHealth: (id: string) =>
    request<HealthResult>(`/api/servers/${id}/health`, {
      method: 'POST',
    }),

  serverLogs: (id: string, tail = 200) =>
    request<{ container: string; logs: string }>(`/api/servers/${id}/logs?tail=${tail}`),

  serverStats: (id: string) => request<ContainerStats>(`/api/servers/${id}/stats`),

  listAudit: () => request<AuditEntry[]>('/api/audit'),

  editServer: (id: string, payload: EditPayload) =>
    request<{ id: string; redeploying: boolean }>(`/api/servers/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(payload),
    }),

  qrUrl: (id: string) => `/api/servers/${id}/qr`,

  testSSH: (payload: SSHPayload) =>
    request<{ ok: boolean }>('/api/servers/test-ssh', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
}
