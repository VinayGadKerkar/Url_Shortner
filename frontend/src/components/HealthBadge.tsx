import { useEffect, useState } from 'react'
import { api } from '../api'
import type { HealthResponse } from '../types'

export function HealthBadge() {
  const [health, setHealth] = useState<HealthResponse | null>(null)
  const [checking, setChecking] = useState(true)

  useEffect(() => {
    let mounted = true
    const check = async () => {
      try {
        const h = await api.health()
        if (mounted) { setHealth(h); setChecking(false) }
      } catch {
        if (mounted) { setHealth(null); setChecking(false) }
      }
    }
    check()
    const interval = setInterval(check, 30_000)
    return () => { mounted = false; clearInterval(interval) }
  }, [])

  if (checking) {
    return (
      <span className="flex items-center gap-1.5 text-xs text-slate-500">
        <span className="w-1.5 h-1.5 rounded-full bg-slate-500 animate-pulse" />
        checking
      </span>
    )
  }

  const ok = health?.status === 'ok'
  return (
    <span
      className={`flex items-center gap-1.5 text-xs font-medium ${ok ? 'text-emerald-400' : 'text-rose-400'}`}
      title={ok ? 'All systems operational' : `postgres: ${health?.components.postgres ?? '?'} · redis: ${health?.components.redis ?? '?'}`}
    >
      <span className={`w-1.5 h-1.5 rounded-full ${ok ? 'bg-emerald-400' : 'bg-rose-400'}`} />
      {ok ? 'All systems up' : 'Degraded'}
    </span>
  )
}
