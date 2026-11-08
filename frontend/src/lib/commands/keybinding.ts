// Keybindings are written like "Mod+Shift+B". "Mod" is Cmd on macOS and Ctrl
// elsewhere; "Ctrl", "Alt" (Option) and "Shift" mean the same key on both.
// They are matched against keydown events as chords.

export type Platform = 'mac' | 'other'

export interface Chord {
  ctrl: boolean
  alt: boolean
  shift: boolean
  meta: boolean
  // Lowercase key name: a letter, digit, symbol, or a named key such as "tab".
  key: string
}

export function detectPlatform(userAgent = globalThis.navigator?.userAgent ?? ''): Platform {
  return /Mac|iPhone|iPad/.test(userAgent) ? 'mac' : 'other'
}

const NAMED_KEYS = new Set([
  'tab',
  'enter',
  'escape',
  'space',
  'backspace',
  'delete',
  'insert',
  'home',
  'end',
  'pageup',
  'pagedown',
  'arrowup',
  'arrowdown',
  'arrowleft',
  'arrowright',
  ...Array.from({ length: 12 }, (_, i) => `f${i + 1}`),
])

const KEY_ALIASES: Record<string, string> = {
  esc: 'escape',
  return: 'enter',
  del: 'delete',
  up: 'arrowup',
  down: 'arrowdown',
  left: 'arrowleft',
  right: 'arrowright',
}

// Physical keys whose character changes with Shift or the keyboard layout.
// Matching them by code keeps "Mod+Shift+]" working although Shift turns
// event.key into "}".
const CODE_KEYS: Record<string, string> = {
  BracketLeft: '[',
  BracketRight: ']',
  Minus: '-',
  Equal: '=',
  Comma: ',',
  Period: '.',
  Slash: '/',
  Backslash: '\\',
  Semicolon: ';',
  Quote: "'",
  Backquote: '`',
}
const SYMBOL_KEYS = new Set(Object.values(CODE_KEYS))

const MODIFIER_KEYS = new Set([
  'Control',
  'Alt',
  'AltGraph',
  'Shift',
  'Meta',
  'OS',
  'CapsLock',
  'Fn',
])

function normalizeKey(name: string): string | null {
  const key = name.toLowerCase()
  const aliased = KEY_ALIASES[key] ?? key
  if (NAMED_KEYS.has(aliased) || /^[a-z0-9]$/.test(aliased) || SYMBOL_KEYS.has(aliased)) {
    return aliased
  }
  return null
}

// parseKeybinding turns "Mod+Shift+B" into a chord for platform. It throws on
// an unknown modifier or key, or a binding without a key.
export function parseKeybinding(binding: string, platform: Platform): Chord {
  const parts = binding.split('+').map((part) => part.trim())
  const keyName = parts.pop()
  const key = keyName ? normalizeKey(keyName) : null
  if (!key) throw new Error(`keybinding "${binding}": unknown key "${keyName ?? ''}"`)

  const chord: Chord = { ctrl: false, alt: false, shift: false, meta: false, key }
  for (const part of parts) {
    switch (part.toLowerCase()) {
      case 'mod':
        if (platform === 'mac') chord.meta = true
        else chord.ctrl = true
        break
      case 'ctrl':
      case 'control':
        chord.ctrl = true
        break
      case 'alt':
      case 'option':
        chord.alt = true
        break
      case 'shift':
        chord.shift = true
        break
      case 'meta':
      case 'cmd':
      case 'command':
        chord.meta = true
        break
      default:
        throw new Error(`keybinding "${binding}": unknown modifier "${part}"`)
    }
  }
  return chord
}

type KeyEventLike = Pick<
  KeyboardEvent,
  'key' | 'code' | 'ctrlKey' | 'altKey' | 'shiftKey' | 'metaKey'
>

// chordFromEvent returns the chord a keydown event represents, or null for a
// bare modifier press or a key no binding can use.
export function chordFromEvent(event: KeyEventLike): Chord | null {
  if (MODIFIER_KEYS.has(event.key)) return null
  const key = eventKey(event)
  if (!key) return null
  return {
    ctrl: event.ctrlKey,
    alt: event.altKey,
    shift: event.shiftKey,
    meta: event.metaKey,
    key,
  }
}

function eventKey({ key, code }: KeyEventLike): string | null {
  // Letters follow the layout (event.key), so AZERTY's A is "a". When the key
  // produces no Latin letter (Option on macOS, Cyrillic layouts), fall back to
  // the physical key.
  const letter = /^Key([A-Z])$/.exec(code)
  if (letter) return /^[a-zA-Z]$/.test(key) ? key.toLowerCase() : letter[1].toLowerCase()

  const digit = /^(?:Digit|Numpad)(\d)$/.exec(code)
  if (digit) return digit[1]

  if (code in CODE_KEYS) return CODE_KEYS[code]
  if (key === ' ') return 'space'
  return normalizeKey(key)
}

// chordId is a canonical string for comparing chords.
export function chordId(chord: Chord): string {
  return [
    chord.ctrl && 'ctrl',
    chord.alt && 'alt',
    chord.shift && 'shift',
    chord.meta && 'meta',
    chord.key,
  ]
    .filter(Boolean)
    .join('+')
}

const MAC_KEY_SYMBOLS: Record<string, string> = {
  tab: '⇥',
  enter: '↩',
  escape: '⎋',
  space: 'Space',
  backspace: '⌫',
  delete: '⌦',
  arrowup: '↑',
  arrowdown: '↓',
  arrowleft: '←',
  arrowright: '→',
  pageup: '⇞',
  pagedown: '⇟',
}

const OTHER_KEY_NAMES: Record<string, string> = {
  escape: 'Esc',
  arrowup: 'Up',
  arrowdown: 'Down',
  arrowleft: 'Left',
  arrowright: 'Right',
  pageup: 'PgUp',
  pagedown: 'PgDn',
  delete: 'Del',
  insert: 'Ins',
}

function keyLabel(key: string, platform: Platform): string {
  const named = platform === 'mac' ? MAC_KEY_SYMBOLS[key] : OTHER_KEY_NAMES[key]
  if (named) return named
  return key.length === 1 ? key.toUpperCase() : key[0].toUpperCase() + key.slice(1)
}

// formatKeybinding renders a binding the way the platform writes shortcuts:
// "⌃⇧⌘B" on macOS, "Ctrl+Shift+B" elsewhere.
export function formatKeybinding(binding: string, platform: Platform): string {
  const chord = parseKeybinding(binding, platform)
  const key = keyLabel(chord.key, platform)
  if (platform === 'mac') {
    return `${chord.ctrl ? '⌃' : ''}${chord.alt ? '⌥' : ''}${chord.shift ? '⇧' : ''}${chord.meta ? '⌘' : ''}${key}`
  }
  return [
    chord.ctrl && 'Ctrl',
    chord.alt && 'Alt',
    chord.shift && 'Shift',
    chord.meta && 'Win',
    key,
  ]
    .filter(Boolean)
    .join('+')
}
