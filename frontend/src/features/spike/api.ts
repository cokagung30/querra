// Backend calls for the M0.2 spike. Temporary: removed with internal/spike.
import { Events } from '@wailsio/runtime'

import { SpikeCancelResult } from '../../../bindings/events'
import {
  SpikeService,
  type DatasetInfo,
} from '../../../bindings/github.com/cokagung30/querra/internal/app'
import type {
  CancelResult,
  KeychainStep,
  MemStats,
} from '../../../bindings/github.com/cokagung30/querra/internal/spike'

import type { Row } from './pageCache'

export type { CancelResult, DatasetInfo, KeychainStep, MemStats }

export const autorunMode = () => SpikeService.AutorunMode()
export const reportReady = () => SpikeService.ReportReady()
export const openDataset = () => SpikeService.OpenDataset()
export const memStats = () => SpikeService.MemStats()
export const finish = () => SpikeService.Finish()
export const keychainWrite = (value: string) => SpikeService.KeychainWrite(value)
export const keychainRead = () => SpikeService.KeychainRead()
export const keychainDelete = () => SpikeService.KeychainDelete()

export function report(event: string, data: Record<string, unknown>): Promise<void> {
  return SpikeService.Report(event, data)
}

export async function fetchRows(offset: number, limit: number): Promise<Row[]> {
  const rows = await SpikeService.FetchRows(offset, limit)
  return (rows ?? []).map((row) => row ?? [])
}

// Starts a 30-second server-side sleep; cancel() the returned promise to test
// cancellation. A cancelled promise discards its result, so the outcome
// arrives through onCancelResult instead.
export function sleepAndObserve(engine: string) {
  return SpikeService.SleepAndObserve(engine)
}

export function onCancelResult(callback: (result: CancelResult) => void): () => void {
  return Events.On(SpikeCancelResult, (event) => callback(event.data))
}
