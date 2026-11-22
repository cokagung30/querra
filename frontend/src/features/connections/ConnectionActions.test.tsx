import { QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import { commands, installKeybindings } from '@/lib/commands'
import { newQueryClient } from '@/lib/query'
import { uiStorage } from '@/lib/storage'
import { resetConnectionsStore, useConnectionsStore } from '@/stores/connections'
import { conn, group } from '@/test/connections'

import {
  arrangeConnections,
  createGroup,
  deleteConnection,
  deleteGroup,
  duplicateConnection,
  listConnections,
  listGroups,
  renameGroup,
} from './api'
import { registerConnectionCommands } from './commands'
import { ConnectionDialogs } from './ConnectionDialogs'
import { ConnectionList } from './ConnectionList'

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  listConnections: vi.fn(),
  listGroups: vi.fn(),
  listEngines: vi.fn().mockResolvedValue([]),
  arrangeConnections: vi.fn(),
  createGroup: vi.fn(),
  deleteConnection: vi.fn(),
  deleteGroup: vi.fn(),
  duplicateConnection: vi.fn(),
  renameGroup: vi.fn(),
}))

const row = (name: string) => screen.getByRole('button', { name: new RegExp(`^${name}`) })
const invalidName = (message: string) =>
  new Error('call failed', { cause: { code: 'invalid_input', message, detail: 'name' } })

describe('connection actions', () => {
  let cleanup: (() => void)[] = []

  async function renderAll() {
    const client = newQueryClient()
    cleanup = [registerConnectionCommands(commands, client), installKeybindings(commands)]
    render(
      <QueryClientProvider client={client}>
        <TooltipProvider>
          <ConnectionList />
          <ConnectionDialogs />
        </TooltipProvider>
      </QueryClientProvider>,
    )
    await screen.findByText('Blog')
  }

  beforeEach(() => {
    resetConnectionsStore()
    uiStorage.setItem('querra.connections.collapsed', '[]')
    vi.clearAllMocks()
    vi.mocked(listGroups).mockResolvedValue([group('a', 0, 'Clients'), group('b', 1, 'Internal')])
    vi.mocked(listConnections).mockResolvedValue([
      conn('shop', 'a', 0, { name: 'Shop' }),
      conn('blog', 'a', 1, { name: 'Blog' }),
      conn('stats', 'b', 0, { name: 'Stats' }),
    ])
    vi.mocked(deleteConnection).mockResolvedValue()
    vi.mocked(deleteGroup).mockResolvedValue()
    vi.mocked(arrangeConnections).mockResolvedValue()
  })

  afterEach(() => cleanup.forEach((fn) => fn()))

  it('deletes the selected connection with Delete only while the list has focus', async () => {
    await renderAll()
    fireEvent.click(row('Blog'), { detail: 1 })
    act(() => row('Blog').blur()) // focus elsewhere, Blog still selected
    fireEvent.keyDown(document.body, { key: 'Delete' })
    expect(screen.queryByRole('alertdialog')).toBeNull()

    act(() => row('Blog').focus())
    fireEvent.keyDown(row('Blog'), { key: 'Delete' })
    const dialog = screen.getByRole('alertdialog', { name: 'Delete “Blog”?' })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(deleteConnection).toHaveBeenCalledWith('blog'))
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
  })

  it('duplicates from the context menu and selects the copy', async () => {
    vi.mocked(duplicateConnection).mockResolvedValue(conn('copy', 'a', 2, { name: 'Blog (copy)' }))
    await renderAll()
    fireEvent.contextMenu(row('Blog'))
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Duplicate Connection' }))
    expect(duplicateConnection).toHaveBeenCalledWith('blog', 'Blog (copy)')
    await waitFor(() => expect(useConnectionsStore.getState().selectedId).toBe('copy'))
  })

  it('moves a connection to another group from the context menu', async () => {
    await renderAll()
    fireEvent.contextMenu(row('Blog'))
    fireEvent.keyDown(await screen.findByRole('menuitem', { name: 'Move to Group' }), {
      key: 'ArrowRight',
    })
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Internal' }))
    await waitFor(() => expect(arrangeConnections).toHaveBeenCalledWith('b', ['stats', 'blog']))
  })

  it('creates a group for a connection and moves it there', async () => {
    vi.mocked(createGroup).mockResolvedValue(group('new', 2, 'Archive'))
    await renderAll()
    act(() => void commands.execute('group.new', { moveId: 'blog' }))
    const dialog = screen.getByRole('dialog', { name: 'New Group' })
    fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'Archive' } })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Create' }))
    await waitFor(() => expect(arrangeConnections).toHaveBeenCalledWith('new', ['blog']))
    expect(createGroup).toHaveBeenCalledWith('Archive')
  })

  it('shows why a group name was refused and keeps the dialog open', async () => {
    vi.mocked(renameGroup).mockRejectedValue(invalidName('a group named "Internal" already exists'))
    await renderAll()
    act(() => void commands.execute('group.rename', 'a'))
    const dialog = screen.getByRole('dialog', { name: 'Rename Group' })
    const name = within(dialog).getByLabelText('Name')
    expect(name).toHaveValue('Clients')
    fireEvent.change(name, { target: { value: 'internal' } })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    expect(
      await within(dialog).findByText('a group named "Internal" already exists'),
    ).toBeInTheDocument()
    expect(renameGroup).toHaveBeenCalledWith('a', 'internal')
  })

  it('confirms before deleting a group and says where its connections go', async () => {
    await renderAll()
    act(() => void commands.execute('group.delete', 'a'))
    const dialog = screen.getByRole('alertdialog', { name: 'Delete group “Clients”?' })
    expect(within(dialog).getByText(/2 connections move to “No group”/)).toBeInTheDocument()
    fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(deleteGroup).toHaveBeenCalledWith('a'))
  })
})
