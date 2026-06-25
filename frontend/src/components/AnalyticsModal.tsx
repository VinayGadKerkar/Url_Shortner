import { useEffect, useState, useCallback } from 'react'
import { api } from '../api'
import type { AnalyticsResponse } from '../types'

interface Props {
  shortCode: string | null
  onClose: () => void
}

function fmt(date?: string | null) {
  if (!date) return '—'
  return new Date(date).toLocaleString(undefined, {
    month: 'short', day: 'numeric', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

function StatBox({ label, value, accent = false }: { label: string; value: string; accent?: boolean }) {
  return (
    <div className="bg-slate-800/60 border border-slate-700/50 rounded-xl p-4">
      <p className="text-xs text-slate-500 mb-1">{label}</p>
      <p className={`text-lg font-semibold ${accent ? 'text-indigo-400' : 'text-slate-100'}`}>{value}</p>
    </div>
  )
}

export function AnalyticsModal({ shortCode, onClose }: Props) {
  const [data, setData] = useState<AnalyticsResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async (code: string) => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.analytics(code)
      setData(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load analytics')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (shortCode) {
      setData(null)
      load(shortCode)
    }
  }, [shortCode, load])

  // Close on Escape
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  if (!shortCode) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-fade-in"
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
      role="dialog"
      aria-modal="true"
      aria-label="Analytics"
    >
      <div className="glass rounded-2xl w-full max-w-md animate-slide-up">
        {/* Modal header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800">
          <div className="flex items-center gap-2">
            <svg className="w-4 h-4 text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round"
                d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
            </svg>
            <h2 className="text-sm font-semibold text-slate-200">Analytics</h2>
            <span className="font-mono text-xs text-indigo-400 bg-indigo-500/10 px-2 py-0.5 rounded-md">
              /{shortCode}
            </span>
          </div>
          <button onClick={onClose} className="btn-ghost p-1.5" aria-label="Close">
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Body */}
        <div className="p-6">
          {loading && (
            <div className="flex justify-center items-center py-12">
              <svg className="w-6 h-6 text-indigo-400 animate-spin" viewBox="0 0 24 24" fill="none">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
              </svg>
            </div>
          )}

          {error && (
            <div className="text-sm text-rose-400 bg-rose-400/10 border border-rose-400/20 rounded-xl px-4 py-3">
              {error}
            </div>
          )}

          {data && !loading && (
            <div className="space-y-4 animate-fade-in">
              {/* Click count — hero stat */}
              <div className="bg-indigo-600/10 border border-indigo-500/20 rounded-xl p-5 text-center">
                <p className="text-xs text-indigo-400 mb-1 uppercase tracking-wider font-medium">Total Clicks</p>
                <p className="text-4xl font-bold text-indigo-300">{data.click_count.toLocaleString()}</p>
              </div>

              {/* Stat grid */}
              <div className="grid grid-cols-2 gap-3">
                <StatBox label="Created" value={fmt(data.created_at)} />
                <StatBox label="Last accessed" value={fmt(data.last_accessed_at)} />
                {data.expires_at && (
                  <StatBox label="Expires" value={fmt(data.expires_at)} />
                )}
              </div>

              {/* Original URL */}
              <div className="bg-slate-800/60 border border-slate-700/50 rounded-xl p-4">
                <p className="text-xs text-slate-500 mb-1.5">Destination URL</p>
                <a
                  href={data.long_url}
                  target="_blank"
                  rel="noreferrer"
                  className="text-sm text-slate-300 hover:text-indigo-400 break-all transition-colors"
                >
                  {data.long_url}
                </a>
              </div>

              {/* Refresh */}
              <button
                onClick={() => load(shortCode)}
                className="w-full btn-ghost text-sm justify-center flex items-center gap-1.5"
              >
                <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round"
                    d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                Refresh
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
