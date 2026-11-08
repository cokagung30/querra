import type { ParseKeys } from 'i18next'

import { chordId, detectPlatform, parseKeybinding, type Chord, type Platform } from './keybinding'

// Every user action is a command: it has a stable id, a translated title, and
// optionally a default keybinding that the user can remap (PLAN §2.4).
export interface Command {
  // Stable id such as "tab.close"; keybinding overrides refer to it.
  id: string
  titleKey: ParseKeys
  // e.g. "Mod+Shift+B" (see keybinding.ts).
  defaultKeybinding?: string
  // arg is undefined when the command comes from a keybinding.
  run: (arg?: unknown) => void
  // Commands that are not enabled neither run nor swallow their keybinding.
  isEnabled?: () => boolean
}

// Keybinding overrides by command id; null unbinds the command.
export type KeybindingOverrides = Record<string, string | null>

export interface KeybindingConflict {
  chord: string
  commandIds: string[]
}

export class CommandRegistry {
  readonly platform: Platform
  private readonly commands = new Map<string, Command>()
  private overrides: KeybindingOverrides = {}
  private byChord = new Map<string, string[]>()
  private readonly listeners = new Set<() => void>()
  private version = 0

  constructor(platform: Platform = detectPlatform()) {
    this.platform = platform
  }

  // register adds a command and returns a function that removes it.
  register(command: Command): () => void {
    if (this.commands.has(command.id)) {
      throw new Error(`command "${command.id}" is already registered`)
    }
    this.commands.set(command.id, command)
    this.changed()
    return () => {
      if (this.commands.get(command.id) === command) {
        this.commands.delete(command.id)
        this.changed()
      }
    }
  }

  get(id: string): Command | undefined {
    return this.commands.get(id)
  }

  list(): Command[] {
    return [...this.commands.values()]
  }

  // execute runs a command if it exists and is enabled, and reports whether
  // it ran.
  execute(id: string, arg?: unknown): boolean {
    const command = this.commands.get(id)
    if (!command || (command.isEnabled && !command.isEnabled())) return false
    command.run(arg)
    return true
  }

  // keybinding returns the binding in effect for a command: the user's
  // override if there is one, else the default.
  keybinding(id: string): string | undefined {
    if (id in this.overrides) return this.overrides[id] ?? undefined
    return this.commands.get(id)?.defaultKeybinding
  }

  setOverrides(overrides: KeybindingOverrides): void {
    this.overrides = { ...overrides }
    this.changed()
  }

  // commandForChord returns the command bound to chord. When several commands
  // share a chord, the one registered first wins (see conflicts()).
  commandForChord(chord: Chord): Command | undefined {
    const ids = this.byChord.get(chordId(chord))
    return ids ? this.commands.get(ids[0]) : undefined
  }

  conflicts(): KeybindingConflict[] {
    return [...this.byChord]
      .filter(([, ids]) => ids.length > 1)
      .map(([chord, commandIds]) => ({ chord, commandIds }))
  }

  // subscribe and getVersion plug into React's useSyncExternalStore.
  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }

  getVersion = (): number => this.version

  private changed(): void {
    this.byChord = new Map()
    for (const command of this.commands.values()) {
      const binding = this.keybinding(command.id)
      if (!binding) continue
      let chord: string
      try {
        chord = chordId(parseKeybinding(binding, this.platform))
      } catch (err) {
        console.warn(`commands: ignoring keybinding of "${command.id}":`, err)
        continue
      }
      this.byChord.set(chord, [...(this.byChord.get(chord) ?? []), command.id])
    }
    if (import.meta.env.DEV) {
      for (const { chord, commandIds } of this.conflicts()) {
        console.warn(`commands: ${commandIds.join(', ')} share the keybinding ${chord}`)
      }
    }
    this.version++
    for (const listener of this.listeners) listener()
  }
}

// The app-wide registry.
export const commands = new CommandRegistry()
