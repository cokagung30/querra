import { lazy, Suspense, useCallback, useEffect, useRef, useState, type ReactNode } from 'react'

import { Button } from '@/components/ui/button'

import {
  autorunMode,
  fetchRows,
  finish,
  keychainDelete,
  keychainRead,
  keychainWrite,
  memStats,
  onCancelResult,
  openDataset,
  report,
  reportReady,
  sleepAndObserve,
  type CancelResult,
  type DatasetInfo,
} from './api'
import { runGridBench } from './bench'
import { RowPageCache } from './pageCache'
import { SpikeGrid } from './SpikeGrid'

const SqlEditor = lazy(() => import('./SqlEditor'))

const KEYCHAIN_DUMMY_VALUE = 'querra-spike-dummy-value'

// Engine plus cancellation strategy, matching internal/spike/cancel.go.
const CANCEL_PROBES = ['postgres', 'postgres-cancel-request', 'mysql', 'mysql-kill-query'] as const
type CancelProbe = (typeof CANCEL_PROBES)[number]

// Temporary M0.2 spike screen (docs/spikes/fase-0.md). Its strings are not
// translated because the screen is deleted in M0.3.
export function SpikePage() {
  const [dataset, setDataset] = useState<DatasetInfo | null>(null)
  const [cache] = useState(() => new RowPageCache(fetchRows))
  const [showEditor, setShowEditor] = useState(false)
  const [log, setLog] = useState<string[]>([])
  const scrollRef = useRef<HTMLDivElement>(null)

  const append = useCallback((line: string) => setLog((lines) => [...lines.slice(-30), line]), [])

  const loadDataset = useCallback(async () => {
    const info = await openDataset()
    setDataset(info)
    append(
      `dataset: ${info.rowCount} rows × ${info.columns?.length ?? 0} cols, generated in ${info.generateMs} ms`,
    )
    await cache.waitForRow(0)
    return info
  }, [append, cache])

  const runBench = useCallback(async () => {
    if (!scrollRef.current) return
    append('scroll benchmark running…')
    const phases = await runGridBench(scrollRef.current)
    const fetch = cache.fetchStats()
    for (const p of phases) {
      append(
        `${p.phase}: ${p.avgFps} fps, p95 ${p.p95FrameMs} ms, long ${p.longFrames}, blank ${p.blankFrames}`,
      )
    }
    append(`FetchRows: ${fetch.count} calls, avg ${fetch.avgMs} ms, p95 ${fetch.p95Ms} ms`)
    await report('grid', { phases, fetch })
  }, [append, cache])

  const runCancelTest = useCallback((engine: CancelProbe) => {
    const done = new Promise<CancelResult>((resolve) => {
      const off = onCancelResult((result) => {
        if (result.engine === engine) {
          off()
          resolve(result)
        }
      })
    })
    const call = sleepAndObserve(engine)
    setTimeout(() => call.cancel(), 1000)
    call.catch(() => {}) // a cancelled call rejects; the result arrives as an event
    return done
  }, [])

  useEffect(
    () =>
      onCancelResult((r) =>
        append(
          `cancel ${r.engine}: returned after ${r.elapsedMs} ms, still running on server: ${r.stillRunning}, session usable: ${r.sessionUsable}${r.err ? ` (${r.err})` : ''}`,
        ),
      ),
    [append],
  )

  // Report the first paint for the startup measurement, then run the
  // benchmark the dev/perf scripts asked for, if any.
  useEffect(() => {
    let active = true
    const run = async () => {
      await afterPaint()
      await reportReady()
      const mode = await autorunMode()
      if (!active || !mode) return

      switch (mode) {
        case 'grid':
          await loadDataset()
          await afterPaint()
          await report('grid-loaded', {})
          await runBench()
          break
        case 'cancel':
          for (const probe of CANCEL_PROBES) await runCancelTest(probe)
          break
        case 'keychain-write':
          await keychainWrite(KEYCHAIN_DUMMY_VALUE)
          break
        case 'keychain-read':
          await keychainRead()
          break
        case 'keychain-delete':
          await keychainDelete()
          break
        default:
          return // "startup" quits from Go; "idle" just stays open
      }
      await finish()
    }
    run().catch((err: unknown) => console.error('spike autorun:', err))
    return () => {
      active = false
    }
  }, [loadDataset, runBench, runCancelTest])

  const logKeychain = async (step: Promise<{ op: string; ms: number; err?: string }>) => {
    const s = await step
    append(`keychain ${s.op}: ${s.ms} ms${s.err ? ` — ${s.err}` : ''}`)
  }

  return (
    <div className="flex min-h-0 flex-1 gap-4">
      <div className="flex min-h-0 flex-1 flex-col gap-2">
        {dataset ? (
          <SpikeGrid dataset={dataset} cache={cache} scrollRef={scrollRef} />
        ) : (
          <div className="flex flex-1 items-center justify-center rounded-md border text-sm text-muted-foreground">
            Load the dataset to test the grid.
          </div>
        )}
        {showEditor && (
          <Suspense fallback={<div className="h-48 rounded-md border" />}>
            <SqlEditor />
          </Suspense>
        )}
      </div>

      <aside className="flex w-72 shrink-0 flex-col gap-3 text-sm">
        <Section title="Grid">
          <Button size="sm" disabled={dataset !== null} onClick={() => void loadDataset()}>
            Load 100k × 20
          </Button>
          <Button size="sm" variant="outline" disabled={!dataset} onClick={() => void runBench()}>
            Run scroll benchmark
          </Button>
        </Section>
        <Section title="Cancellation (sleep 30 s, cancel at 1 s)">
          {CANCEL_PROBES.map((probe) => (
            <Button
              key={probe}
              size="sm"
              variant="outline"
              onClick={() => void runCancelTest(probe)}
            >
              {probe}
            </Button>
          ))}
        </Section>
        <Section title="Keychain">
          <Button
            size="sm"
            variant="outline"
            onClick={() => void logKeychain(keychainWrite(KEYCHAIN_DUMMY_VALUE))}
          >
            Write
          </Button>
          <Button size="sm" variant="outline" onClick={() => void logKeychain(keychainRead())}>
            Read
          </Button>
          <Button size="sm" variant="outline" onClick={() => void logKeychain(keychainDelete())}>
            Delete
          </Button>
        </Section>
        <Section title="Other">
          <Button size="sm" variant="outline" onClick={() => setShowEditor((v) => !v)}>
            {showEditor ? 'Hide' : 'Show'} SQL editor
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              void memStats().then((m) =>
                append(`Go heap ${m.heapAllocMB.toFixed(1)} MB, sys ${m.sysMB.toFixed(1)} MB`),
              )
            }
          >
            Go memory
          </Button>
        </Section>
        <pre className="min-h-0 flex-1 overflow-auto rounded-md border bg-muted/40 p-2 text-[11px] leading-snug whitespace-pre-wrap">
          {log.join('\n')}
        </pre>
      </aside>
    </div>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-1.5">
      <h2 className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
        {title}
      </h2>
      <div className="flex flex-wrap gap-1.5">{children}</div>
    </section>
  )
}

function afterPaint(): Promise<void> {
  return new Promise((resolve) =>
    requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
  )
}
