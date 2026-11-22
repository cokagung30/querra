import { QueryClientProvider, type QueryClient } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { newQueryClient } from '@/lib/query'
import { resetConnectionsStore, useConnectionsStore } from '@/stores/connections'
import { conn, group } from '@/test/connections'

import { arrangeConnections, arrangeGroups, type Connection, type Group } from './api'
import { connectionKeys } from './queries'
import { useArrange, type DropEvent } from './useArrange'

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  listConnections: vi.fn(() => new Promise(() => {})),
  listGroups: vi.fn(() => new Promise(() => {})),
  arrangeConnections: vi.fn(),
  arrangeGroups: vi.fn(),
}))

const connections = [conn('a1', 'a', 0), conn('a2', 'a', 1), conn('b1', 'b', 0)]
const groups = [group('a', 0), group('b', 1)]

const drop = (
  active: string,
  activeType: string,
  over: string,
  overData: Record<string, unknown>,
): DropEvent => ({
  active: { id: active, data: { current: { type: activeType } } },
  over: { id: over, data: { current: overData } },
})

let client: QueryClient

function setup(filter = '') {
  client = newQueryClient()
  client.setQueryData(connectionKeys.connections, connections)
  client.setQueryData(connectionKeys.groups, groups)
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  const { result } = renderHook(() => useArrange(filter), { wrapper })
  return (event: DropEvent) => result.current(event)
}

const order = () =>
  (client.getQueryData<Connection[]>(connectionKeys.connections) ?? [])
    .map((c) => `${c.settings.groupId}/${c.id}@${c.sortOrder}`)
    .sort()

describe('useArrange', () => {
  beforeEach(() => {
    resetConnectionsStore()
    vi.mocked(arrangeConnections).mockReset().mockResolvedValue()
    vi.mocked(arrangeGroups).mockReset().mockResolvedValue()
  })

  it('arranges the destination group when a connection is dropped on another', async () => {
    const onDrop = setup()
    act(() => onDrop(drop('b1', 'connection', 'a1', { type: 'connection' })))
    // The list shows the new order at once, before the backend answers.
    expect(order()).toEqual(['a/a1@1', 'a/a2@2', 'a/b1@0'])
    await waitFor(() => expect(arrangeConnections).toHaveBeenCalledWith('a', ['b1', 'a1', 'a2']))
  })

  it('appends to a group when dropped on the group itself', async () => {
    const onDrop = setup()
    act(() => onDrop(drop('a1', 'connection', 'section:', { type: 'section', groupId: '' })))
    await waitFor(() => expect(arrangeConnections).toHaveBeenCalledWith('', ['a1']))
  })

  it('reorders groups', async () => {
    const onDrop = setup()
    act(() => onDrop(drop('group:b', 'group', 'group:a', { type: 'group' })))
    await waitFor(() => expect(arrangeGroups).toHaveBeenCalledWith(['b', 'a']))
    expect(client.getQueryData<Group[]>(connectionKeys.groups)?.map((g) => g.sortOrder)).toEqual([
      1, 0,
    ])
  })

  it('does nothing while the list is filtered', async () => {
    const onDrop = setup('shop')
    await act(async () => onDrop(drop('b1', 'connection', 'a1', { type: 'connection' })))
    await act(async () => onDrop(drop('group:b', 'group', 'group:a', { type: 'group' })))
    expect(arrangeConnections).not.toHaveBeenCalled()
    expect(arrangeGroups).not.toHaveBeenCalled()
  })

  it('puts the order back and reports the error when Arrange fails', async () => {
    vi.mocked(arrangeConnections).mockRejectedValue(
      new Error('call failed', { cause: { code: 'not_found', message: 'gone' } }),
    )
    const onDrop = setup()
    const before = order()
    act(() => onDrop(drop('b1', 'connection', 'a1', { type: 'connection' })))
    await waitFor(() => expect(useConnectionsStore.getState().error?.code).toBe('not_found'))
    expect(order()).toEqual(before)
  })
})
