import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { App } from './App'

vi.mock('./api', () => ({
  getAppInfo: vi.fn().mockResolvedValue({ name: 'Querra', version: '1.2.3' }),
}))

describe('App', () => {
  it('shows the version reported by the backend', async () => {
    render(<App />)
    expect(await screen.findByText('v1.2.3')).toBeInTheDocument()
  })
})
