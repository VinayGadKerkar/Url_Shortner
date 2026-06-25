import { CopyButton } from './CopyButton'
import type { CreateURLResponse } from '../types'

interface Props {
  result: CreateURLResponse
  onViewAnalytics: (shortCode: string) => void
}

function fmt(date: string) {
  return new Date(date).toLocaleString(undefined, {
    month: 'short', day: 'numeric', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

export function ResultCard({ result, onViewAnalytics }: Props) {
  const shortUrl = `${window.location.origin}/${result.short_code}`

  return (
    <div className="glass rounded-2xl p-5 animate-slide-up space-y-4">
      {/* Header */}
      <div className="flex items-center gap-2">
        <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-emerald-500/15">
          <svg className="w-4 h-4 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
          </svg>
        </span>
        <span className="text-sm font-medium text-emerald-400">URL shortened successfully</span>
      </div>

      {/* Short URL row */}
      <div className="flex items-center gap-2 bg-slate-800/80 border border-slate-700/60 rounded-xl px-4 py-3">
        <a
          href={shortUrl}
          target="_blank"
          rel="noreferrer"
          className="flex-1 font-mono text-indigo-400 hover:text-indigo-300 text-sm truncate transition-colors"
        >
          {shortUrl}
        </a>
        <CopyButton text={shortUrl} />
      </div>

      {/* Original URL */}
      <div>
        <p className="text-xs text-slate-500 mb-1">Original URL</p>
        <p className="text-sm text-slate-400 truncate">{result.long_url}</p>
      </div>

      {/* Meta row */}
      <div className="flex items-center justify-between text-xs text-slate-500 pt-1 border-t border-slate-800">
        <span>Created {fmt(result.created_at)}</span>
        {result.expires_at && (
          <span className="text-amber-500/80">Expires {fmt(result.expires_at)}</span>
        )}
        <button
          onClick={() => onViewAnalytics(result.short_code)}
          className="btn-ghost text-xs"
        >
          Analytics →
        </button>
      </div>
    </div>
  )
}
