import type { QueryClient } from '@tanstack/react-query'
import i18n from 'i18next'

import { toAppError } from '@/lib/apperr'
import { commands as defaultRegistry, type CommandRegistry } from '@/lib/commands'
import { queryClient } from '@/lib/query'
import { useConnectionsStore } from '@/stores/connections'

import { duplicateConnection, type Connection } from './api'
import { connectionKeys } from './queries'

// registerConnectionCommands adds the connection list's actions. Default
// keybindings avoid what the OS or the macOS menu takes (app/commands.test.ts).
// Commands that act on one connection or group take its id as the argument;
// from a keybinding they act on the selected connection.
export function registerConnectionCommands(
  registry: CommandRegistry = defaultRegistry,
  client: QueryClient = queryClient,
): () => void {
  const store = () => useConnectionsStore.getState()
  const target = (arg: unknown) => (typeof arg === 'string' ? arg : store().selectedId)

  const unregister = [
    registry.register({
      id: 'connection.new',
      titleKey: 'commands.connection.new',
      defaultKeybinding: 'Mod+N',
      run: () => store().openNew(),
    }),
    registry.register({
      id: 'connection.edit',
      titleKey: 'commands.connection.edit',
      run: (arg) => {
        const id = target(arg)
        if (id) store().openEdit(id)
      },
    }),
    registry.register({
      id: 'connection.duplicate',
      titleKey: 'commands.connection.duplicate',
      run: (arg) => {
        const id = target(arg)
        if (id) void duplicate(client, id)
      },
    }),
    registry.register({
      id: 'connection.delete',
      titleKey: 'commands.connection.delete',
      // Fn+Delete on a Mac keyboard; Backspace belongs to the Edit menu there.
      defaultKeybinding: 'Delete',
      // From the keyboard only while the list has focus, so Delete elsewhere
      // never removes a connection.
      isEnabled: (arg) =>
        typeof arg === 'string' || (store().listFocused && store().selectedId !== null),
      run: (arg) => {
        const id = target(arg)
        if (id) store().setConfirm({ kind: 'connection', id })
      },
    }),
    registry.register({
      id: 'group.new',
      titleKey: 'commands.group.new',
      // { moveId } moves that connection into the new group.
      run: (arg) => {
        const moveId = (arg as { moveId?: string } | undefined)?.moveId
        store().setGroupDialog({ mode: 'new', moveId })
      },
    }),
    registry.register({
      id: 'group.rename',
      titleKey: 'commands.group.rename',
      isEnabled: (arg) => typeof arg === 'string',
      run: (arg) => store().setGroupDialog({ mode: 'rename', id: arg as string }),
    }),
    registry.register({
      id: 'group.delete',
      titleKey: 'commands.group.delete',
      isEnabled: (arg) => typeof arg === 'string',
      run: (arg) => store().setConfirm({ kind: 'group', id: arg as string }),
    }),
  ]
  return () => unregister.forEach((fn) => fn())
}

// duplicate copies a connection (and its saved secrets) as "<name> (copy)",
// placed right after it, and selects the copy.
async function duplicate(client: QueryClient, id: string) {
  const source = client
    .getQueryData<Connection[]>(connectionKeys.connections)
    ?.find((c) => c.id === id)
  if (!source) return
  try {
    const copy = await duplicateConnection(
      id,
      i18n.t('connections.copyName', { name: source.settings.name }),
    )
    await client.invalidateQueries({ queryKey: connectionKeys.connections })
    useConnectionsStore.getState().select(copy.id)
  } catch (err) {
    useConnectionsStore.getState().setError(toAppError(err))
  }
}
