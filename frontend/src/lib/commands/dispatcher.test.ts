import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { installKeybindings } from './dispatcher'
import { CommandRegistry, type Command } from './registry'

describe('installKeybindings', () => {
  let registry: CommandRegistry
  let uninstall: () => void
  let run: ReturnType<typeof vi.fn<Command['run']>>

  beforeEach(() => {
    registry = new CommandRegistry('other')
    run = vi.fn<Command['run']>()
    registry.register({
      id: 'tab.new',
      titleKey: 'commands.tab.new',
      defaultKeybinding: 'Mod+T',
      run,
    })
    uninstall = installKeybindings(registry)
  })

  afterEach(() => {
    uninstall()
    document.body.innerHTML = ''
  })

  function press(target: EventTarget, init: KeyboardEventInit) {
    const event = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init })
    target.dispatchEvent(event)
    return event
  }

  it('runs the bound command and prevents the default action', () => {
    const event = press(document.body, { key: 't', code: 'KeyT', ctrlKey: true })
    expect(run).toHaveBeenCalledOnce()
    expect(run).toHaveBeenCalledWith()
    expect(event.defaultPrevented).toBe(true)
  })

  it('leaves unbound keys alone', () => {
    const event = press(document.body, { key: 'y', code: 'KeyY', ctrlKey: true })
    expect(run).not.toHaveBeenCalled()
    expect(event.defaultPrevented).toBe(false)
  })

  it('skips events a widget already handled', () => {
    const input = document.body.appendChild(document.createElement('div'))
    input.addEventListener('keydown', (e) => e.preventDefault())
    press(input, { key: 't', code: 'KeyT', ctrlKey: true })
    expect(run).not.toHaveBeenCalled()
  })

  it('lets typing through in text fields but still runs modifier chords', () => {
    registry.register({
      id: 'plain.t',
      titleKey: 'commands.tab.new',
      defaultKeybinding: 'Shift+T',
      run: vi.fn(),
    })
    const input = document.body.appendChild(document.createElement('input'))

    const typed = press(input, { key: 'T', code: 'KeyT', shiftKey: true })
    expect(typed.defaultPrevented).toBe(false)
    expect(registry.get('plain.t')?.run).not.toHaveBeenCalled()

    press(input, { key: 't', code: 'KeyT', ctrlKey: true })
    expect(run).toHaveBeenCalledOnce()
  })

  it('ignores AltGr, which types characters on Windows', () => {
    registry.setOverrides({ 'tab.new': 'Ctrl+Alt+Q' })
    const event = new KeyboardEvent('keydown', {
      bubbles: true,
      cancelable: true,
      key: '@',
      code: 'KeyQ',
      ctrlKey: true,
      altKey: true,
    })
    vi.spyOn(event, 'getModifierState').mockImplementation((k) => k === 'AltGraph')
    document.body.dispatchEvent(event)
    expect(run).not.toHaveBeenCalled()
  })

  it('does not swallow the keybinding of a disabled command', () => {
    registry.register({
      id: 'tab.close',
      titleKey: 'commands.tab.close',
      defaultKeybinding: 'Mod+W',
      run: vi.fn(),
      isEnabled: () => false,
    })
    const event = press(document.body, { key: 'w', code: 'KeyW', ctrlKey: true })
    expect(event.defaultPrevented).toBe(false)
  })

  it('stops listening once uninstalled', () => {
    uninstall()
    press(document.body, { key: 't', code: 'KeyT', ctrlKey: true })
    expect(run).not.toHaveBeenCalled()
  })
})
