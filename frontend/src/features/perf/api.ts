// Backend calls for the dev/perf measurements (PerfService). They do nothing
// unless the app was started with QUERRA_PERF_AUTORUN.
import {
  PerfService,
  type DatasetInfo,
} from '../../../bindings/github.com/cokagung30/querra/internal/app'

import type { Row } from './pageCache'

export type { DatasetInfo }

export type PerfMode = '' | 'startup' | 'idle' | 'grid'

// ready reports the shell's first painted frame and returns the autorun mode,
// or '' for normal use.
export async function ready(): Promise<PerfMode> {
  return (await PerfService.Ready()) as PerfMode
}

export const openDataset = () => PerfService.OpenDataset()
export const finish = () => PerfService.Finish()

export function report(event: string, data: Record<string, unknown>): Promise<void> {
  return PerfService.Report(event, data)
}

export async function fetchRows(offset: number, limit: number): Promise<Row[]> {
  const rows = await PerfService.FetchRows(offset, limit)
  return (rows ?? []).map((row) => row ?? [])
}
