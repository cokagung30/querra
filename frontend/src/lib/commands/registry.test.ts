import { describe, expect, it, vi } from 'vitest'

import { parseKeybinding } from './keybinding'
import { CommandRegistry, type Command } from './registry'

function command(id: string, defaultKeybinding?: string, extra: Partial<Command> = {}): Command {
  return { id, titleKey: 'commands.tab.new', defaultKeybinding, run: vi.fn(), ...extra }
}

describe('CommandRegistry', () => {
  it('finds a command by the chord of its keybinding', () => {
    const registry = new CommandRegistry('other')
    const cmd = command('tab.new', 'Mod+T')
    registry.register(cmd)

    expect(registry.commandForChord(parseKeybinding('Ctrl+T', 'other'))).toBe(cmd)
    expect(registry.commandForChord(parseKeybinding('Ctrl+Y', 'other'))).toBeUndefined()
  })

  it('rejects duplicate ids', () => {
    const registry = new CommandRegistry('other')
    registry.register(command('tab.new'))
    expect(() => registry.register(command('tab.new'))).toThrow(/already registered/)
  })

  it('unregisters', () => {
    const registry = new CommandRegistry('other')
    const unregister = registry.register(command('tab.new', 'Mod+T'))
    unregister()
    expect(registry.get('tab.new')).toBeUndefined()
    expect(registry.commandForChord(parseKeybinding('Ctrl+T', 'other'))).toBeUndefined()
  })

  it('executes only enabled commands and passes the argument', () => {
    const registry = new CommandRegistry('other')
    let enabled = false
    const cmd = command('tab.close', undefined, { isEnabled: () => enabled })
    registry.register(cmd)

    expect(registry.execute('tab.close', 'query-1')).toBe(false)
    enabled = true
    expect(registry.execute('tab.close', 'query-1')).toBe(true)
    expect(cmd.run).toHaveBeenCalledWith('query-1')
    expect(registry.execute('missing')).toBe(false)
  })

  it('applies overrides, including unbinding', () => {
    const registry = new CommandRegistry('mac')
    const newTab = command('tab.new', 'Mod+T')
    const close = command('tab.close', 'Mod+W')
    registry.register(newTab)
    registry.register(close)

    registry.setOverrides({ 'tab.new': 'Mod+Shift+N', 'tab.close': null })
    expect(registry.keybinding('tab.new')).toBe('Mod+Shift+N')
    expect(registry.keybinding('tab.close')).toBeUndefined()
    expect(registry.commandForChord(parseKeybinding('Cmd+Shift+N', 'mac'))).toBe(newTab)
    expect(registry.commandForChord(parseKeybinding('Cmd+T', 'mac'))).toBeUndefined()
    expect(registry.commandForChord(parseKeybinding('Cmd+W', 'mac'))).toBeUndefined()
  })

  it('reports conflicts and keeps the first registered command', () => {
    const registry = new CommandRegistry('other')
    const first = command('a.first', 'Mod+K')
    registry.register(first)
    registry.register(command('b.second', 'Ctrl+K'))

    expect(registry.conflicts()).toEqual([{ chord: 'ctrl+k', commandIds: ['a.first', 'b.second'] }])
    expect(registry.commandForChord(parseKeybinding('Ctrl+K', 'other'))).toBe(first)
  })

  it('ignores an invalid override instead of throwing', () => {
    const registry = new CommandRegistry('other')
    registry.register(command('tab.new', 'Mod+T'))
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    registry.setOverrides({ 'tab.new': 'Mod+Banana' })
    expect(warn).toHaveBeenCalled()
    expect(registry.commandForChord(parseKeybinding('Ctrl+T', 'other'))).toBeUndefined()
    warn.mockRestore()
  })

  it('notifies subscribers on every change', () => {
    const registry = new CommandRegistry('other')
    const listener = vi.fn()
    registry.subscribe(listener)
    const before = registry.getVersion()
    registry.register(command('tab.new', 'Mod+T'))
    registry.setOverrides({})
    expect(listener).toHaveBeenCalledTimes(2)
    expect(registry.getVersion()).toBe(before + 2)
  })
})
