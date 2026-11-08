import { lazy, Suspense } from 'react'

import { TooltipProvider } from '@/components/ui/tooltip'
import { usePerfAutorun } from '@/features/perf/usePerfAutorun'

import { AppShell } from './AppShell'

// Loaded only for QUERRA_PERF_AUTORUN=grid, so the benchmark grid stays out of
// the startup bundle.
const PerfGridBench = lazy(() => import('@/features/perf/PerfGridBench'))

export function App() {
  const perfMode = usePerfAutorun()

  return (
    <TooltipProvider delayDuration={500}>
      <AppShell />
      {perfMode === 'grid' && (
        <Suspense fallback={null}>
          <PerfGridBench />
        </Suspense>
      )}
    </TooltipProvider>
  )
}
