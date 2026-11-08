import { FileCode, Plus, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { commands } from '@/lib/commands'
import { cn } from '@/lib/utils'
import { useTabsStore, type Tab } from '@/stores/tabs'

import { CommandButton } from './CommandButton'

export function TabBar() {
  const { t } = useTranslation()
  const tabs = useTabsStore((s) => s.tabs)
  const activeId = useTabsStore((s) => s.activeId)

  return (
    <div className="flex h-9 shrink-0 items-stretch border-b bg-muted/40">
      <div
        role="tablist"
        aria-label={t('tabs.label')}
        className="flex min-w-0 items-stretch overflow-x-auto [scrollbar-width:none]"
      >
        {tabs.map((tab) => (
          <TabItem key={tab.id} tab={tab} active={tab.id === activeId} />
        ))}
      </div>
      <div className="flex items-center px-1">
        <CommandButton commandId="tab.new" icon={Plus} />
      </div>
    </div>
  )
}

function TabItem({ tab, active }: { tab: Tab; active: boolean }) {
  const { t } = useTranslation()
  const activate = useTabsStore((s) => s.activate)
  const title = t('tabs.query', { n: tab.seq })

  return (
    <div
      className={cn(
        'group relative flex shrink-0 items-center border-r',
        active ? 'bg-background text-foreground' : 'text-muted-foreground hover:bg-muted',
      )}
    >
      {active && <span className="absolute inset-x-0 top-0 h-0.5 bg-primary" aria-hidden />}
      <button
        type="button"
        role="tab"
        id={`tab-${tab.id}`}
        aria-selected={active}
        aria-controls="tab-panel"
        className="flex h-full max-w-48 items-center gap-1.5 pr-1 pl-3 text-xs outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
        onClick={() => activate(tab.id)}
      >
        <FileCode className="size-3.5 shrink-0" aria-hidden />
        <span className="truncate">{title}</span>
      </button>
      <button
        type="button"
        aria-label={t('tabs.close', { title })}
        className={cn(
          'mr-1.5 rounded-sm p-0.5 outline-none hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring',
          active
            ? 'opacity-70 hover:opacity-100'
            : 'opacity-0 group-hover:opacity-70 focus-visible:opacity-100',
        )}
        onClick={() => commands.execute('tab.close', tab.id)}
      >
        <X className="size-3" aria-hidden />
      </button>
    </div>
  )
}
