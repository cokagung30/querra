import { QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import en from '@/i18n/en.json'
import { newQueryClient } from '@/lib/query'
import { resetConnectionsStore, useConnectionsStore } from '@/stores/connections'
import { conn, group } from '@/test/connections'

import {
  createConnection,
  listConnections,
  listEngines,
  listGroups,
  testConnection,
  updateConnection,
  type TestResult,
} from './api'
import { ConnectionDialog } from './ConnectionDialog'

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  listConnections: vi.fn(),
  listGroups: vi.fn(),
  listEngines: vi.fn(),
  createConnection: vi.fn(),
  updateConnection: vi.fn(),
  testConnection: vi.fn(),
  chooseFile: vi.fn(),
}))

// A test call that never finishes, as Wails' CancellablePromise.
function pendingTest() {
  const cancel = vi.fn()
  vi.mocked(testConnection).mockReturnValue(
    Object.assign(new Promise<TestResult>(() => {}), { cancel }) as never,
  )
  return cancel
}

function renderDialog() {
  render(
    <QueryClientProvider client={newQueryClient()}>
      <TooltipProvider>
        <ConnectionDialog />
      </TooltipProvider>
    </QueryClientProvider>,
  )
}

const dialog = () => screen.getByRole('dialog')
const field = (label: string) => within(dialog()).getByLabelText(label)
const type = (label: string, value: string) => fireEvent.change(field(label), { target: { value } })
const click = (name: string | RegExp) =>
  fireEvent.click(within(dialog()).getByRole('button', { name }))

// The dialog is rendered once per test and opened through the store, as the
// commands do.
async function openNew() {
  act(() => useConnectionsStore.getState().openNew())
  // The form appears once the engines have loaded.
  await within(await screen.findByRole('dialog')).findAllByText('PostgreSQL')
}

async function openEdit(id: string) {
  act(() => useConnectionsStore.getState().openEdit(id))
  await waitFor(() => expect(field('Name')).toHaveValue('Shop'))
}

describe('ConnectionDialog', () => {
  beforeEach(() => {
    resetConnectionsStore()
    vi.clearAllMocks()
    vi.mocked(listEngines).mockResolvedValue([
      { engine: 'postgres', defaultPort: 5432, capabilities: {} } as never,
    ])
    vi.mocked(listGroups).mockResolvedValue([group('a', 0, 'Clients')])
    vi.mocked(listConnections).mockResolvedValue([
      conn('shop', 'a', 0, { name: 'Shop', host: 'db.example.com', port: 5433 }, 'saved'),
    ])
  })

  it('creates a connection with the typed password', async () => {
    vi.mocked(createConnection).mockResolvedValue(conn('new', '', 0))
    renderDialog()
    await openNew()
    type('Name', 'Local')
    type('Host', 'localhost')
    type('User', 'app')
    type('Password', 's3cret')
    type('Database', 'app_dev')
    click('Save')
    await waitFor(() => expect(createConnection).toHaveBeenCalled())
    const [settings, password] = vi.mocked(createConnection).mock.calls[0]
    expect(settings).toMatchObject({
      name: 'Local',
      engine: 'postgres',
      env: 'local',
      host: 'localhost',
      port: 0,
      username: 'app',
      database: 'app_dev',
      readOnly: false,
      ssl: { mode: 'prefer' },
    })
    expect(password).toEqual({ action: 'set', value: 's3cret' })
    await waitFor(() => expect(useConnectionsStore.getState().dialog).toBeNull())
    expect(useConnectionsStore.getState().selectedId).toBe('new')
  })

  it('turns read-only on when Production is chosen, and marks the dialog', async () => {
    renderDialog()
    await openNew()
    fireEvent.click(within(dialog()).getByRole('radio', { name: 'Production' }))
    expect(within(dialog()).getByText('PROD')).toBeInTheDocument()
    expect(within(dialog()).getByText(/Read-only is on for Production/)).toBeInTheDocument()
    fireEvent.mouseDown(within(dialog()).getByRole('tab', { name: 'Advanced' }))
    expect(within(dialog()).getByRole('switch', { name: 'Read-only' })).toBeChecked()
  })

  it('keeps a saved password unless it is cleared or replaced', async () => {
    vi.mocked(updateConnection).mockResolvedValue(conn('shop', 'a', 0))
    renderDialog()
    await openEdit('shop')
    const password = field('Password')
    expect(password).toHaveValue('')
    expect(password).toHaveAttribute('placeholder', 'Saved in keychain — leave empty to keep')
    type('Password', 'typo')
    type('Password', '')
    click('Save')
    await waitFor(() => expect(updateConnection).toHaveBeenCalled())
    expect(vi.mocked(updateConnection).mock.calls[0][0]).toBe('shop')
    expect(vi.mocked(updateConnection).mock.calls[0][1]).toMatchObject({ port: 5433 })
    expect(vi.mocked(updateConnection).mock.calls[0][2]).toEqual({ action: 'keep', value: '' })
    await waitFor(() => expect(useConnectionsStore.getState().dialog).toBeNull())

    await openEdit('shop')
    click('Clear password')
    click('Save')
    await waitFor(() => expect(updateConnection).toHaveBeenCalledTimes(2))
    expect(vi.mocked(updateConnection).mock.calls[1][2]).toEqual({ action: 'clear', value: '' })
  })

  it('shows a refused field under it and switches to its tab', async () => {
    vi.mocked(createConnection).mockRejectedValue(
      new Error('call failed', {
        cause: { code: 'invalid_input', message: 'the host is required', detail: 'host' },
      }),
    )
    renderDialog()
    await openNew()
    type('Name', 'Local')
    fireEvent.mouseDown(within(dialog()).getByRole('tab', { name: 'SSL' }))
    click('Save')
    expect(await within(dialog()).findByText('the host is required')).toBeInTheDocument()
    expect(within(dialog()).getByRole('tab', { name: 'General' })).toHaveAttribute(
      'aria-selected',
      'true',
    )
    expect(field('Host')).toHaveAttribute('aria-invalid', 'true')
  })

  it('shows each stage of a failed test and why it failed', async () => {
    vi.mocked(testConnection).mockReturnValue(
      Object.assign(
        Promise.resolve({
          ok: false,
          stages: [
            { stage: 'tcp', status: 'ok' },
            { stage: 'tls', status: 'ok' },
            { stage: 'auth', status: 'failed' },
            { stage: 'ping', status: 'not_run' },
          ],
          error: { code: 'auth_failed', message: 'password authentication failed' },
          tls: true,
          durationMs: 142,
        } as TestResult),
        { cancel: vi.fn() },
      ) as never,
    )
    renderDialog()
    await openEdit('shop')
    click('Test Connection')
    const result = await within(dialog()).findByRole('status')
    expect(within(result).getByText('Login')).toHaveAttribute('data-status', 'failed')
    expect(within(result).getByText('TCP')).toHaveAttribute('data-status', 'ok')
    expect(within(result).getByText(en.errors.auth_failed)).toBeInTheDocument()
    expect(within(result).getByText(en.errorHints.auth_failed)).toBeInTheDocument()
    expect(within(result).getByText('142 ms')).toBeInTheDocument()
    expect(vi.mocked(testConnection).mock.calls[0][2]).toEqual({ action: 'keep', value: '' })
  })

  it('cancels a running test when the dialog closes or saves', async () => {
    const cancel = pendingTest()
    renderDialog()
    await openEdit('shop')
    click('Test Connection')
    click('Cancel test')
    expect(cancel).toHaveBeenCalledTimes(1)

    click('Test Connection')
    vi.mocked(updateConnection).mockResolvedValue(conn('shop', 'a', 0))
    click('Save')
    expect(cancel).toHaveBeenCalledTimes(2)
    await waitFor(() => expect(useConnectionsStore.getState().dialog).toBeNull())

    await openEdit('shop')
    click('Test Connection')
    click('Cancel')
    expect(cancel).toHaveBeenCalledTimes(3)
  })

  it('asks before throwing away changes', async () => {
    renderDialog()
    await openEdit('shop')
    type('Name', 'Shop 2')
    click('Cancel')
    const confirm = screen.getByRole('alertdialog', { name: 'Discard changes?' })
    fireEvent.click(within(confirm).getByRole('button', { name: 'Keep editing' }))
    expect(useConnectionsStore.getState().dialog).not.toBeNull()

    click('Cancel')
    fireEvent.click(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Discard' }),
    )
    expect(useConnectionsStore.getState().dialog).toBeNull()
  })
})
