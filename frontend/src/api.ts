import type {
  AnalyticsResponse,
  CreateURLRequest,
  CreateURLResponse,
  HealthResponse,
} from './types'

const BASE = '/api/v1'

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body?.error ?? `Request failed: ${res.status}`)
  }

  return res.json() as Promise<T>
}

export const api = {
  shorten(payload: CreateURLRequest): Promise<CreateURLResponse> {
    return request<CreateURLResponse>(`${BASE}/shorten`, {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  },

  analytics(shortCode: string): Promise<AnalyticsResponse> {
    return request<AnalyticsResponse>(`${BASE}/analytics/${shortCode}`)
  },

  health(): Promise<HealthResponse> {
    return request<HealthResponse>('/health')
  },
}
