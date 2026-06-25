// Persisted UI preferences (filters / sorting) and manual server order.
// Stored in localStorage so they survive page reloads and sessions.

const PREFS_KEY = 'mtprotoui.prefs.v1'
const ORDER_KEY = 'mtprotoui.order.v1'

export type Prefs = {
  tagFilter: string | null
  sortKey: string
  sortDir: string
}

export function loadPrefs(): Partial<Prefs> {
  try {
    const raw = localStorage.getItem(PREFS_KEY)
    return raw ? (JSON.parse(raw) as Partial<Prefs>) : {}
  } catch {
    return {}
  }
}

export function savePrefs(prefs: Prefs): void {
  try {
    localStorage.setItem(PREFS_KEY, JSON.stringify(prefs))
  } catch {
    // storage unavailable / quota — ignore
  }
}

export function loadOrder(): string[] {
  try {
    const raw = localStorage.getItem(ORDER_KEY)
    const parsed = raw ? JSON.parse(raw) : []
    return Array.isArray(parsed) ? (parsed as string[]) : []
  } catch {
    return []
  }
}

export function saveOrder(ids: string[]): void {
  try {
    localStorage.setItem(ORDER_KEY, JSON.stringify(ids))
  } catch {
    // ignore
  }
}
