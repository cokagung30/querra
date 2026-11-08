import { percentile, round } from './stats'

// A row as it crosses the bridge: one string per cell, null for SQL NULL.
export type Row = (string | null)[]

type FetchPage = (offset: number, limit: number) => Promise<Row[]>

export interface FetchStats {
  count: number
  avgMs: number
  p95Ms: number
  maxMs: number
}

// RowPageCache holds fixed-size pages of a result set fetched from Go and
// evicts the least recently used ones. React subscribes with
// useSyncExternalStore(cache.subscribe, cache.getVersion).
export class RowPageCache {
  private readonly pages = new Map<number, Row[]>()
  private readonly pending = new Set<number>()
  private readonly listeners = new Set<() => void>()
  private readonly latenciesMs: number[] = []
  private version = 0
  private readonly fetchPage: FetchPage
  readonly pageSize: number
  private readonly maxPages: number

  constructor(fetchPage: FetchPage, pageSize = 500, maxPages = 40) {
    this.fetchPage = fetchPage
    this.pageSize = pageSize
    this.maxPages = maxPages
  }

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  getVersion = (): number => this.version

  row(index: number): Row | undefined {
    return this.pages.get(Math.floor(index / this.pageSize))?.[index % this.pageSize]
  }

  // ensureRange loads every page that covers rows first..last.
  ensureRange(first: number, last: number): void {
    const lastPage = Math.floor(last / this.pageSize)
    for (let page = Math.floor(first / this.pageSize); page <= lastPage; page++) {
      const rows = this.pages.get(page)
      if (rows) {
        // Re-insert to mark the page as most recently used.
        this.pages.delete(page)
        this.pages.set(page, rows)
      } else if (!this.pending.has(page)) {
        void this.load(page)
      }
    }
  }

  // waitForRow resolves once the page holding index has loaded.
  waitForRow(index: number): Promise<void> {
    this.ensureRange(index, index)
    return new Promise((resolve) => {
      if (this.row(index)) return resolve()
      const unsubscribe = this.subscribe(() => {
        if (this.row(index)) {
          unsubscribe()
          resolve()
        }
      })
    })
  }

  fetchStats(): FetchStats {
    const total = this.latenciesMs.reduce((sum, ms) => sum + ms, 0)
    return {
      count: this.latenciesMs.length,
      avgMs: round(this.latenciesMs.length ? total / this.latenciesMs.length : 0),
      p95Ms: round(percentile(this.latenciesMs, 95)),
      maxMs: round(Math.max(0, ...this.latenciesMs)),
    }
  }

  private async load(page: number): Promise<void> {
    this.pending.add(page)
    const start = performance.now()
    try {
      const rows = await this.fetchPage(page * this.pageSize, this.pageSize)
      this.latenciesMs.push(performance.now() - start)
      this.pages.set(page, rows)
      for (const oldest of this.pages.keys()) {
        if (this.pages.size <= this.maxPages) break
        this.pages.delete(oldest)
      }
      this.version++
      this.listeners.forEach((listener) => listener())
    } catch (err: unknown) {
      console.error('fetch rows:', err)
    } finally {
      this.pending.delete(page)
    }
  }
}
