import type { Connection, Group, PasswordChange, Settings } from './api'

// A section of the sidebar list: a group, or null for "No group".
export interface Section {
  group: Group | null
  items: Connection[]
}

const bySortOrder = <T extends { sortOrder: number }>(a: T, b: T) => a.sortOrder - b.sortOrder

// sections groups the connections for the sidebar: groups in their order, then
// "No group", which also takes connections whose group no longer exists. A
// filter matches name, host or database, ignoring case, and drops the
// sections it leaves empty; without one every section stays as a drop target.
export function sections(connections: Connection[], groups: Group[], filter: string): Section[] {
  const query = filter.trim().toLowerCase()
  const matches = (c: Connection) =>
    !query ||
    [c.settings.name, c.settings.host, c.settings.database].some((v) =>
      v.toLowerCase().includes(query),
    )
  const known = new Set(groups.map((g) => g.id))
  const items = (inGroup: (c: Connection) => boolean) =>
    connections.filter((c) => inGroup(c) && matches(c)).sort(bySortOrder)

  const out: Section[] = [...groups].sort(bySortOrder).map((group) => ({
    group,
    items: items((c) => c.settings.groupId === group.id),
  }))
  out.push({ group: null, items: items((c) => !known.has(c.settings.groupId)) })
  return query ? out.filter((s) => s.items.length > 0) : out
}

// dotToken is the CSS variable for a connection's dot: its own color, or its
// environment's.
export function dotToken(settings: Settings): string {
  return settings.color ? `--conn-${settings.color}` : `--env-${settings.env}`
}

// A drop target: the connection dropped on, or the end of a group ("" for
// "No group").
export type MoveTarget = string | { groupId: string }

// moveConnection is the Arrange call for dragging activeId onto target: the
// destination group and its full new order, or null when nothing changes.
export function moveConnection(
  list: Section[],
  activeId: string,
  target: MoveTarget,
): { groupId: string; ids: string[] } | null {
  const sectionOf = (id: string) => list.find((s) => s.items.some((c) => c.id === id))
  const from = sectionOf(activeId)
  const to =
    typeof target === 'string'
      ? sectionOf(target)
      : list.find((s) => (s.group?.id ?? '') === target.groupId)
  if (!from || !to) return null

  const before = to.items.map((c) => c.id)
  const ids = before.filter((id) => id !== activeId)
  if (typeof target === 'string') {
    if (target === activeId) return null
    // Within a group the dragged item takes the target's place; dropped into
    // another group it goes in front of the target.
    const index = before.indexOf(target)
    ids.splice(
      from === to && before.indexOf(activeId) < index ? index : ids.indexOf(target),
      0,
      activeId,
    )
  } else {
    ids.push(activeId)
  }
  if (from === to && ids.every((id, i) => id === before[i])) return null
  return { groupId: to.group?.id ?? '', ids }
}

// moveGroup is the new order of all groups after dragging activeId onto overId,
// or null when nothing changes.
export function moveGroup(groups: Group[], activeId: string, overId: string): string[] | null {
  const ids = [...groups].sort(bySortOrder).map((g) => g.id)
  const from = ids.indexOf(activeId)
  const to = ids.indexOf(overId)
  if (from < 0 || to < 0 || from === to) return null
  ids.splice(to, 0, ...ids.splice(from, 1))
  return ids
}

// passwordChange is what Save sends for the password field: a typed value is
// set; an empty field keeps the saved password, or clears it after Clear.
export function passwordChange(value: string, cleared: boolean): PasswordChange {
  if (value) return { action: 'set', value } as PasswordChange
  return { action: cleared ? 'clear' : 'keep', value: '' } as PasswordChange
}
