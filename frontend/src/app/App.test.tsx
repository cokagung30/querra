import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { commands, installKeybindings } from '@/lib/commands'
import { useLayoutStore } from '@/stores/layout'
import { resetTabsStore } from '@/stores/tabs'

import { App } from './App'
import { registerShellCommands } from './commands'

vi.mock('./api', () => ({
  getAppInfo: vi.fn().mockResolvedValue({ name: 'Querra', version: '1.2.3' }),
}))
vi.mock('@/features/perf/api', () => ({
  ready: vi.fn().mockResolvedValue(''),
}))

// The shell commands use the app-wide registry, as in main.tsx.
const mod = commands.platform === 'mac' ? { metaKey: true } : { ctrlKey: true }

function press(init: KeyboardEventInit) {
  act(() => {
    fireEvent.keyDown(document.body, init)
  })
}

describe('App shell', () => {
  let cleanup: (() => void)[] = []

  beforeEach(() => {
    resetTabsStore()
    useLayoutStore.setState({ sidebarCollapsed: false, resultsCollapsed: false })
    cleanup = [registerShellCommands(commands), installKeybindings(commands)]
  })

  afterEach(() => {
    cleanup.forEach((fn) => fn())
  })

  it('shows the layout regions and the version reported by the backend', async () => {
    render(<App />)
    expect(screen.getByRole('navigation', { name: 'Connections' })).toBeInTheDocument()
    expect(screen.getByRole('tablist', { name: 'Open tabs' })).toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'Results' })).toBeInTheDocument()
    expect(screen.getByRole('contentinfo', { name: 'Status bar' })).toBeInTheDocument()
    expect(await screen.findByText('v1.2.3')).toBeInTheDocument()
  })

  it('lists shortcuts on the welcome screen', () => {
    render(<App />)
    const shortcuts = screen.getByRole('region', { name: 'Keyboard shortcuts' })
    expect(within(shortcuts).getByText('New Query Tab')).toBeInTheDocument()
  })

  it('opens, switches and closes tabs from the keyboard', () => {
    render(<App />)
    press({ key: 't', code: 'KeyT', ...mod })
    press({ key: 't', code: 'KeyT', ...mod })
    expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['Query 1', 'Query 2'])
    expect(screen.getByRole('tab', { name: 'Query 2' })).toHaveAttribute('aria-selected', 'true')

    press({ key: 'Tab', code: 'Tab', ctrlKey: true })
    expect(screen.getByRole('tab', { name: 'Query 1' })).toHaveAttribute('aria-selected', 'true')

    press({ key: 'w', code: 'KeyW', ...mod })
    expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['Query 2'])
  })

  it('closes a tab with its close button', () => {
    render(<App />)
    fireEvent.click(screen.getByRole('button', { name: 'New Query Tab' }))
    fireEvent.click(screen.getByRole('button', { name: 'Close Query 1' }))
    expect(screen.queryAllByRole('tab')).toEqual([])
    expect(screen.getByText('Querra')).toBeInTheDocument() // back to the welcome screen
  })

  it('toggles the sidebar from the keyboard and the status bar', () => {
    render(<App />)
    const toggle = screen.getByRole('button', { name: 'Toggle Sidebar' })
    expect(toggle).toHaveAttribute('aria-pressed', 'true')

    press({ key: 'b', code: 'KeyB', ...mod })
    expect(useLayoutStore.getState().sidebarCollapsed).toBe(true)
    expect(toggle).toHaveAttribute('aria-pressed', 'false')

    fireEvent.click(toggle)
    expect(useLayoutStore.getState().sidebarCollapsed).toBe(false)
  })
})
