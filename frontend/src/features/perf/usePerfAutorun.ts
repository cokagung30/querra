import { useEffect, useState } from 'react'

import { ready, type PerfMode } from './api'
import { afterPaint } from './paint'

// usePerfAutorun reports the shell's first painted frame to PerfService and
// returns the dev/perf autorun mode ('' for normal use).
export function usePerfAutorun(): PerfMode {
  const [mode, setMode] = useState<PerfMode>('')

  useEffect(() => {
    let active = true
    afterPaint()
      .then(ready)
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
