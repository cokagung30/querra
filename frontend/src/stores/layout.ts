import { create } from 'zustand'

// Which shell panels are collapsed. The panels' sizes live in the resizable
// layout itself (persisted in the webview's localStorage); this store only
// mirrors the collapsed state so commands and buttons can toggle it.
interface LayoutState {
  sidebarCollapsed: boolean
  resultsCollapsed: boolean
  setSidebarCollapsed: (collapsed: boolean) => void
  setResultsCollapsed: (collapsed: boolean) => void
  toggleSidebar: () => void
  toggleResults: () => void
}

export const useLayoutStore = create<LayoutState>()((set) => ({
  sidebarCollapsed: false,
  resultsCollapsed: false,
  setSidebarCollapsed: (sidebarCollapsed) => set({ sidebarCollapsed }),
  setResultsCollapsed: (resultsCollapsed) => set({ resultsCollapsed }),
  toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
  toggleResults: () => set((s) => ({ resultsCollapsed: !s.resultsCollapsed })),
}))
