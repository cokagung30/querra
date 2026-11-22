import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  pointerWithin,
  useDroppable,
  useSensor,
  useSensors,
  type Announcements,
  type CollisionDetection,
  type UniqueIdentifier,
} from '@dnd-kit/core'
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import {
  Check,
  ChevronRight,
  Copy,
  Database,
  Folder,
  FolderPlus,
  MoreHorizontal,
  Pencil,
  Plus,
  Trash2,
  X,
  type LucideIcon,
} from 'lucide-react'
import { useState, type HTMLAttributes, type ReactElement } from 'react'
import { useTranslation } from 'react-i18next'

import { CommandButton } from '@/app/CommandButton'
import { Button } from '@/components/ui/button'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { errorText } from '@/lib/apperr'
import { commands, useKeybindingLabel } from '@/lib/commands'
import { uiStorage } from '@/lib/storage'
import { cn } from '@/lib/utils'
import { useConnectionsStore } from '@/stores/connections'

import type { Connection, Group } from './api'
import { ConnectionRow } from './ConnectionRow'
import { sections, type Section } from './model'
import { useConnections, useGroups } from './queries'
import { useArrange } from './useArrange'

const COLLAPSED_KEY = 'querra.connections.collapsed'

// useCollapsedGroups remembers which groups are folded ("" is "No group").
function useCollapsedGroups(): [Set<string>, (key: string) => void] {
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return new Set<string>(JSON.parse(uiStorage.getItem(COLLAPSED_KEY) ?? '[]'))
    } catch {
      return new Set<string>()
    }
  })
  const toggle = (key: string) => {
    const next = new Set(collapsed)
    if (!next.delete(key)) next.add(key)
    uiStorage.setItem(COLLAPSED_KEY, JSON.stringify([...next]))
    setCollapsed(next)
  }
  return [collapsed, toggle]
}

// A dragged group only lands on groups; a dragged connection lands on the
// connection under the pointer, else the group it is over (keyboard drags have
// no pointer, so they use the closest center).
const collision: CollisionDetection = (args) => {
  const draggingGroup = args.active.data.current?.type === 'group'
  const droppableContainers = args.droppableContainers.filter(
    (c) => (c.data.current?.type === 'group') === draggingGroup,
  )
  const hits = pointerWithin({ ...args, droppableContainers })
  if (hits.length > 0) {
    return [hits.find((h) => !String(h.id).startsWith('section:')) ?? hits[0]]
  }
  return closestCenter({ ...args, droppableContainers })
}

// ConnectionList is the sidebar's list of saved connections, by group. Groups
// and connections are ordered by drag and drop, also from the keyboard.
export function ConnectionList() {
  const { t } = useTranslation()
  const connections = useConnections()
  const groups = useGroups()
  const [filter, setFilter] = useState('')
  const [collapsed, toggle] = useCollapsedGroups()
  const setListFocused = useConnectionsStore((s) => s.setListFocused)
  const onDragEnd = useArrange(filter)
  const sensors = useSensors(
    // A few pixels of movement before a drag, so clicks and double-clicks stay clicks.
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    // Space picks up and drops; Enter stays "open" on a row.
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
      keyboardCodes: { start: ['Space'], cancel: ['Escape'], end: ['Space'] },
    }),
  )

  const loaded = connections.data && groups.data
  const empty = loaded && connections.data.length === 0 && groups.data.length === 0
  const list = loaded ? sections(connections.data, groups.data, filter) : []
  const dragging = filter.trim() === ''
  const nameOf = useNames(connections.data, groups.data)

  return (
    <div className="flex h-full flex-col">
      <div className="flex h-9 shrink-0 items-center justify-between pr-1.5 pl-3">
        <h2 className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
          {t('sidebar.label')}
        </h2>
        <div className="flex items-center">
          <CommandButton commandId="connection.new" icon={Plus} />
          <MoreMenu />
        </div>
      </div>
      {empty ? (
        <EmptyState />
      ) : (
        <>
          <div className="shrink-0 px-2 pb-2">
            <Input
              type="search"
              aria-label={t('connections.search')}
              placeholder={t('connections.search')}
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="h-7 text-sm"
            />
          </div>
          <div
            className="min-h-0 flex-1 overflow-y-auto px-1.5 pb-2"
            onFocus={() => setListFocused(true)}
            onBlur={(e) => {
              if (!e.currentTarget.contains(e.relatedTarget)) setListFocused(false)
            }}
          >
            <DndContext
              sensors={sensors}
              collisionDetection={collision}
              onDragEnd={onDragEnd}
              accessibility={{
                announcements: announcements(t, nameOf),
                screenReaderInstructions: { draggable: t('connections.drag.instructions') },
              }}
            >
              <SortableContext
                items={list.flatMap((s) => (s.group ? [`group:${s.group.id}`] : []))}
                strategy={verticalListSortingStrategy}
                disabled={!dragging}
              >
                {list.map((section) => {
                  const key = section.group?.id ?? ''
                  return (
                    <SectionView
                      key={key}
                      section={section}
                      open={filter !== '' || !collapsed.has(key)}
                      onToggle={() => toggle(key)}
                      dragging={dragging}
                    />
                  )
                })}
              </SortableContext>
            </DndContext>
            {loaded && filter && list.length === 0 && (
              <p className="px-2 py-4 text-center text-xs text-muted-foreground">
                {t('connections.noMatches', { filter: filter.trim() })}
              </p>
            )}
          </div>
          <ListError />
        </>
      )}
    </div>
  )
}

type NameOf = (id: UniqueIdentifier | undefined) => string

// useNames gives drag announcements a readable name for a sortable id.
function useNames(connections: Connection[] = [], groups: Group[] = []): NameOf {
  const { t } = useTranslation()
  return (id) => {
    const s = String(id ?? '')
    if (s.startsWith('group:')) return groups.find((g) => `group:${g.id}` === s)?.name ?? ''
    if (s === 'section:') return t('connections.noGroup')
    if (s.startsWith('section:')) return groups.find((g) => `section:${g.id}` === s)?.name ?? ''
    return connections.find((c) => c.id === s)?.settings.name ?? ''
  }
}

function announcements(t: ReturnType<typeof useTranslation>['t'], nameOf: NameOf): Announcements {
  return {
    onDragStart: ({ active }) => t('connections.drag.start', { name: nameOf(active.id) }),
    onDragOver: ({ active, over }) =>
      over
        ? t('connections.drag.over', { name: nameOf(active.id), target: nameOf(over.id) })
        : undefined,
    onDragEnd: ({ active }) => t('connections.drag.end', { name: nameOf(active.id) }),
    onDragCancel: ({ active }) => t('connections.drag.cancel', { name: nameOf(active.id) }),
  }
}

function SectionView({
  section,
  open,
  onToggle,
  dragging,
}: {
  section: Section
  open: boolean
  onToggle: () => void
  dragging: boolean
}) {
  const { t } = useTranslation()
  const groupId = section.group?.id ?? ''
  // "No group" is not a real group: it stays last and cannot be dragged.
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: `group:${groupId}`,
    data: { type: 'group' },
    disabled: !section.group || !dragging,
  })
  const { setNodeRef: setDropRef, isOver } = useDroppable({
    id: `section:${groupId}`,
    data: { type: 'section', groupId },
    disabled: !dragging,
  })
  const handle: HTMLAttributes<HTMLButtonElement> = section.group
    ? { ...attributes, ...listeners }
    : {}

  return (
    <section
      ref={section.group ? setNodeRef : undefined}
      style={{
        transform: CSS.Translate.toString(transform),
        transition,
      }}
      className={cn('mt-1', isDragging && 'relative z-10 opacity-60')}
    >
      <GroupMenu group={section.group}>
        <button
          type="button"
          {...handle}
          aria-expanded={open}
          onClick={onToggle}
          className="flex h-6 w-full items-center gap-1 rounded-md px-1 text-xs font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
        >
          <ChevronRight
            aria-hidden
            className={cn('size-3.5 transition-transform', open && 'rotate-90')}
          />
          <span className="flex-1 truncate text-left">
            {section.group?.name ?? t('connections.noGroup')}
          </span>
          <span className="tabular-nums">{section.items.length}</span>
        </button>
      </GroupMenu>
      <ul
        ref={setDropRef}
        className={cn(
          'mt-0.5 flex min-h-1 flex-col gap-px rounded-md',
          isOver && 'bg-sidebar-accent/50',
        )}
      >
        <SortableContext
          items={section.items.map((c) => c.id)}
          strategy={verticalListSortingStrategy}
          disabled={!dragging}
        >
          {open && section.items.map((c) => <SortableRow key={c.id} connection={c} />)}
        </SortableContext>
      </ul>
    </section>
  )
}

function SortableRow({ connection }: { connection: Connection }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: connection.id,
    data: { type: 'connection' },
  })
  return (
    <RowMenu connection={connection}>
      <li
        ref={setNodeRef}
        style={{ transform: CSS.Translate.toString(transform), transition }}
        className={cn(isDragging && 'relative z-10 opacity-60')}
      >
        <ConnectionRow connection={connection} dragHandle={{ ...attributes, ...listeners }} />
      </li>
    </RowMenu>
  )
}

// CommandMenuItem is a context menu item that runs a registered command and
// shows its keybinding.
function CommandMenuItem({
  commandId,
  arg,
  icon: Icon,
  destructive,
}: {
  commandId: string
  arg?: unknown
  icon: LucideIcon
  destructive?: boolean
}) {
  const { t } = useTranslation()
  const keybinding = useKeybindingLabel(commandId)
  const command = commands.get(commandId)
  if (!command) return null
  return (
    <ContextMenuItem
      variant={destructive ? 'destructive' : 'default'}
      onSelect={() => commands.execute(commandId, arg)}
    >
      <Icon />
      {t(command.titleKey)}
      {keybinding && <ContextMenuShortcut>{keybinding}</ContextMenuShortcut>}
    </ContextMenuItem>
  )
}

// RowMenu is a connection's context menu. Opening it selects the connection.
function RowMenu({ connection, children }: { connection: Connection; children: ReactElement }) {
  const { t } = useTranslation()
  const groups = useGroups().data ?? []
  const select = useConnectionsStore((s) => s.select)
  // Choosing a group goes through the same path as dropping on it.
  const arrange = useArrange('')
  const moveTo = (groupId: string) =>
    arrange({
      active: { id: connection.id, data: { current: { type: 'connection' } } },
      over: { id: `section:${groupId}`, data: { current: { type: 'section', groupId } } },
    })
  const current = connection.settings.groupId
  const choices = [
    ...[...groups]
      .sort((a, b) => a.sortOrder - b.sortOrder)
      .map((g) => ({ id: g.id, name: g.name })),
    { id: '', name: t('connections.noGroup') },
  ]

  return (
    <ContextMenu onOpenChange={(open) => open && select(connection.id)}>
      <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
      <ContextMenuContent className="w-56">
        <CommandMenuItem commandId="connection.edit" arg={connection.id} icon={Pencil} />
        <CommandMenuItem commandId="connection.duplicate" arg={connection.id} icon={Copy} />
        <ContextMenuSeparator />
        <ContextMenuSub>
          <ContextMenuSubTrigger>
            <Folder />
            {t('connections.moveTo')}
          </ContextMenuSubTrigger>
          <ContextMenuSubContent className="w-48">
            {choices.map((g) => (
              <ContextMenuItem key={g.id} disabled={g.id === current} onSelect={() => moveTo(g.id)}>
                {g.id === current ? <Check /> : <span className="size-4" />}
                {g.name}
              </ContextMenuItem>
            ))}
            <ContextMenuSeparator />
            <CommandMenuItem
              commandId="group.new"
              arg={{ moveId: connection.id }}
              icon={FolderPlus}
            />
          </ContextMenuSubContent>
        </ContextMenuSub>
        <ContextMenuSeparator />
        <CommandMenuItem
          commandId="connection.delete"
          arg={connection.id}
          icon={Trash2}
          destructive
        />
      </ContextMenuContent>
    </ContextMenu>
  )
}

// GroupMenu is a group header's context menu; "No group" has none.
function GroupMenu({ group, children }: { group: Group | null; children: ReactElement }) {
  if (!group) return children
  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
      <ContextMenuContent className="w-48">
        <CommandMenuItem commandId="group.rename" arg={group.id} icon={Pencil} />
        <CommandMenuItem commandId="group.delete" arg={group.id} icon={Trash2} destructive />
      </ContextMenuContent>
    </ContextMenu>
  )
}

// MoreMenu is the sidebar header's menu for list-wide actions.
function MoreMenu() {
  const { t } = useTranslation()
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label={t('connections.more')}
          className="text-muted-foreground"
        >
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuItem onSelect={() => commands.execute('group.new')}>
          <FolderPlus />
          {t('commands.group.new')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

// ListError shows why the last list action failed, until dismissed.
function ListError() {
  const { t } = useTranslation()
  const error = useConnectionsStore((s) => s.error)
  const setError = useConnectionsStore((s) => s.setError)
  if (!error) return null
  const text = errorText(error)
  return (
    <div
      role="alert"
      className="mx-2 mb-2 flex shrink-0 items-start gap-2 rounded-md border border-destructive/40 bg-destructive/10 px-2 py-1.5 text-xs"
    >
      <div className="min-w-0 flex-1">
        <p className="text-destructive">{text.message}</p>
        {text.hint && <p className="text-muted-foreground">{text.hint}</p>}
      </div>
      <Button
        variant="ghost"
        size="icon-xs"
        aria-label={t('connections.dismiss')}
        onClick={() => setError(null)}
      >
        <X />
      </Button>
    </div>
  )
}

function EmptyState() {
  const { t } = useTranslation()
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-1.5 px-4 text-center">
      <Database className="mb-1 size-6 text-muted-foreground/60" aria-hidden />
      <p className="text-sm font-medium">{t('sidebar.empty')}</p>
      <p className="text-xs text-muted-foreground">{t('sidebar.emptyHint')}</p>
      <Button
        size="sm"
        variant="outline"
        className="mt-2"
        onClick={() => commands.execute('connection.new')}
      >
        {t('commands.connection.new')}
      </Button>
    </div>
  )
}
