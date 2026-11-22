import { QueryClientProvider } from '@tanstack/react-query'
import { lazy, Suspense } from 'react'

import { TooltipProvider } from '@/components/ui/tooltip'
import { ConnectionDialogs } from '@/features/connections/ConnectionDialogs'
import { usePerfAutorun } from '@/features/perf/usePerfAutorun'
import { queryClient } from '@/lib/query'

import { AppShell } from './AppShell'

// Loaded only for QUERRA_PERF_AUTORUN=grid, so the benchmark grid stays out of
// the startup bundle.
const PerfGridBench = lazy(() => import('@/features/perf/PerfGridBench'))

export function App() {
  const perfMode = usePerfAutorun()

  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider delayDuration={500}>
        <AppShell />
        <ConnectionDialogs />
        {perfMode === 'grid' && (
          <Suspense fallback={null}>
            <PerfGridBench />
          </Suspense>
        )}
      </TooltipProvider>
    </QueryClientProvider>
  )
}
