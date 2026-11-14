import { useEffect, useState } from 'react'

import { ready, type PerfMode } from './api'
import { afterPaint } from './paint'

// The first painted frame is reported once per page load. StrictMode runs
// effects twice in development, and PerfService.Ready has side effects (in
// startup mode it quits the app), so every mount shares one report.
let readyReport: Promise<PerfMode> | undefined

function reportReady(): Promise<PerfMode> {
  readyReport ??= afterPaint().then(ready)
  return readyReport
}

// usePerfAutorun reports the shell's first painted frame to PerfService and
// returns the dev/perf autorun mode ('' for normal use).
export function usePerfAutorun(): PerfMode {
  const [mode, setMode] = useState<PerfMode>('')

  useEffect(() => {
    let active = true
    reportReady()
      .then((m) => {
        if (active) setMode(m)
      })
      .catch((err: unknown) => console.error('perf: report ready:', err))
    return () => {
      active = false
    }
  }, [])

  return mode
}
