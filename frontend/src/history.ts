import type { HistoryItem } from './types'

const KEY = 'snip_history'
const MAX = 10

export function getHistory(): HistoryItem[] {
  try {
    return JSON.parse(localStorage.getItem(KEY) ?? '[]')
  } catch {
    return []
  }
}

export function addToHistory(item: HistoryItem): void {
  const history = getHistory().filter((h) => h.short_code !== item.short_code)
  history.unshift(item)
  localStorage.setItem(KEY, JSON.stringify(history.slice(0, MAX)))
}

export function clearHistory(): void {
  localStorage.removeItem(KEY)
}
