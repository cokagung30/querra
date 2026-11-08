import { useCallback, useEffect } from 'react'
import { useDefaultLayout, usePanelRef } from 'react-resizable-panels'

import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '@/components/ui/resizable'
import { uiStorage } from '@/lib/storage'
import { useLayoutStore } from '@/stores/layout'

import { ContentArea } from './ContentArea'
import { ResultPanel } from './ResultPanel'
import { Sidebar } from './Sidebar'
import { StatusBar } from './StatusBar'
import { TabBar } from './TabBar'

// AppShell lays out the window: sidebar | (tab bar, content above results),
// with a status bar along the bottom. Panel sizes are remembered between runs;
// the sidebar and the results panel collapse with their commands or by
// dragging them closed.
export function AppShell() {
  const sidebarCollapsed = useLayoutStore((s) => s.sidebarCollapsed)
  const setSidebarCollapsed = useLayoutStore((s) => s.setSidebarCollapsed)
  const resultsCollapsed = useLayoutStore((s) => s.resultsCollapsed)
  const setResultsCollapsed = useLayoutStore((s) => s.setResultsCollapsed)

  const sidebar = useCollapsiblePanel(sidebarCollapsed, setSidebarCollapsed)
  const results = useCollapsiblePanel(resultsCollapsed, setResultsCollapsed)
  const mainLayout = useDefaultLayout({
    id: 'querra.shell.main',
    onlySaveAfterUserInteractions: true,
    storage: uiStorage,
  })
  const workLayout = useDefaultLayout({
    id: 'querra.shell.work',
    onlySaveAfterUserInteractions: true,
    storage: uiStorage,
  })

  return (
    <div className="flex h-screen flex-col overflow-hidden select-none">
      <ResizablePanelGroup
        orientation="horizontal"
        id="shell-main"
        className="min-h-0 flex-1"
        defaultLayout={mainLayout.defaultLayout}
        onLayoutChanged={mainLayout.onLayoutChanged}
      >
        <ResizablePanel
          id="sidebar"
          panelRef={sidebar.panelRef}
          onResize={sidebar.onResize}
          collapsible
          collapsedSize="0%"
          defaultSize="20%"
          minSize="180px"
          maxSize="40%"
        >
          <Sidebar />
        </ResizablePanel>
        <ResizableHandle />
        <ResizablePanel id="work" minSize="320px">
          <div className="flex h-full flex-col">
            <TabBar />
            <ResizablePanelGroup
              orientation="vertical"
              id="shell-work"
              className="min-h-0 flex-1"
              defaultLayout={workLayout.defaultLayout}
              onLayoutChanged={workLayout.onLayoutChanged}
            >
              <ResizablePanel id="content" minSize="120px">
                <ContentArea />
              </ResizablePanel>
              <ResizableHandle />
              <ResizablePanel
                id="results"
                panelRef={results.panelRef}
                onResize={results.onResize}
                collapsible
                collapsedSize="0%"
                defaultSize="35%"
                minSize="100px"
              >
                <ResultPanel />
              </ResizablePanel>
            </ResizablePanelGroup>
          </div>
        </ResizablePanel>
      </ResizablePanelGroup>
      <StatusBar />
    </div>
  )
}

// useCollapsiblePanel keeps a collapsible panel and the store's collapsed flag
// in step: the flag drives the panel, and dragging the panel closed (or open)
// updates the flag.
function useCollapsiblePanel(collapsed: boolean, setCollapsed: (collapsed: boolean) => void) {
  const panelRef = usePanelRef()

  useEffect(() => {
    const panel = panelRef.current
    if (!panel) return
    if (collapsed && !panel.isCollapsed()) panel.collapse()
    else if (!collapsed && panel.isCollapsed()) panel.expand()
  }, [collapsed, panelRef])

  const onResize = useCallback(() => {
    const panel = panelRef.current
    if (panel) setCollapsed(panel.isCollapsed())
  }, [panelRef, setCollapsed])

  return { panelRef, onResize }
}
