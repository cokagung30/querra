import { Database } from 'lucide-react'
import { useTranslation } from 'react-i18next'

// Sidebar holds the connection list and schema explorer (M1.4, M1.6). Anywhere
// a connection is listed here, a Production connection must be marked with
// the env-production color.
export function Sidebar() {
  const { t } = useTranslation()
  return (
    <nav
      aria-label={t('sidebar.label')}
      className="flex h-full flex-col bg-sidebar text-sidebar-foreground"
    >
      <h2 className="flex h-9 shrink-0 items-center px-3 text-xs font-semibold tracking-wide text-muted-foreground uppercase">
        {t('sidebar.label')}
      </h2>
      <div className="flex flex-1 flex-col items-center justify-center gap-1.5 px-4 text-center">
        <Database className="mb-1 size-6 text-muted-foreground/60" aria-hidden />
        <p className="text-sm font-medium">{t('sidebar.empty')}</p>
        <p className="text-xs text-muted-foreground">{t('sidebar.emptyHint')}</p>
      </div>
    </nav>
  )
}
