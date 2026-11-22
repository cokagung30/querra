import { useQueryClient } from '@tanstack/react-query'
import type { CancellablePromise } from '@wailsio/runtime'
import { Lock, Plug } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
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
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { errorText, toAppError, type AppError } from '@/lib/apperr'
import { cn } from '@/lib/utils'
import { useConnectionsStore } from '@/stores/connections'

import {
  chooseFile,
  Color,
  createConnection,
  Env,
  PasswordState,
  SSLMode,
  testConnection,
  updateConnection,
  type Connection,
  type EngineInfo,
  type Group,
  type SSL,
  type TestResult,
} from './api'
import { dotToken, passwordChange } from './model'
import { connectionKeys, useConnections, useEngines, useGroups } from './queries'
import { TestResultPanel, type TestState } from './TestResultPanel'
import { newSettings, useConnectionForm, type Tab } from './useConnectionForm'

const NO_GROUP = '__none__' // Radix Select reserves "" for "no value".
const ENVS = [Env.EnvLocal, Env.EnvStaging, Env.EnvProduction] as const
const COLORS = [
  Color.ColorRed,
  Color.ColorOrange,
  Color.ColorYellow,
  Color.ColorGreen,
  Color.ColorTeal,
  Color.ColorBlue,
  Color.ColorPurple,
  Color.ColorGray,
] as const
const SSL_MODES = [
  SSLMode.SSLDisable,
  SSLMode.SSLPrefer,
  SSLMode.SSLRequire,
  SSLMode.SSLVerifyCA,
  SSLMode.SSLVerifyFull,
] as const

// ConnectionDialog is the New/Edit Connection dialog the store opens. It waits
// for the engines (and, when editing, the connection) before showing the form.
export function ConnectionDialog() {
  const dialog = useConnectionsStore((s) => s.dialog)
  const engines = useEngines().data
  const connections = useConnections().data
  const groups = useGroups().data ?? []
  if (!dialog || !engines) return null
  const editing = dialog.mode === 'edit' ? connections?.find((c) => c.id === dialog.id) : undefined
  if (dialog.mode === 'edit' && !editing) return null
  return (
    <ConnectionForm
      key={editing?.id ?? 'new'}
      editing={editing}
      engines={engines}
      groups={groups}
    />
  )
}

function ConnectionForm({
  editing,
  engines,
  groups,
}: {
  editing: Connection | undefined
  engines: EngineInfo[]
  groups: Group[]
}) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const closeDialog = useConnectionsStore((s) => s.closeDialog)
  const select = useConnectionsStore((s) => s.select)
  const [form, dispatch] = useConnectionForm(editing?.settings ?? newSettings(engines[0]))
  const [test, setTest] = useState<TestState>({ status: 'idle' })
  const [formError, setFormError] = useState<AppError | null>(null)
  const [saving, setSaving] = useState(false)
  const [discarding, setDiscarding] = useState(false)
  const running = useRef<CancellablePromise<TestResult> | null>(null)

  const s = form.settings
  const production = s.env === Env.EnvProduction
  const engine = engines.find((e) => e.engine === s.engine)
  const password = passwordChange(form.password, form.cleared)

  const stopTest = () => {
    running.current?.cancel()
    running.current = null
  }
  // Closing the dialog cancels a test that is still running.
  useEffect(() => stopTest, [])

  // fail shows a refused field next to it, anything else above the buttons.
  const fail = (err: unknown) => {
    const e = toAppError(err)
    if (e.code === 'invalid_input' && e.detail) {
      dispatch({ type: 'error', field: e.detail, message: e.message })
      return null
    }
    return e
  }

  const runTest = async () => {
    stopTest()
    setFormError(null)
    setTest({ status: 'running' })
    const call = testConnection(editing?.id ?? '', s, password)
    running.current = call
    try {
      const result = await call
      if (running.current === call) setTest({ status: 'done', result })
    } catch (err) {
      if (running.current !== call) return // cancelled
      const e = fail(err)
      setTest(e ? { status: 'error', error: e } : { status: 'idle' })
    } finally {
      if (running.current === call) running.current = null
    }
  }

  const cancelTest = () => {
    stopTest()
    setTest({ status: 'idle' })
  }

  const save = async () => {
    stopTest()
    setTest((prev) => (prev.status === 'running' ? { status: 'idle' } : prev))
    setFormError(null)
    setSaving(true)
    try {
      const saved = editing
        ? await updateConnection(editing.id, s, password)
        : await createConnection(s, password)
      await client.invalidateQueries({ queryKey: connectionKeys.connections })
      select(saved.id)
      closeDialog()
    } catch (err) {
      setFormError(fail(err))
    } finally {
      setSaving(false)
    }
  }

  const requestClose = () => {
    if (form.dirty) setDiscarding(true)
    else close()
  }
  const close = () => {
    stopTest()
    closeDialog()
  }

  const passwordPlaceholder = form.cleared
    ? t('connections.form.passwordCleared')
    : editing?.passwordState === PasswordState.PasswordSaved
      ? t('connections.form.passwordSaved')
      : editing?.passwordState === PasswordState.PasswordRequired
        ? t('connections.form.passwordRequired')
        : ''
  const err = (field: string) => form.errors[field]
  const invalid = (id: string, field: string) => ({
    id,
    'aria-invalid': err(field) ? true : undefined,
    'aria-describedby': err(field) ? `${id}-error` : undefined,
  })

  return (
    <>
      <Dialog open onOpenChange={(open) => !open && requestClose()}>
        <DialogContent
          className={cn(
            'gap-0 p-0 sm:max-w-[640px]',
            production && 'border-t-[3px] border-t-env-production',
          )}
        >
          <form
            onSubmit={(e) => {
              e.preventDefault()
              void save()
            }}
          >
            <DialogHeader className="px-5 pt-5 pb-3">
              <DialogTitle className="flex items-center gap-2">
                {editing ? t('connections.form.editTitle') : t('connections.form.newTitle')}
                {production && <ProdBadge />}
              </DialogTitle>
              <DialogDescription className="sr-only">
                {t('connections.form.description')}
              </DialogDescription>
            </DialogHeader>

            <Tabs
              value={form.tab}
              onValueChange={(tab) => dispatch({ type: 'tab', tab: tab as Tab })}
            >
              <TabsList variant="line" className="w-full justify-start border-b px-4">
                <TabsTrigger value="general">{t('connections.form.tabs.general')}</TabsTrigger>
                <TabsTrigger value="ssl">{t('connections.form.tabs.ssl')}</TabsTrigger>
                <TabsTrigger value="ssh" disabled>
                  {t('connections.form.tabs.ssh')}
                  <span className="ml-1 rounded-sm bg-muted px-1 text-[10px] text-muted-foreground uppercase">
                    {t('connections.form.soon')}
                  </span>
                </TabsTrigger>
                <TabsTrigger value="advanced">{t('connections.form.tabs.advanced')}</TabsTrigger>
              </TabsList>

              <TabsContent value="general" className="grid gap-3 px-5 py-4">
                <Field id="conn-engine" label={t('connections.form.engine')} error={err('engine')}>
                  <Select
                    value={s.engine}
                    onValueChange={(v) =>
                      dispatch({ type: 'set', patch: { engine: v as typeof s.engine } })
                    }
                  >
                    <SelectTrigger {...invalid('conn-engine', 'engine')} className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {engines.map((e) => (
                        <SelectItem key={e.engine} value={e.engine}>
                          {t(`connections.engines.${e.engine}` as 'connections.engines.postgres')}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
                <Field id="conn-name" label={t('connections.form.name')} error={err('name')}>
                  <Input
                    {...invalid('conn-name', 'name')}
                    value={s.name}
                    autoFocus
                    onChange={(e) => dispatch({ type: 'set', patch: { name: e.target.value } })}
                  />
                </Field>
                <Field label={t('connections.form.env')} error={err('env')}>
                  <div
                    role="radiogroup"
                    aria-label={t('connections.form.env')}
                    className="inline-flex w-fit rounded-md border p-0.5"
                  >
                    {ENVS.map((env) => (
                      <label
                        key={env}
                        className={cn(
                          'flex cursor-default items-center gap-1.5 rounded-sm px-2.5 py-1 text-sm has-focus-visible:ring-2 has-focus-visible:ring-ring',
                          s.env === env ? 'bg-accent font-medium' : 'text-muted-foreground',
                        )}
                      >
                        <input
                          type="radio"
                          name="conn-env"
                          className="sr-only"
                          checked={s.env === env}
                          onChange={() => dispatch({ type: 'env', env })}
                        />
                        <span
                          aria-hidden
                          className="size-2 rounded-full"
                          style={{ background: `var(--env-${env})` }}
                        />
                        {t(`connections.form.envs.${env}` as 'connections.form.envs.local')}
                      </label>
                    ))}
                  </div>
                  {production && s.readOnly && (
                    <p className="flex items-center gap-1 text-xs text-muted-foreground">
                      <Lock aria-hidden className="size-3" />
                      {t('connections.form.prodReadOnly')}
                    </p>
                  )}
                </Field>
                <Field label={t('connections.form.color')} error={err('color')}>
                  <div
                    role="radiogroup"
                    aria-label={t('connections.form.color')}
                    className="flex items-center gap-1.5"
                  >
                    {(['', ...COLORS] as const).map((color) => (
                      <label
                        key={color || 'auto'}
                        title={t(
                          `connections.colors.${color || 'auto'}` as 'connections.colors.auto',
                        )}
                        className={cn(
                          'flex size-5 items-center justify-center rounded-full has-focus-visible:ring-2 has-focus-visible:ring-ring',
                          s.color === color &&
                            'ring-2 ring-foreground ring-offset-2 ring-offset-background',
                        )}
                        style={{
                          background: color ? `var(--conn-${color})` : undefined,
                          border: color ? undefined : '1px dashed var(--muted-foreground)',
                        }}
                      >
                        <input
                          type="radio"
                          name="conn-color"
                          className="sr-only"
                          aria-label={t(
                            `connections.colors.${color || 'auto'}` as 'connections.colors.auto',
                          )}
                          checked={s.color === color}
                          onChange={() =>
                            dispatch({ type: 'set', patch: { color: color as typeof s.color } })
                          }
                        />
                      </label>
                    ))}
                    <span
                      aria-hidden
                      className="ml-2 size-2 rounded-full"
                      style={{ background: `var(${dotToken(s)})` }}
                    />
                  </div>
                </Field>
                <Field id="conn-group" label={t('connections.form.group')} error={err('groupId')}>
                  <Select
                    value={groups.some((g) => g.id === s.groupId) ? s.groupId : NO_GROUP}
                    onValueChange={(v) =>
                      dispatch({ type: 'set', patch: { groupId: v === NO_GROUP ? '' : v } })
                    }
                  >
                    <SelectTrigger {...invalid('conn-group', 'groupId')} className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {[...groups]
                        .sort((a, b) => a.sortOrder - b.sortOrder)
                        .map((g) => (
                          <SelectItem key={g.id} value={g.id}>
                            {g.name}
                          </SelectItem>
                        ))}
                      <SelectItem value={NO_GROUP}>{t('connections.noGroup')}</SelectItem>
                    </SelectContent>
                  </Select>
                </Field>
                <Field
                  id="conn-host"
                  label={t('connections.form.host')}
                  error={err('host') ?? err('port')}
                  errorId={err('host') ? 'conn-host-error' : 'conn-port-error'}
                >
                  <div className="flex gap-2">
                    <Input
                      {...invalid('conn-host', 'host')}
                      className="flex-1 font-mono"
                      placeholder="localhost"
                      value={s.host}
                      onChange={(e) => dispatch({ type: 'set', patch: { host: e.target.value } })}
                    />
                    <Input
                      {...invalid('conn-port', 'port')}
                      aria-label={t('connections.form.port')}
                      className="w-24 font-mono"
                      inputMode="numeric"
                      placeholder={engine?.defaultPort ? String(engine.defaultPort) : ''}
                      value={s.port ? String(s.port) : ''}
                      onChange={(e) =>
                        dispatch({
                          type: 'set',
                          // Empty means the engine's default port (0).
                          patch: { port: Number(e.target.value.replace(/\D/g, '').slice(0, 5)) },
                        })
                      }
                    />
                  </div>
                </Field>
                <Field id="conn-user" label={t('connections.form.user')} error={err('username')}>
                  <Input
                    {...invalid('conn-user', 'username')}
                    className="font-mono"
                    autoComplete="off"
                    value={s.username}
                    onChange={(e) => dispatch({ type: 'set', patch: { username: e.target.value } })}
                  />
                </Field>
                <Field
                  id="conn-password"
                  label={t('connections.form.password')}
                  error={err('password')}
                >
                  <div className="flex gap-2">
                    <Input
                      {...invalid('conn-password', 'password')}
                      type="password"
                      autoComplete="new-password"
                      className="flex-1"
                      placeholder={passwordPlaceholder}
                      value={form.password}
                      onChange={(e) => dispatch({ type: 'password', value: e.target.value })}
                    />
                    {editing?.passwordState === PasswordState.PasswordSaved && (
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        aria-label={
                          form.cleared
                            ? t('connections.form.keepPassword')
                            : t('connections.form.clearPassword')
                        }
                        onClick={() => dispatch({ type: 'cleared', cleared: !form.cleared })}
                      >
                        {form.cleared ? t('connections.form.undo') : t('connections.form.clear')}
                      </Button>
                    )}
                  </div>
                </Field>
                <Field
                  id="conn-database"
                  label={t('connections.form.database')}
                  error={err('database')}
                >
                  <Input
                    {...invalid('conn-database', 'database')}
                    className="font-mono"
                    value={s.database}
                    onChange={(e) => dispatch({ type: 'set', patch: { database: e.target.value } })}
                  />
                </Field>
              </TabsContent>

              <TabsContent value="ssl" className="grid gap-3 px-5 py-4">
                <Field
                  id="conn-ssl-mode"
                  label={t('connections.form.sslMode')}
                  error={err('ssl.mode')}
                >
                  <Select
                    value={s.ssl.mode}
                    onValueChange={(v) =>
                      dispatch({ type: 'ssl', patch: { mode: v as SSL['mode'] } })
                    }
                  >
                    <SelectTrigger {...invalid('conn-ssl-mode', 'ssl.mode')} className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {SSL_MODES.map((mode) => (
                        <SelectItem key={mode} value={mode}>
                          {t(
                            `connections.form.sslModes.${mode}` as 'connections.form.sslModes.prefer',
                          )}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
                <FileField
                  id="conn-ssl-ca"
                  label={t('connections.form.caFile')}
                  value={s.ssl.caFile}
                  onChange={(caFile) => dispatch({ type: 'ssl', patch: { caFile } })}
                />
                <FileField
                  id="conn-ssl-cert"
                  label={t('connections.form.certFile')}
                  value={s.ssl.certFile}
                  onChange={(certFile) => dispatch({ type: 'ssl', patch: { certFile } })}
                />
                <FileField
                  id="conn-ssl-key"
                  label={t('connections.form.keyFile')}
                  value={s.ssl.keyFile}
                  onChange={(keyFile) => dispatch({ type: 'ssl', patch: { keyFile } })}
                />
              </TabsContent>

              <TabsContent value="advanced" className="grid gap-3 px-5 py-4">
                <div className="flex items-start gap-3">
                  <Switch
                    id="conn-readonly"
                    checked={s.readOnly}
                    onCheckedChange={(readOnly) => dispatch({ type: 'set', patch: { readOnly } })}
                    aria-describedby="conn-readonly-hint"
                  />
                  <div className="grid gap-1">
                    <Label htmlFor="conn-readonly">{t('connections.form.readOnly')}</Label>
                    <p id="conn-readonly-hint" className="text-xs text-muted-foreground">
                      {t('connections.form.readOnlyHint')}
                    </p>
                  </div>
                </div>
              </TabsContent>
            </Tabs>

            <div className="grid gap-2 px-5 pb-4">
              {test.status !== 'idle' && <TestResultPanel state={test} />}
              {formError && <ErrorBox error={formError} />}
            </div>

            <DialogFooter className="mx-0 mb-0 sm:justify-between">
              {test.status === 'running' ? (
                <Button type="button" variant="outline" onClick={cancelTest}>
                  {t('connections.test.cancel')}
                </Button>
              ) : (
                <Button type="button" variant="outline" onClick={() => void runTest()}>
                  <Plug aria-hidden />
                  {t('connections.test.run')}
                </Button>
              )}
              <div className="flex gap-2">
                <Button type="button" variant="ghost" onClick={requestClose}>
                  {t('common.cancel')}
                </Button>
                <Button type="submit" disabled={saving}>
                  {t('common.save')}
                </Button>
              </div>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={discarding} onOpenChange={setDiscarding}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('connections.discard.title')}</AlertDialogTitle>
            <AlertDialogDescription>{t('connections.discard.body')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('connections.discard.keep')}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={close}>
              {t('connections.discard.discard')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

function ProdBadge() {
  const { t } = useTranslation()
  return (
    <span className="rounded-sm bg-env-production/15 px-1.5 text-[11px] leading-5 font-semibold text-env-production">
      {t('connections.env.production')}
    </span>
  )
}

// Field lays out a label and its control, with the field's error below it.
function Field({
  id,
  label,
  error,
  errorId,
  children,
}: {
  id?: string
  label: string
  error?: string
  errorId?: string
  children: ReactNode
}) {
  return (
    <div className="grid grid-cols-[110px_1fr] items-start gap-x-3">
      {id ? (
        <Label htmlFor={id} className="pt-2 text-muted-foreground">
          {label}
        </Label>
      ) : (
        <span className="pt-1.5 text-sm text-muted-foreground">{label}</span>
      )}
      <div className="grid min-w-0 gap-1">
        {children}
        {error && (
          <p id={errorId ?? `${id}-error`} className="text-xs text-destructive">
            {error}
          </p>
        )}
      </div>
    </div>
  )
}

// FileField is a path with a Browse… button; only the path is stored.
function FileField({
  id,
  label,
  value,
  onChange,
}: {
  id: string
  label: string
  value: string
  onChange: (path: string) => void
}) {
  const { t } = useTranslation()
  return (
    <Field id={id} label={label}>
      <div className="flex gap-2">
        <Input
          id={id}
          className="flex-1 font-mono"
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
        <Button
          type="button"
          variant="outline"
          aria-label={t('connections.form.browseFor', { label })}
          onClick={async () => {
            const path = await chooseFile(label)
            if (path) onChange(path)
          }}
        >
          {t('connections.form.browse')}
        </Button>
      </div>
    </Field>
  )
}

function ErrorBox({ error }: { error: AppError }) {
  const text = errorText(error)
  return (
    <div
      role="alert"
      className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs"
    >
      <p className="text-destructive">{text.message}</p>
      {text.hint && <p className="text-muted-foreground">{text.hint}</p>}
    </div>
  )
}
