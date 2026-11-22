import type { ParseKeys } from 'i18next'
import { KeyRound, Lock } from 'lucide-react'
import type { HTMLAttributes } from 'react'
import { useTranslation } from 'react-i18next'

import { commands } from '@/lib/commands'
import { cn } from '@/lib/utils'
import { useConnectionsStore } from '@/stores/connections'

import { Env, PasswordState, type Connection, type Settings } from './api'
import { dotToken } from './model'

// address is the row's second line: host[:port]/database, or just the
// database (a file path) for an engine without a host.
function address(s: Settings): string {
  if (!s.host) return s.database
  return `${s.host}${s.port ? `:${s.port}` : ''}/${s.database}`
}

// ConnectionRow is one connection in the sidebar. A click selects it; Enter
// (a keyboard click) or a double-click edits it until M1.5 makes that connect.
// dragHandle carries dnd-kit's attributes and listeners for the row.
export function ConnectionRow({
  connection,
  dragHandle,
}: {
  connection: Connection
  dragHandle?: HTMLAttributes<HTMLButtonElement>
}) {
  const { t } = useTranslation()
  const s = connection.settings
  const selected = useConnectionsStore((st) => st.selectedId === connection.id)
  const select = useConnectionsStore((st) => st.select)
  const production = s.env === Env.EnvProduction
  const edit = () => commands.execute('connection.edit', connection.id)

  return (
    <button
      {...dragHandle}
      type="button"
      data-env={s.env}
      className={cn(
        'flex w-full flex-col items-start gap-0.5 rounded-md border-l-2 px-2 py-1.5 text-left outline-none',
        'hover:bg-sidebar-accent focus-visible:ring-2 focus-visible:ring-ring',
        production ? 'border-env-production' : 'border-transparent',
        selected && 'bg-sidebar-accent',
      )}
      // detail is 0 for a click from the keyboard (Enter on the focused row).
      // A mouse click also focuses the row, which WKWebView does not do for
      // buttons, so list keybindings such as Delete apply to it.
      onClick={(e) => (e.detail === 0 ? edit() : e.currentTarget.focus())}
      // The selection follows focus, from a click or from Tab.
      onFocus={() => select(connection.id)}
      onDoubleClick={edit}
    >
      <span className="flex w-full min-w-0 items-center gap-1.5">
        <span
          aria-hidden
          className="size-2 shrink-0 rounded-full"
          style={{ background: `var(${dotToken(s)})` }}
        />
        <span className="truncate text-sm" title={s.name}>
          {s.name}
        </span>
        {production ? (
          <span className="shrink-0 rounded-sm bg-env-production/15 px-1 text-[10px] leading-4 font-semibold text-env-production">
            {t('connections.env.production')}
          </span>
        ) : (
          <span className="shrink-0 text-xs text-muted-foreground">
            {t(`connections.env.${s.env}` as ParseKeys)}
          </span>
        )}
        {s.readOnly && (
          <Lock
            role="img"
            aria-label={t('connections.readOnly')}
            className="size-3 shrink-0 text-muted-foreground"
          />
        )}
        {connection.passwordState === PasswordState.PasswordRequired && (
          <KeyRound
            role="img"
            aria-label={t('connections.passwordRequired')}
            className="size-3 shrink-0 text-muted-foreground"
          />
        )}
      </span>
      <span
        className="w-full truncate pl-3.5 font-mono text-xs text-muted-foreground"
        title={address(s)}
      >
        {address(s)}
      </span>
    </button>
  )
}
