import { useReducer } from 'react'

import { Env, SSLMode, type EngineInfo, type SSL, type Settings } from './api'

export type Tab = 'general' | 'ssl' | 'ssh' | 'advanced'

// FormState is the connection dialog's working copy. password is what was
// typed (never a saved value); cleared means Clear was pressed.
export interface FormState {
  settings: Settings
  password: string
  cleared: boolean
  tab: Tab
  // Field errors from Go's invalid_input, by its detail (e.g. "host", "ssl.mode").
  errors: Record<string, string>
  dirty: boolean
}

type Action =
  | { type: 'set'; patch: Partial<Settings> }
  | { type: 'ssl'; patch: Partial<SSL> }
  | { type: 'env'; env: Env }
  | { type: 'password'; value: string }
  | { type: 'cleared'; cleared: boolean }
  | { type: 'tab'; tab: Tab }
  | { type: 'error'; field: string; message: string }

// fieldTab is the tab that holds a field named in an invalid_input detail.
export function fieldTab(field: string): Tab {
  if (field.startsWith('ssl.')) return 'ssl'
  if (field === 'readOnly') return 'advanced'
  return 'general'
}

export function newSettings(engine: EngineInfo | undefined): Settings {
  return {
    groupId: '',
    name: '',
    engine: engine?.engine ?? ('' as Settings['engine']),
    env: Env.EnvLocal,
    color: '' as Settings['color'],
    host: '',
    port: 0,
    username: '',
    database: '',
    ssl: { mode: SSLMode.SSLPrefer, caFile: '', certFile: '', keyFile: '' },
    readOnly: false,
  }
}

// Editing a field clears its error.
const without = (errors: Record<string, string>, fields: string[]) =>
  Object.fromEntries(Object.entries(errors).filter(([k]) => !fields.includes(k)))

function reduce(state: FormState, action: Action): FormState {
  switch (action.type) {
    case 'set':
      return {
        ...state,
        settings: { ...state.settings, ...action.patch },
        errors: without(state.errors, Object.keys(action.patch)),
        dirty: true,
      }
    case 'ssl':
      return {
        ...state,
        settings: { ...state.settings, ssl: { ...state.settings.ssl, ...action.patch } },
        errors: without(
          state.errors,
          Object.keys(action.patch).map((k) => `ssl.${k}`),
        ),
        dirty: true,
      }
    case 'env': {
      // Choosing Production turns read-only on (PLAN §3.7); it can be turned
      // off again on the Advanced tab.
      const toProduction =
        action.env === Env.EnvProduction && state.settings.env !== Env.EnvProduction
      return reduce(state, {
        type: 'set',
        patch: { env: action.env, ...(toProduction && { readOnly: true }) },
      })
    }
    case 'password':
      return {
        ...state,
        password: action.value,
        errors: without(state.errors, ['password']),
        dirty: true,
      }
    case 'cleared':
      return { ...state, cleared: action.cleared, dirty: true }
    case 'tab':
      return { ...state, tab: action.tab }
    case 'error':
      return {
        ...state,
        errors: { ...state.errors, [action.field]: action.message },
        tab: fieldTab(action.field),
      }
  }
}

export function useConnectionForm(initial: Settings) {
  return useReducer(reduce, undefined, (): FormState => ({
    settings: initial,
    password: '',
    cleared: false,
    tab: 'general',
    errors: {},
    dirty: false,
  }))
}
