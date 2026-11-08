// uiStorage persists small, non-sensitive UI state (panel sizes) in the
// webview's localStorage. When that is unavailable or throws, it falls back to
// memory for the session. Settings that matter, such as keybindings, belong in
// the Go settings store (PLAN §3.6) instead.
const memory = new Map<string, string>()

function local(): Storage | undefined {
  try {
    return globalThis.localStorage ?? undefined
  } catch {
    return undefined
  }
}

export const uiStorage: Pick<Storage, 'getItem' | 'setItem'> = {
  getItem(key) {
    try {
      const store = local()
      if (store) return store.getItem(key)
    } catch {
      // fall back to memory
    }
    return memory.get(key) ?? null
  },
  setItem(key, value) {
    try {
      const store = local()
      if (store) {
        store.setItem(key, value)
        return
      }
    } catch {
      // fall back to memory
    }
    memory.set(key, value)
  },
}
