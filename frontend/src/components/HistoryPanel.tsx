import { CopyButton } from './CopyButton'
import type { HistoryItem } from '../types'

interface Props {
  items: HistoryItem[]
  onViewAnalytics: (shortCode: string) => void
  onClear: () => void
}

function timeAgo(date: string) {
  const diff = Date.now() - new Date(date).getTime()
  const mins = Math.floor(diff / 60_000)
  const hours = Math.floor(diff / 3_600_000)
  const days = Math.floor(diff / 86_400_000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  if (hours < 24) return `${hours}h ago`
  return `${days}d ago`
}

export function HistoryPanel({ items, onViewAnalytics, onClear }: Props) {
  if (items.length === 0) return null

  return (
    <div className="space-y-3">
      {/* Panel header */}
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider">
          Recent
        </h2>
        <button onClick={onClear} className="btn-ghost text-xs text-slate-600 hover:text-rose-400">
          Clear all
        </button>
      </div>

      {/* Items */}
      <div className="space-y-2">
        {items.map((item) => {
          const shortUrl = `${window.location.origin}/${item.short_code}`
          return (
            <div
              key={item.short_code}
              className="glass rounded-xl px-4 py-3 flex items-center gap-3 group"
            >
              {/* Icon */}
              <div className="flex-shrink-0 w-7 h-7 rounded-lg bg-slate-700/60 flex items-center justify-center">
                <svg className="w-3.5 h-3.5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round"
                    d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                </svg>
              </div>

              {/* Content */}
              <div className="flex-1 min-w-0">
                <a
                  href={shortUrl}
                  target="_blank"
                  rel="noreferrer"
                  className="block font-mono text-sm text-indigo-400 hover:text-indigo-300 transition-colors truncate"
                >
                  /{item.short_code}
                </a>
                <p className="text-xs text-slate-600 truncate">{item.long_url}</p>
              </div>

              {/* Actions */}
              <div className="flex items-center gap-1 flex-shrink-0">
                <span className="text-xs text-slate-600 mr-1">{timeAgo(item.created_at)}</span>
                <CopyButton text={shortUrl} />
                <button
                  onClick={() => onViewAnalytics(item.short_code)}
                  className="btn-ghost p-1.5"
                  aria-label="View analytics"
                  title="View analytics"
                >
                  <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round"
                      d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                  </svg>
                </button>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
