import { useMutation, useQueryClient } from '@tanstack/react-query'

import { toAppError } from '@/lib/apperr'
import { useConnectionsStore } from '@/stores/connections'

import { arrangeConnections, arrangeGroups, type Connection, type Group } from './api'
import { moveConnection, moveGroup, sections, type MoveTarget } from './model'
import { connectionKeys } from './queries'

// The parts of dnd-kit's DragEndEvent a drop needs. Draggables and droppables
// carry data.type: "connection", "group" (sortable id "group:<id>") or
// "section" (the end of a group, with data.groupId; "" is "No group").
interface DropNode {
  id: string | number
  data: { current?: Record<string, unknown> }
}
export interface DropEvent {
  active: DropNode
  over: DropNode | null
}

const groupId = (sortableId: string | number) => String(sortableId).replace(/^group:/, '')

// useArrange returns the drag-end handler for the sidebar. It shows the new
// order at once, saves it with Arrange or ArrangeGroups, and puts the old
// order back (reporting the error) if saving fails. While the list is
// filtered it does nothing, because the visible order is only part of a group.
export function useArrange(filter: string): (event: DropEvent) => void {
  const client = useQueryClient()
  const setError = useConnectionsStore((s) => s.setError)
  const save = useMutation({
    mutationFn: (op: { groupId: string; ids: string[] } | { groups: string[] }) =>
      'groups' in op ? arrangeGroups(op.groups) : arrangeConnections(op.groupId, op.ids),
  })

  // optimistic applies update to the cached key and saves op, rolling back on
  // failure; the refetch afterwards brings the backend's order either way.
  function optimistic<T>(
    key: readonly string[],
    update: (old: T) => T,
    op: Parameters<typeof save.mutate>[0],
  ) {
    const previous = client.getQueryData<T>(key)
    if (previous === undefined) return
    void client.cancelQueries({ queryKey: key })
    client.setQueryData<T>(key, update(previous))
    save.mutate(op, {
      onError: (err) => {
        client.setQueryData<T>(key, previous)
        setError(toAppError(err))
      },
      onSettled: () => client.invalidateQueries({ queryKey: key }),
    })
  }

  return ({ active, over }) => {
    if (filter.trim() || !over) return
    const connections = client.getQueryData<Connection[]>(connectionKeys.connections) ?? []
    const groups = client.getQueryData<Group[]>(connectionKeys.groups) ?? []

    if (active.data.current?.type === 'group') {
      if (over.data.current?.type !== 'group') return
      const ids = moveGroup(groups, groupId(active.id), groupId(over.id))
      if (!ids) return
      optimistic<Group[]>(
        connectionKeys.groups,
        (old) => old.map((g) => ({ ...g, sortOrder: ids.indexOf(g.id) })),
        { groups: ids },
      )
      return
    }

    const overData = over.data.current ?? {}
    const target: MoveTarget =
      overData.type === 'section'
        ? { groupId: String(overData.groupId ?? '') }
        : overData.type === 'group'
          ? { groupId: groupId(over.id) }
          : String(over.id)
    const move = moveConnection(sections(connections, groups, ''), String(active.id), target)
    if (!move) return
    optimistic<Connection[]>(
      connectionKeys.connections,
      (old) =>
        old.map((c) => {
          const i = move.ids.indexOf(c.id)
          return i < 0
            ? c
            : { ...c, sortOrder: i, settings: { ...c.settings, groupId: move.groupId } }
        }),
      move,
    )
  }
}
