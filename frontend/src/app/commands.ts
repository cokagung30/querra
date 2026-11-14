import { commands as defaultRegistry, type CommandRegistry } from '@/lib/commands'
import { useLayoutStore } from '@/stores/layout'
import { useTabsStore } from '@/stores/tabs'

// Default keybindings avoid what the OS or the macOS menu (internal/app/menu.go)
// already takes; commands.test.ts lists those keys.
export function registerShellCommands(registry: CommandRegistry = defaultRegistry): () => void {
  const tabs = () => useTabsStore.getState()
  const layout = () => useLayoutStore.getState()

  const unregister = [
    registry.register({
      id: 'view.toggleSidebar',
      titleKey: 'commands.view.toggleSidebar',
      defaultKeybinding: 'Mod+B',
      run: () => layout().toggleSidebar(),
    }),
    registry.register({
      id: 'view.toggleResults',
      titleKey: 'commands.view.toggleResults',
      defaultKeybinding: 'Mod+J',
      run: () => layout().toggleResults(),
    }),
    registry.register({
      id: 'tab.new',
      titleKey: 'commands.tab.new',
      defaultKeybinding: 'Mod+T',
      run: () => {
        tabs().openQueryTab()
      },
    }),
    registry.register({
      id: 'tab.close',
      titleKey: 'commands.tab.close',
      defaultKeybinding: 'Mod+W',
      // A tab's close button passes its id; the keybinding closes the active tab.
      run: (tabId) => tabs().closeTab(typeof tabId === 'string' ? tabId : undefined),
      isEnabled: () => tabs().tabs.length > 0,
    }),
    registry.register({
      id: 'tab.activate',
      titleKey: 'commands.tab.activate',
      // Run by clicking a tab, which passes its id; no default keybinding.
      run: (tabId) => {
        if (typeof tabId === 'string') tabs().activate(tabId)
      },
    }),
    registry.register({
      id: 'tab.next',
      titleKey: 'commands.tab.next',
      defaultKeybinding: 'Ctrl+Tab',
      run: () => tabs().activateNext(),
      isEnabled: () => tabs().tabs.length > 1,
    }),
    registry.register({
      id: 'tab.previous',
      titleKey: 'commands.tab.previous',
      defaultKeybinding: 'Ctrl+Shift+Tab',
      run: () => tabs().activatePrevious(),
      isEnabled: () => tabs().tabs.length > 1,
    }),
  ]
  return () => unregister.forEach((fn) => fn())
}
