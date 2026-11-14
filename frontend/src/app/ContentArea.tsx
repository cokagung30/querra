import { useTranslation } from 'react-i18next'

import { Kbd } from '@/components/ui/kbd'
import { commands, useKeybindingLabel } from '@/lib/commands'
import { useTabsStore } from '@/stores/tabs'

// Commands the welcome screen lists with their shortcuts.
const WELCOME_COMMANDS = ['tab.new', 'view.toggleSidebar', 'view.toggleResults']

export function ContentArea() {
  const { t } = useTranslation()
  const activeTab = useTabsStore((s) => s.tabs.find((tab) => tab.id === s.activeId))

  if (!activeTab) return <Welcome />

  return (
    <div
      role="tabpanel"
      id="tab-panel"
      aria-labelledby={`tab-${activeTab.id}`}
      className="flex h-full items-center justify-center p-6 text-sm text-muted-foreground"
    >
      {t('query.placeholder')}
    </div>
  )
}

function Welcome() {
  const { t } = useTranslation()
  return (
    <div className="flex h-full flex-col items-center justify-center gap-8 p-6">
      <div className="text-center">
        <h1 className="text-2xl font-semibold tracking-tight">{t('app.name')}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t('app.tagline')}</p>
      </div>
      <section aria-labelledby="welcome-shortcuts" className="flex flex-col items-center gap-3">
        <h2
          id="welcome-shortcuts"
          className="text-xs font-semibold tracking-wide text-muted-foreground uppercase"
        >
          {t('welcome.shortcuts')}
        </h2>
        <dl className="grid grid-cols-[auto_auto] items-center gap-x-8 gap-y-2 text-sm">
          {WELCOME_COMMANDS.map((id) => (
            <ShortcutRow key={id} commandId={id} />
          ))}
        </dl>
      </section>
    </div>
  )
}

function ShortcutRow({ commandId }: { commandId: string }) {
  const { t } = useTranslation()
  // Also re-renders when commands change, which keeps commands.get current.
  const keybinding = useKeybindingLabel(commandId)
  const command = commands.get(commandId)
  if (!command || !keybinding) return null
  return (
    <>
      <dt className="text-muted-foreground">{t(command.titleKey)}</dt>
      <dd>
        <Kbd>{keybinding}</Kbd>
      </dd>
    </>
  )
}
