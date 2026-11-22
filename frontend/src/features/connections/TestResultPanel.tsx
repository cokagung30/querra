import { CircleCheck, CircleMinus, CircleX, LoaderCircle, Minus } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { errorText, type AppError } from '@/lib/apperr'
import { cn } from '@/lib/utils'

import { StageStatus, type TestResult } from './api'

export type TestState =
  | { status: 'idle' }
  | { status: 'running' }
  | { status: 'done'; result: TestResult }
  | { status: 'error'; error: AppError }

const ICONS = {
  [StageStatus.StageOK]: CircleCheck,
  [StageStatus.StageFailed]: CircleX,
  [StageStatus.StageSkipped]: CircleMinus,
  [StageStatus.StageNotRun]: Minus,
} as Record<string, typeof CircleCheck>

// TestResultPanel shows Test Connection's progress and result: each stage
// (TCP → TLS → login → ping) with its status, and for a failure the reason and
// a hint (PLAN §3.9).
export function TestResultPanel({ state }: { state: Exclude<TestState, { status: 'idle' }> }) {
  const { t } = useTranslation()

  if (state.status === 'running') {
    return (
      <div role="status" className="flex items-center gap-2 text-xs text-muted-foreground">
        <LoaderCircle aria-hidden className="size-3.5 animate-spin" />
        {t('connections.test.running')}
      </div>
    )
  }

  const result = state.status === 'done' ? state.result : undefined
  const error = state.status === 'error' ? state.error : result?.error
  const text = error ? errorText(error) : undefined

  return (
    <div role="status" className="grid gap-2 rounded-md border bg-background px-3 py-2 text-xs">
      {result && (
        <div className="flex items-center gap-4">
          <ul className="flex flex-1 flex-wrap items-center gap-x-4 gap-y-1">
            {(result.stages ?? []).map(({ stage, status }) => {
              const Icon = ICONS[status] ?? Minus
              return (
                <li key={stage} className="flex items-center gap-1">
                  <Icon
                    aria-hidden
                    className={cn(
                      'size-3.5',
                      status === StageStatus.StageOK && 'text-env-local',
                      status === StageStatus.StageFailed && 'text-destructive',
                      (status === StageStatus.StageSkipped || status === StageStatus.StageNotRun) &&
                        'text-muted-foreground',
                    )}
                  />
                  <span
                    data-status={status}
                    className={cn(
                      'font-medium',
                      status === StageStatus.StageFailed && 'text-destructive',
                      status !== StageStatus.StageOK &&
                        status !== StageStatus.StageFailed &&
                        'text-muted-foreground',
                    )}
                  >
                    {t(`connections.test.stages.${stage}` as 'connections.test.stages.tcp')}
                  </span>
                  {status === StageStatus.StageSkipped && (
                    <span className="text-muted-foreground">({t('connections.test.skipped')})</span>
                  )}
                  {status === StageStatus.StageNotRun && (
                    <span className="text-muted-foreground">({t('connections.test.notRun')})</span>
                  )}
                </li>
              )
            })}
          </ul>
          <span className="shrink-0 font-mono text-muted-foreground tabular-nums">
            {t('connections.test.duration', { ms: result.durationMs })}
          </span>
        </div>
      )}
      {result?.ok && (
        <p className="text-env-local">
          {result.serverVersion
            ? t('connections.test.okVersion', { version: result.serverVersion })
            : t('connections.test.ok')}{' '}
          <span className="text-muted-foreground">
            · {result.tls ? t('connections.test.tls') : t('connections.test.noTls')}
          </span>
        </p>
      )}
      {text && (
        <div>
          <p className="text-destructive">{text.message}</p>
          {text.hint && <p className="text-muted-foreground">{text.hint}</p>}
        </div>
      )}
    </div>
  )
}
