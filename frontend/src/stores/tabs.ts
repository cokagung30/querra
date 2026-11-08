import { create } from 'zustand'

// A query tab. Until the SQL editor lands (M2.3) it is only a placeholder;
// table and structure tabs join later. Titles are not stored: the UI renders
// them with t('tabs.query', { n: seq }).
export interface QueryTab {
  id: string
  kind: 'query'
  seq: number
}

export type Tab = QueryTab

interface TabsState {
  tabs: Tab[]
  activeId: string | null
  nextSeq: number
  openQueryTab: () => string
  // closeTab closes the given tab, or the active one.
  closeTab: (id?: string) => void
  activate: (id: string) => void
  activateNext: () => void
  activatePrevious: () => void
}

const initialState = { tabs: [] as Tab[], activeId: null as string | null, nextSeq: 1 }

export const useTabsStore = create<TabsState>()((set, get) => ({
  ...initialState,

  openQueryTab: () => {
    const { nextSeq } = get()
    const tab: QueryTab = { id: `query-${nextSeq}`, kind: 'query', seq: nextSeq }
    set((s) => ({ tabs: [...s.tabs, tab], activeId: tab.id, nextSeq: s.nextSeq + 1 }))
    return tab.id
  },

  closeTab: (id) => {
    const { tabs, activeId } = get()
    const target = id ?? activeId
    const index = tabs.findIndex((tab) => tab.id === target)
    if (index < 0) return
    const remaining = tabs.filter((tab) => tab.id !== target)
    // Closing the active tab activates its right neighbour, else its left one.
    const nextActive =
      target === activeId
        ? (remaining[Math.min(index, remaining.length - 1)]?.id ?? null)
        : activeId
    set({ tabs: remaining, activeId: nextActive })
  },

  activate: (id) => {
    if (get().tabs.some((tab) => tab.id === id)) set({ activeId: id })
  },

  activateNext: () => set((s) => ({ activeId: neighbour(s.tabs, s.activeId, 1) })),
  activatePrevious: () => set((s) => ({ activeId: neighbour(s.tabs, s.activeId, -1) })),
}))

// neighbour returns the id of the tab step positions away from activeId,
// wrapping around at either end.
function neighbour(tabs: Tab[], activeId: string | null, step: 1 | -1): string | null {
  if (tabs.length === 0) return null
  const index = tabs.findIndex((tab) => tab.id === activeId)
  if (index < 0) return (step === 1 ? tabs[0] : tabs[tabs.length - 1]).id
  return tabs[(index + step + tabs.length) % tabs.length].id
}

// resetTabsStore restores the empty state (tests).
export function resetTabsStore(): void {
  useTabsStore.setState(initialState)
}
