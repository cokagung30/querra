import { useTranslation } from 'react-i18next'

import { ConnectionList } from '@/features/connections/ConnectionList'

// Sidebar holds the connection list, and the schema explorer from M1.6.
// Anywhere a connection is listed here, a Production connection must be marked
// with the env-production color.
export function Sidebar() {
  const { t } = useTranslation()
  return (
    <nav aria-label={t('sidebar.label')} className="h-full bg-sidebar text-sidebar-foreground">
      <ConnectionList />
    </nav>
  )
}
