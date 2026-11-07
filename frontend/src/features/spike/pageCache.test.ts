import { describe, expect, it, vi } from 'vitest'

import { RowPageCache, type Row } from './pageCache'

function fakeFetch() {
  return vi.fn(async (offset: number, limit: number): Promise<Row[]> =>
    Array.from({ length: limit }, (_, i) => [String(offset + i)]),
  )
}

describe('RowPageCache', () => {
  it('fetches each page covering the range once', async () => {
    const fetch = fakeFetch()
    const cache = new RowPageCache(fetch, 10)

    cache.ensureRange(5, 25)
    cache.ensureRange(5, 25)
    await cache.waitForRow(25)

    expect(fetch.mock.calls).toEqual([
      [0, 10],
      [10, 10],
      [20, 10],
    ])
    expect(cache.row(25)).toEqual(['25'])
  })

  it('evicts the least recently used page', async () => {
    const cache = new RowPageCache(fakeFetch(), 10, 2)

    await cache.waitForRow(0)
    await cache.waitForRow(10)
    cache.ensureRange(0, 0) // touch page 0 so page 1 becomes the oldest
    await cache.waitForRow(20)

    expect(cache.row(0)).toBeDefined()
    expect(cache.row(10)).toBeUndefined()
    expect(cache.row(20)).toBeDefined()
  })

  it('notifies subscribers when a page arrives', async () => {
    const cache = new RowPageCache(fakeFetch(), 10)
    const listener = vi.fn()
    cache.subscribe(listener)

    await cache.waitForRow(0)

    expect(listener).toHaveBeenCalled()
    expect(cache.getVersion()).toBe(1)
  })
})
