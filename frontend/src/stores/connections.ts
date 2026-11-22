import { create } from 'zustand'

import type { AppError } from '@/lib/apperr'

// The connection dialog that is open, if any.
export type ConnectionDialog = { mode: 'new' } | { mode: 'edit'; id: string }

// A deletion waiting for the user to confirm it.
export type ConfirmDelete = { kind: 'connection' | 'group'; id: string }

// The group name dialog: a new group (and the connection to move into it), or
// renaming one.
export type GroupDialog = { mode: 'new'; moveId?: string } | { mode: 'rename'; id: string }

// UI state for the connection list: the selected connection, whether the list
// has focus (keybindings such as Delete only act then), the open dialogs, and
// the error of the last list action (drag and drop, delete, …), if it failed.
// The connections themselves come from TanStack Query, not from here.
interface ConnectionsState {
  selectedId: string | null
  listFocused: boolean
  dialog: ConnectionDialog | null
  confirm: ConfirmDelete | null
  groupDialog: GroupDialog | null
  error: AppError | null
  select: (id: string | null) => void
  setListFocused: (focused: boolean) => void
  openNew: () => void
  openEdit: (id: string) => void
  closeDialog: () => void
  setConfirm: (confirm: ConfirmDelete | null) => void
  setGroupDialog: (dialog: GroupDialog | null) => void
  setError: (error: AppError | null) => void
}

const initial = {
  selectedId: null,
  listFocused: false,
  dialog: null,
  confirm: null,
  groupDialog: null,
  error: null,
}

export const useConnectionsStore = create<ConnectionsState>()((set) => ({
  ...initial,
  select: (selectedId) => set({ selectedId }),
  setListFocused: (listFocused) => set({ listFocused }),
  openNew: () => set({ dialog: { mode: 'new' } }),
  openEdit: (id) => set({ selectedId: id, dialog: { mode: 'edit', id } }),
  closeDialog: () => set({ dialog: null }),
  setConfirm: (confirm) => set({ confirm }),
  setGroupDialog: (groupDialog) => set({ groupDialog }),
  setError: (error) => set({ error }),
}))

export function resetConnectionsStore() {
  useConnectionsStore.setState(initial)
}
