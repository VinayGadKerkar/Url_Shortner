import { useState } from 'react'
import { ShortenForm } from './components/ShortenForm'
import { ResultCard } from './components/ResultCard'
import { AnalyticsModal } from './components/AnalyticsModal'
import { HistoryPanel } from './components/HistoryPanel'
import { HealthBadge } from './components/HealthBadge'
import { addToHistory, getHistory, clearHistory } from './history'
import type { CreateURLResponse, HistoryItem } from './types'

export default function App() {
  const [result, setResult] = useState<CreateURLResponse | null>(null)
  const [analyticsCode, setAnalyticsCode] = useState<string | null>(null)
  const [history, setHistory] = useState<HistoryItem[]>(getHistory)

  const handleSuccess = (res: CreateURLResponse) => {
    setResult(res)
    const item: HistoryItem = {
      short_code: res.short_code,
      short_url: res.short_url,
      long_url: res.long_url,
      created_at: res.created_at,
      expires_at: res.expires_at,
    }
    addToHistory(item)
    setHistory(getHistory())
  }

  const handleClearHistory = () => {
    clearHistory()
    setHistory([])
  }

  return (
    <div className="min-h-screen bg-slate-950">
      {/* Background grid + gradient */}
      <div className="fixed inset-0 pointer-events-none">
        <div
          className="absolute inset-0 opacity-[0.03]"
          style={{
            backgroundImage:
              'linear-gradient(rgba(99,102,241,0.6) 1px, transparent 1px), linear-gradient(90deg, rgba(99,102,241,0.6) 1px, transparent 1px)',
            backgroundSize: '64px 64px',
          }}
        />
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[800px] h-[400px] bg-indigo-600/10 rounded-full blur-[100px]" />
      </div>

      {/* Navbar */}
      <header className="relative z-10 border-b border-slate-800/60 bg-slate-950/80 backdrop-blur-xl">
        <div className="max-w-2xl mx-auto px-4 h-14 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="w-7 h-7 rounded-lg bg-indigo-600 flex items-center justify-center">
              <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
            </div>
            <span className="font-semibold text-slate-100 tracking-tight">GoURL</span>
            <span className="text-slate-700 text-sm hidden sm:inline">URL Shortener</span>
          </div>
          <HealthBadge />
        </div>
      </header>

      {/* Main content */}
      <main className="relative z-10 max-w-2xl mx-auto px-4 py-12 space-y-8">
        {/* Hero */}
        <div className="text-center space-y-3 mb-10">
          <h1 className="text-4xl font-bold text-slate-100 tracking-tight">
            Shorten your links
          </h1>
          <p className="text-slate-500 max-w-sm mx-auto">
            Fast, reliable URL shortener with analytics. Paste a URL and get a short link in seconds.
          </p>
        </div>

        {/* Form card */}
        <div className="glass rounded-2xl p-6">
          <ShortenForm onSuccess={handleSuccess} />
        </div>

        {/* Result */}
        {result && (
          <ResultCard
            result={result}
            onViewAnalytics={setAnalyticsCode}
          />
        )}

        {/* History */}
        <HistoryPanel
          items={history}
          onViewAnalytics={setAnalyticsCode}
          onClear={handleClearHistory}
        />

        {/* Footer */}
        <footer className="text-center text-xs text-slate-700 pt-4">
          Powered by Go · PostgreSQL · Redis · Kafka
        </footer>
      </main>

      {/* Analytics modal */}
      <AnalyticsModal
        shortCode={analyticsCode}
        onClose={() => setAnalyticsCode(null)}
      />
    </div>
  )
}
