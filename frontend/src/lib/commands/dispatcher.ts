import { chordFromEvent } from './keybinding'
import type { CommandRegistry } from './registry'

// installKeybindings is the app's single keydown listener: it runs the command
// bound to each chord. Features register commands instead of listening for
// keys themselves, so every shortcut stays remappable.
//
// It listens in the bubble phase and skips events a focused widget already
// handled (defaultPrevented), so editors such as CodeMirror keep their own
// keys. In text fields, chords without Ctrl, Alt or Meta are left alone so
// typing, Tab and Enter behave normally.
export function installKeybindings(registry: CommandRegistry, target: Window = window): () => void {
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.defaultPrevented || event.isComposing) return
    // AltGr arrives as Ctrl+Alt on Windows; it types characters such as "@".
    if (event.getModifierState?.('AltGraph')) return

    const chord = chordFromEvent(event)
    if (!chord) return
    if (isTextField(event.target) && !chord.ctrl && !chord.alt && !chord.meta) return

    const command = registry.commandForChord(chord)
    if (command && registry.execute(command.id)) event.preventDefault()
  }

  target.addEventListener('keydown', onKeyDown)
  return () => target.removeEventListener('keydown', onKeyDown)
}

function isTextField(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable) return true
  return (
    target instanceof HTMLInputElement ||
    target instanceof HTMLTextAreaElement ||
    target instanceof HTMLSelectElement
  )
}
