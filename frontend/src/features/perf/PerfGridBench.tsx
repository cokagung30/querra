import { useEffect, useRef, useState } from 'react'

import { fetchRows, finish, openDataset, report, type DatasetInfo } from './api'
import { runGridBench } from './bench'
import { afterPaint } from './paint'
import { RowPageCache } from './pageCache'
import { PerfGrid } from './PerfGrid'

// Grid scroll benchmark for QUERRA_PERF_AUTORUN=grid, loaded only in that mode.
// It covers the shell, and the spacers reproduce the grid size of the M0.2
// spike page so results stay comparable with docs/spikes/fase-0.md.
export default function PerfGridBench() {
  const [dataset, setDataset] = useState<DatasetInfo | null>(null)
  const [cache] = useState(() => new RowPageCache(fetchRows))
  const scrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    let active = true
    const load = async () => {
      const info = await openDataset()
      await cache.waitForRow(0)
      if (active) setDataset(info)
    }
    load().catch((err: unknown) => console.error('perf: open dataset:', err))
    return () => {
      active = false
    }
  }, [cache])

  useEffect(() => {
    if (!dataset) return
    let active = true
    const run = async () => {
      await afterPaint()
      if (!active || !scrollRef.current) return
      await report('grid-loaded', {})
      const phases = await runGridBench(scrollRef.current)
      await report('grid', { phases, fetch: cache.fetchStats() })
      await finish()
    }
    run().catch((err: unknown) => console.error('perf: grid benchmark:', err))
    return () => {
      active = false
    }
  }, [cache, dataset])

  return (
    <div className="fixed inset-0 z-50 flex gap-4 bg-background p-4 pt-14">
      <div className="flex min-h-0 flex-1 flex-col">
        {dataset && <PerfGrid dataset={dataset} cache={cache} scrollRef={scrollRef} />}
      </div>
      <div className="w-72 shrink-0" />
    </div>
  )
}
