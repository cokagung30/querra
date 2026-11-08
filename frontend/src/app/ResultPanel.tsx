import { useTranslation } from 'react-i18next'

// ResultPanel shows the active tab's query results (M2.2).
export function ResultPanel() {
  const { t } = useTranslation()
  return (
    <section aria-label={t('results.label')} className="flex h-full flex-col">
      <h2 className="flex h-8 shrink-0 items-center border-b px-3 text-xs font-semibold tracking-wide text-muted-foreground uppercase">
        {t('results.label')}
      </h2>
      <div className="flex flex-1 items-center justify-center p-4 text-center text-sm text-muted-foreground">
        {t('results.empty')}
      </div>
    </section>
  )
}
