import { describe, expect, it } from 'vitest'

import {
  chordFromEvent,
  chordId,
  detectPlatform,
  formatKeybinding,
  parseKeybinding,
} from './keybinding'

function key(init: Partial<KeyboardEvent> & Pick<KeyboardEvent, 'key' | 'code'>) {
  return { ctrlKey: false, altKey: false, shiftKey: false, metaKey: false, ...init }
}

describe('detectPlatform', () => {
  it('recognizes macOS webviews', () => {
    expect(detectPlatform('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit')).toBe(
      'mac',
    )
    expect(detectPlatform('Mozilla/5.0 (Windows NT 10.0; Win64; x64) Edg/157.0')).toBe('other')
  })
})

describe('parseKeybinding', () => {
  it('maps Mod to Cmd on macOS and Ctrl elsewhere', () => {
    expect(chordId(parseKeybinding('Mod+B', 'mac'))).toBe('meta+b')
    expect(chordId(parseKeybinding('Mod+B', 'other'))).toBe('ctrl+b')
  })

  it('keeps Ctrl as Control on macOS', () => {
    expect(chordId(parseKeybinding('Ctrl+Tab', 'mac'))).toBe('ctrl+tab')
  })

  it('accepts aliases and any case, in a canonical order', () => {
    expect(chordId(parseKeybinding('shift+cmd+option+Esc', 'mac'))).toBe('alt+shift+meta+escape')
    expect(chordId(parseKeybinding('Mod+Shift+]', 'other'))).toBe('ctrl+shift+]')
  })

  it.each(['Mod+', 'Mod+Shift', 'Hyper+B', 'Mod+Banana', ''])('rejects %j', (binding) => {
    expect(() => parseKeybinding(binding, 'other')).toThrow()
  })
})

describe('chordFromEvent', () => {
  it('ignores bare modifier presses', () => {
    expect(chordFromEvent(key({ key: 'Shift', code: 'ShiftLeft', shiftKey: true }))).toBeNull()
    expect(chordFromEvent(key({ key: 'Meta', code: 'MetaLeft', metaKey: true }))).toBeNull()
  })

  it('lowercases shifted letters', () => {
    const chord = chordFromEvent(key({ key: 'B', code: 'KeyB', ctrlKey: true, shiftKey: true }))
    expect(chord && chordId(chord)).toBe('ctrl+shift+b')
  })

  it('follows the layout for Latin letters (AZERTY A is on the Q key)', () => {
    const chord = chordFromEvent(key({ key: 'a', code: 'KeyQ', ctrlKey: true }))
    expect(chord && chordId(chord)).toBe('ctrl+a')
  })

  it('falls back to the physical key when the layout gives no Latin letter', () => {
    const cyrillic = chordFromEvent(key({ key: 'и', code: 'KeyB', ctrlKey: true }))
    expect(cyrillic && chordId(cyrillic)).toBe('ctrl+b')
    // Option+B types "∫" on a US Mac.
    const option = chordFromEvent(key({ key: '∫', code: 'KeyB', altKey: true, metaKey: true }))
    expect(option && chordId(option)).toBe('alt+meta+b')
  })

  it('reads digits and symbols from the physical key, so Shift does not change them', () => {
    const digit = chordFromEvent(key({ key: '!', code: 'Digit1', ctrlKey: true, shiftKey: true }))
    expect(digit && chordId(digit)).toBe('ctrl+shift+1')
    const bracket = chordFromEvent(
      key({ key: '}', code: 'BracketRight', metaKey: true, shiftKey: true }),
    )
    expect(bracket && chordId(bracket)).toBe('shift+meta+]')
  })

  it('names special keys', () => {
    const tab = chordFromEvent(key({ key: 'Tab', code: 'Tab', ctrlKey: true }))
    expect(tab && chordId(tab)).toBe('ctrl+tab')
    const space = chordFromEvent(key({ key: ' ', code: 'Space', ctrlKey: true }))
    expect(space && chordId(space)).toBe('ctrl+space')
    const f5 = chordFromEvent(key({ key: 'F5', code: 'F5' }))
    expect(f5 && chordId(f5)).toBe('f5')
  })

  it('matches the chord parsed from the same binding', () => {
    const event = key({ key: 'w', code: 'KeyW', metaKey: true })
    const chord = chordFromEvent(event)
    expect(chord && chordId(chord)).toBe(chordId(parseKeybinding('Mod+W', 'mac')))
  })
})

describe('formatKeybinding', () => {
  it('uses symbols in macOS order', () => {
    expect(formatKeybinding('Mod+Shift+B', 'mac')).toBe('⇧⌘B')
    expect(formatKeybinding('Ctrl+Shift+Tab', 'mac')).toBe('⌃⇧⇥')
  })

  it('spells modifiers out elsewhere', () => {
    expect(formatKeybinding('Mod+Shift+B', 'other')).toBe('Ctrl+Shift+B')
    expect(formatKeybinding('Ctrl+Tab', 'other')).toBe('Ctrl+Tab')
    expect(formatKeybinding('Alt+ArrowUp', 'other')).toBe('Alt+Up')
  })
})
