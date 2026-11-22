import { describe, expect, it } from 'vitest'

import { conn, group } from '@/test/connections'

import type { Connection } from './api'
import { dotToken, moveConnection, moveGroup, passwordChange, sections } from './model'

const ids = (cs: Connection[]) => cs.map((c) => c.id)

describe('sections', () => {
  const groups = [group('b', 1), group('a', 0), group('empty', 2)]
  const connections = [
    conn('a2', 'a', 1),
    conn('loose', '', 0),
    conn('a1', 'a', 0, { host: 'DB.Example.com' }),
    conn('b1', 'b', 0, { database: 'shop' }),
  ]

  it('orders groups and their connections, with "No group" last', () => {
    const got = sections(connections, groups, '')
    expect(got.map((s) => s.group?.id ?? null)).toEqual(['a', 'b', 'empty', null])
    expect(got.map((s) => ids(s.items))).toEqual([['a1', 'a2'], ['b1'], [], ['loose']])
  })

  it('keeps an empty "No group" section as a drop target', () => {
    const got = sections([conn('a1', 'a', 0)], groups, '')
    expect(got.at(-1)).toEqual({ group: null, items: [] })
  })

  it('puts connections of a deleted group under "No group"', () => {
    const got = sections([conn('x', 'gone', 0)], [], '')
    expect(got).toEqual([{ group: null, items: [expect.objectContaining({ id: 'x' })] }])
  })

  it('filters on name, host and database ignoring case, and drops empty sections', () => {
    expect(sections(connections, groups, ' example ').map((s) => ids(s.items))).toEqual([['a1']])
    expect(sections(connections, groups, 'SHOP').map((s) => ids(s.items))).toEqual([['b1']])
    expect(sections(connections, groups, 'LOOSE').map((s) => s.group)).toEqual([null])
    expect(sections(connections, groups, 'nothing')).toEqual([])
  })
})

describe('dotToken', () => {
  it('uses the connection color, else the environment color', () => {
    expect(dotToken(conn('x', '', 0, { color: 'teal' }).settings)).toBe('--conn-teal')
    expect(dotToken(conn('x', '', 0, { env: 'production' }).settings)).toBe('--env-production')
  })
})

describe('moveConnection', () => {
  const groups = [group('a', 0), group('b', 1)]
  const all = sections(
    [conn('a1', 'a', 0), conn('a2', 'a', 1), conn('a3', 'a', 2), conn('b1', 'b', 0)],
    groups,
    '',
  )

  it('reorders within a group', () => {
    expect(moveConnection(all, 'a3', 'a1')).toEqual({ groupId: 'a', ids: ['a3', 'a1', 'a2'] })
    expect(moveConnection(all, 'a1', 'a3')).toEqual({ groupId: 'a', ids: ['a2', 'a3', 'a1'] })
  })

  it('moves to another group before the connection it was dropped on', () => {
    expect(moveConnection(all, 'a2', 'b1')).toEqual({ groupId: 'b', ids: ['a2', 'b1'] })
  })

  it('moves to the end of a group, including "No group"', () => {
    expect(moveConnection(all, 'b1', { groupId: 'a' })).toEqual({
      groupId: 'a',
      ids: ['a1', 'a2', 'a3', 'b1'],
    })
    expect(moveConnection(all, 'a1', { groupId: '' })).toEqual({ groupId: '', ids: ['a1'] })
  })

  it('returns null when nothing moves', () => {
    expect(moveConnection(all, 'a1', 'a1')).toBeNull()
    expect(moveConnection(all, 'a3', { groupId: 'a' })).toBeNull()
    expect(moveConnection(all, 'missing', 'a1')).toBeNull()
    expect(moveConnection(all, 'a1', 'missing')).toBeNull()
  })
})

describe('moveGroup', () => {
  const groups = [group('c', 2), group('a', 0), group('b', 1)]

  it('returns the new order of every group', () => {
    expect(moveGroup(groups, 'c', 'a')).toEqual(['c', 'a', 'b'])
    expect(moveGroup(groups, 'a', 'b')).toEqual(['b', 'a', 'c'])
  })

  it('returns null when nothing moves', () => {
    expect(moveGroup(groups, 'a', 'a')).toBeNull()
    expect(moveGroup(groups, 'a', 'missing')).toBeNull()
  })
})

describe('passwordChange', () => {
  it('sets a typed password', () => {
    expect(passwordChange('x', false)).toEqual({ action: 'set', value: 'x' })
  })

  it('keeps the saved password when the field is empty', () => {
    expect(passwordChange('', false)).toEqual({ action: 'keep', value: '' })
  })

  it('clears it when asked and nothing was typed', () => {
    expect(passwordChange('', true)).toEqual({ action: 'clear', value: '' })
    expect(passwordChange('new', true)).toEqual({ action: 'set', value: 'new' })
  })
})
