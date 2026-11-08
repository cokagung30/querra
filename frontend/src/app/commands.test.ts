import i18n from 'i18next'
import { describe, expect, it } from 'vitest'

import { CommandRegistry, parseKeybinding, type Platform } from '@/lib/commands'
import { chordId } from '@/lib/commands/keybinding'

import { registerShellCommands } from './commands'

// Taken by the OS or by the macOS menu in internal/app/menu.go, so a default
// keybinding there would never reach the webview.
const RESERVED: Record<Platform, string[]> = {
  mac: [
    'Cmd+Q',
    'Cmd+H',
    'Cmd+Alt+H',
    'Cmd+M',
    'Cmd+Z',
    'Cmd+Shift+Z',
    'Cmd+X',
    'Cmd+C',
    'Cmd+V',
    'Cmd+A',
  ],
  other: ['Alt+F4', 'Ctrl+C', 'Ctrl+V', 'Ctrl+X', 'Ctrl+Z', 'Ctrl+A'],
}

describe.each<Platform>(['mac', 'other'])('shell commands on %s', (platform) => {
  const registry = new CommandRegistry(platform)
  registerShellCommands(registry)

  it('have translated titles', () => {
    for (const command of registry.list()) {
      expect(i18n.exists(command.titleKey), command.titleKey).toBe(true)
    }
  })

  it('have distinct default keybindings', () => {
    expect(registry.conflicts()).toEqual([])
  })

  it('avoid keybindings the OS or the native menu takes', () => {
    const reserved = new Set(RESERVED[platform].map((b) => chordId(parseKeybinding(b, platform))))
    for (const command of registry.list()) {
      const binding = registry.keybinding(command.id)
      if (binding) {
        expect(reserved.has(chordId(parseKeybinding(binding, platform))), command.id).toBe(false)
      }
    }
  })
})
