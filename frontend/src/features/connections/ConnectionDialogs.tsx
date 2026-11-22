import { useQueryClient } from '@tanstack/react-query'
import { lazy, Suspense, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { errorText, toAppError, type AppError } from '@/lib/apperr'
import { useConnectionsStore, type GroupDialog } from '@/stores/connections'

import { arrangeConnections, createGroup, deleteConnection, deleteGroup, renameGroup } from './api'
import { connectionKeys, useConnections, useGroups } from './queries'

// The New/Edit Connection dialog (tabs, selects, …) loads on first use, to keep
// it out of the startup bundle.
const ConnectionDialog = lazy(() =>
  import('./ConnectionDialog').then((m) => ({ default: m.ConnectionDialog })),
)

// ConnectionDialogs hosts the dialogs the connection commands open. It lives
// outside the sidebar so they work while the sidebar is collapsed.
export function ConnectionDialogs() {
  const editing = useConnectionsStore((s) => s.dialog !== null)
  return (
    <>
      {editing && (
        <Suspense fallback={null}>
          <ConnectionDialog />
        </Suspense>
      )}
      <ConfirmDeleteDialog />
      <GroupNameDialog />
    </>
  )
}

function useRefresh() {
  const client = useQueryClient()
  return () =>
    Promise.all([
      client.invalidateQueries({ queryKey: connectionKeys.connections }),
      client.invalidateQueries({ queryKey: connectionKeys.groups }),
    ])
}

function ConfirmDeleteDialog() {
  const { t } = useTranslation()
  const confirm = useConnectionsStore((s) => s.confirm)
  const setConfirm = useConnectionsStore((s) => s.setConfirm)
  const setError = useConnectionsStore((s) => s.setError)
  const connections = useConnections().data ?? []
  const groups = useGroups().data ?? []
  const refresh = useRefresh()

  let title = ''
  let body = ''
  if (confirm?.kind === 'connection') {
    const name = connections.find((c) => c.id === confirm.id)?.settings.name ?? ''
    title = t('connections.confirm.connectionTitle', { name })
    body = t('connections.confirm.connectionBody')
  } else if (confirm?.kind === 'group') {
    const name = groups.find((g) => g.id === confirm.id)?.name ?? ''
    const count = connections.filter((c) => c.settings.groupId === confirm.id).length
    title = t('connections.confirm.groupTitle', { name })
    body = t('connections.confirm.groupBody', { count })
  }

  const remove = async () => {
    if (!confirm) return
    setConfirm(null)
    try {
      if (confirm.kind === 'connection') {
        await deleteConnection(confirm.id)
        const { selectedId, select } = useConnectionsStore.getState()
        if (selectedId === confirm.id) select(null)
      } else {
        await deleteGroup(confirm.id)
      }
      await refresh()
    } catch (err) {
      setError(toAppError(err))
    }
  }

  return (
    <AlertDialog open={confirm !== null} onOpenChange={(open) => !open && setConfirm(null)}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{body}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
          <AlertDialogAction variant="destructive" onClick={() => void remove()}>
            {t('common.delete')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

function GroupNameDialog() {
  const dialog = useConnectionsStore((s) => s.groupDialog)
  const setGroupDialog = useConnectionsStore((s) => s.setGroupDialog)
  return (
    <Dialog open={dialog !== null} onOpenChange={(open) => !open && setGroupDialog(null)}>
      {dialog && <GroupNameForm dialog={dialog} onDone={() => setGroupDialog(null)} />}
    </Dialog>
  )
}

function GroupNameForm({ dialog, onDone }: { dialog: GroupDialog; onDone: () => void }) {
  const { t } = useTranslation()
  const groups = useGroups().data ?? []
  const refresh = useRefresh()
  const [name, setName] = useState(() =>
    dialog.mode === 'rename' ? (groups.find((g) => g.id === dialog.id)?.name ?? '') : '',
  )
  const [error, setError] = useState<AppError | null>(null)
  const [busy, setBusy] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    try {
      if (dialog.mode === 'new') {
        const group = await createGroup(name)
        if (dialog.moveId) await arrangeConnections(group.id, [dialog.moveId])
      } else {
        await renameGroup(dialog.id, name)
      }
      await refresh()
      onDone()
    } catch (err) {
      setError(toAppError(err))
    } finally {
      setBusy(false)
    }
  }

  // invalid_input carries Go's reason, e.g. that the name is taken.
  const message =
    error && (error.code === 'invalid_input' ? error.message : errorText(error).message)
  return (
    <DialogContent className="sm:max-w-sm">
      <form onSubmit={(e) => void submit(e)} className="grid gap-4">
        <DialogHeader>
          <DialogTitle>
            {dialog.mode === 'new'
              ? t('connections.groups.newTitle')
              : t('connections.groups.renameTitle')}
          </DialogTitle>
        </DialogHeader>
        <div className="grid gap-1.5">
          <Label htmlFor="group-name">{t('connections.groups.name')}</Label>
          <Input
            id="group-name"
            value={name}
            autoFocus
            aria-invalid={error !== null}
            aria-describedby={error ? 'group-name-error' : undefined}
            onChange={(e) => setName(e.target.value)}
          />
          {message && (
            <p id="group-name-error" className="text-xs text-destructive">
              {message}
            </p>
          )}
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button type="button" variant="outline">
              {t('common.cancel')}
            </Button>
          </DialogClose>
          <Button type="submit" disabled={busy}>
            {dialog.mode === 'new' ? t('common.create') : t('common.save')}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  )
}
