export interface CreateURLRequest {
  long_url: string
  custom_alias?: string
  expires_in_hours?: number
}

export interface CreateURLResponse {
  id: string
  short_code: string
  short_url: string
  long_url: string
  created_at: string
  expires_at?: string
}

export interface AnalyticsResponse {
  short_code: string
  long_url: string
  click_count: number
  created_at: string
  expires_at?: string
  last_accessed_at?: string
}

export interface HealthResponse {
  status: 'ok' | 'degraded'
  components: {
    postgres: 'healthy' | 'unhealthy'
    redis: 'healthy' | 'unhealthy'
  }
}

// Stored in localStorage
export interface HistoryItem {
  short_code: string
  short_url: string
  long_url: string
  created_at: string
  expires_at?: string
}
