export { installKeybindings } from './dispatcher'
export {
  chordFromEvent,
  detectPlatform,
  formatKeybinding,
  parseKeybinding,
  type Chord,
  type Platform,
} from './keybinding'
export {
  CommandRegistry,
  commands,
  type Command,
  type KeybindingConflict,
  type KeybindingOverrides,
} from './registry'
export { useKeybindingLabel } from './useKeybindingLabel'
