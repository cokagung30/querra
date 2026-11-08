import { beforeEach, describe, expect, it } from 'vitest'

import { resetTabsStore, useTabsStore } from './tabs'

const state = () => useTabsStore.getState()

describe('tabs store', () => {
  beforeEach(resetTabsStore)

  it('opens numbered query tabs and activates the new one', () => {
    state().openQueryTab()
    const second = state().openQueryTab()
    expect(state().tabs.map((t) => t.seq)).toEqual([1, 2])
    expect(state().activeId).toBe(second)
  })

  it('does not reuse numbers after a tab closes', () => {
    const first = state().openQueryTab()
    state().closeTab(first)
    state().openQueryTab()
    expect(state().tabs.map((t) => t.seq)).toEqual([2])
  })

  it('activates the right neighbour when the active tab closes, else the left one', () => {
    const [a, b, c] = [state().openQueryTab(), state().openQueryTab(), state().openQueryTab()]
    state().activate(b)
    state().closeTab()
    expect(state().activeId).toBe(c)
    state().closeTab(c)
    expect(state().activeId).toBe(a)
    state().closeTab()
    expect(state().activeId).toBeNull()
    expect(state().tabs).toEqual([])
  })

  it('keeps the active tab when another one closes', () => {
    const [a, b] = [state().openQueryTab(), state().openQueryTab()]
    state().closeTab(a)
    expect(state().activeId).toBe(b)
  })

  it('cycles through tabs in both directions, wrapping around', () => {
    const [a, b, c] = [state().openQueryTab(), state().openQueryTab(), state().openQueryTab()]
    state().activateNext()
    expect(state().activeId).toBe(a)
    state().activatePrevious()
    expect(state().activeId).toBe(c)
    state().activatePrevious()
    expect(state().activeId).toBe(b)
  })

  it('ignores unknown ids', () => {
    const a = state().openQueryTab()
    state().activate('nope')
    state().closeTab('nope')
    expect(state().activeId).toBe(a)
    expect(state().tabs).toHaveLength(1)
  })
})
