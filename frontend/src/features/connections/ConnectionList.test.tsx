import { QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import { commands } from '@/lib/commands'
import { newQueryClient } from '@/lib/query'
import { uiStorage } from '@/lib/storage'
import { resetConnectionsStore, useConnectionsStore } from '@/stores/connections'
import { conn, group } from '@/test/connections'

import { listConnections, listGroups } from './api'
import { registerConnectionCommands } from './commands'
import { ConnectionList } from './ConnectionList'

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  listConnections: vi.fn(),
  listGroups: vi.fn(),
  listEngines: vi.fn().mockResolvedValue([]),
}))

const COLLAPSED_KEY = 'querra.connections.collapsed'

function renderList() {
  return render(
    <QueryClientProvider client={newQueryClient()}>
      <TooltipProvider>
        <ConnectionList />
      </TooltipProvider>
    </QueryClientProvider>,
  )
}

// jsdom joins the row's parts without spaces ("ShopPROD…"), so match the start.
const row = (name: string) => screen.getByRole('button', { name: new RegExp(`^${name}`) })

describe('ConnectionList', () => {
  let unregister: () => void

  beforeEach(() => {
    resetConnectionsStore()
    uiStorage.setItem(COLLAPSED_KEY, '[]')
    unregister = registerConnectionCommands(commands)
    vi.mocked(listGroups).mockResolvedValue([group('b', 1, 'Internal'), group('a', 0, 'Clients')])
    vi.mocked(listConnections).mockResolvedValue([
      conn('shop', 'a', 0, { name: 'Shop', env: 'production', color: 'teal', readOnly: true }),
      conn('blog', 'a', 1, { name: 'Blog', host: 'blog.example.com' }),
      conn('stats', 'b', 0, { name: 'Stats', env: 'staging' }, 'required'),
      conn('scratch', '', 0, { name: 'Scratch' }),
    ])
  })

  afterEach(() => unregister())

  it('lists groups in order with "No group" last', async () => {
    renderList()
    await screen.findByText('Shop')
    const headers = screen.getAllByRole('button', { expanded: true }).map((b) => b.textContent)
    expect(headers).toEqual(['Clients2', 'Internal1', 'No group1'])
  })

  it('marks a Production connection even when it has its own color', async () => {
    renderList()
    await screen.findByText('Shop')
    const shop = row('Shop')
    expect(within(shop).getByText('PROD')).toBeInTheDocument()
    expect(shop).toHaveAttribute('data-env', 'production')
    expect(within(shop).getByLabelText('Read-only')).toBeInTheDocument()
    expect(within(row('Blog')).queryByText('PROD')).toBeNull()
  })

  it('shows the environment as text and flags a missing password', async () => {
    renderList()
    await screen.findByText('Stats')
    expect(within(row('Stats')).getByText('staging')).toBeInTheDocument()
    expect(within(row('Stats')).getByLabelText('Password required')).toBeInTheDocument()
    expect(within(row('Blog')).getByText('blog.example.com/blog')).toBeInTheDocument()
  })

  it('filters by name, host or database', async () => {
    renderList()
    await screen.findByText('Shop')
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search connections' }), {
      target: { value: 'BLOG.example' },
    })
    expect(screen.getByText('Blog')).toBeInTheDocument()
    expect(screen.queryByText('Shop')).toBeNull()
    expect(screen.queryByText('Internal')).toBeNull()
  })

  it('remembers collapsed groups', async () => {
    const { unmount } = renderList()
    await screen.findByText('Shop')
    fireEvent.click(screen.getByRole('button', { name: /^Clients/ }))
    expect(screen.queryByText('Shop')).toBeNull()
    unmount()

    renderList()
    await screen.findByText('Stats')
    expect(screen.getByRole('button', { name: /^Clients/ })).toHaveAttribute(
      'aria-expanded',
      'false',
    )
    expect(screen.queryByText('Shop')).toBeNull()
  })

  it('selects on click and edits on Enter or double-click', async () => {
    renderList()
    await screen.findByText('Shop')
    fireEvent.click(row('Blog'), { detail: 1 })
    expect(useConnectionsStore.getState()).toMatchObject({ selectedId: 'blog', dialog: null })

    fireEvent.click(row('Blog'), { detail: 0 }) // Enter on a focused button
    expect(useConnectionsStore.getState().dialog).toEqual({ mode: 'edit', id: 'blog' })

    useConnectionsStore.getState().closeDialog()
    fireEvent.doubleClick(row('Shop'))
    expect(useConnectionsStore.getState().dialog).toEqual({ mode: 'edit', id: 'shop' })
  })

  // WKWebView does not focus a button on click, and Delete acts on the
  // selection only while the list has focus, so a click must focus the row and
  // keyboard focus must move the selection.
  it('focuses the row it selects, and selects the row that gets focus', async () => {
    renderList()
    await screen.findByText('Shop')
    fireEvent.click(row('Blog'), { detail: 1 })
    expect(document.activeElement).toBe(row('Blog'))
    expect(useConnectionsStore.getState().listFocused).toBe(true)

    act(() => row('Stats').focus())
    expect(useConnectionsStore.getState().selectedId).toBe('stats')
  })

  it('offers New Connection when there are none yet', async () => {
    vi.mocked(listConnections).mockResolvedValue([])
    vi.mocked(listGroups).mockResolvedValue([])
    renderList()
    expect(await screen.findByText('No connections yet')).toBeInTheDocument()
    fireEvent.click(screen.getAllByRole('button', { name: 'New Connection' })[0])
    expect(useConnectionsStore.getState().dialog).toEqual({ mode: 'new' })
  })
})
