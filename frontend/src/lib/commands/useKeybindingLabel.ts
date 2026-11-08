import { useSyncExternalStore } from 'react'

import { formatKeybinding } from './keybinding'
import { commands as defaultRegistry, type CommandRegistry } from './registry'

// useKeybindingLabel returns the shortcut currently bound to a command,
// formatted for the platform (e.g. "⌘B" or "Ctrl+B"), or undefined. It
// re-renders when commands or overrides change.
export function useKeybindingLabel(
  commandId: string,
  registry: CommandRegistry = defaultRegistry,
): string | undefined {
  useSyncExternalStore(registry.subscribe, registry.getVersion)
  const binding = registry.keybinding(commandId)
  if (!binding) return undefined
  try {
    return formatKeybinding(binding, registry.platform)
  } catch {
    return undefined
  }
}
