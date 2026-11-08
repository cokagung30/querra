import { PanelBottom, PanelLeft } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useLayoutStore } from '@/stores/layout'

import { getAppInfo, type AppInfo } from './api'
import { CommandButton } from './CommandButton'

// StatusBar shows the active connection (with a PRODUCTION badge once
// connections exist, PLAN §3.7), the app version, and panel toggles.
export function StatusBar() {
  const { t } = useTranslation()
  const [info, setInfo] = useState<AppInfo | null>(null)
  const sidebarCollapsed = useLayoutStore((s) => s.sidebarCollapsed)
  const resultsCollapsed = useLayoutStore((s) => s.resultsCollapsed)

  useEffect(() => {
    getAppInfo()
      .then(setInfo)
      .catch((err: unknown) => console.error('load app info:', err))
  }, [])

  return (
    <footer
      aria-label={t('status.label')}
      className="flex h-7 shrink-0 items-center gap-3 border-t bg-sidebar px-2 text-xs text-muted-foreground"
    >
      <span className="flex items-center gap-1.5 pl-1">
        <span className="size-2 rounded-full bg-muted-foreground/40" aria-hidden />
        {t('status.notConnected')}
      </span>
      <span className="ml-auto tabular-nums">
        {info && t('status.version', { version: info.version })}
      </span>
      <span className="flex items-center">
        <CommandButton
          commandId="view.toggleSidebar"
          icon={PanelLeft}
          pressed={!sidebarCollapsed}
        />
        <CommandButton
          commandId="view.toggleResults"
          icon={PanelBottom}
          pressed={!resultsCollapsed}
        />
      </span>
    </footer>
  )
}
