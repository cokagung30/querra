// Backend calls for saved connections. Components import from here, never from
// the generated bindings, so binding paths live in one place and tests can
// mock this module.
import { Dialogs, type CancellablePromise } from '@wailsio/runtime'

import { ConnectionService } from '../../../bindings/github.com/cokagung30/querra/internal/app'
import type {
  Connection,
  EngineInfo,
  Group,
  PasswordChange,
  Settings,
  TestResult,
} from '../../../bindings/github.com/cokagung30/querra/internal/connection'

export {
  Color,
  Env,
  PasswordAction,
  PasswordState,
  Stage,
  StageStatus,
} from '../../../bindings/github.com/cokagung30/querra/internal/connection'
export type {
  Connection,
  EngineInfo,
  Group,
  PasswordChange,
  SSL,
  Settings,
  StageResult,
  TestResult,
} from '../../../bindings/github.com/cokagung30/querra/internal/connection'
export { Engine, SSLMode } from '../../../bindings/github.com/cokagung30/querra/internal/driver'

// Go returns a nil slice as null.
const list = async <T>(p: Promise<T[] | null>): Promise<T[]> => (await p) ?? []

export const listConnections = (): Promise<Connection[]> => list(ConnectionService.List())
export const listGroups = (): Promise<Group[]> => list(ConnectionService.Groups())
export const listEngines = (): Promise<EngineInfo[]> => list(ConnectionService.Engines())

export const createConnection = (
  settings: Settings,
  password: PasswordChange,
): Promise<Connection> => ConnectionService.Create(settings, password)
export const updateConnection = (
  id: string,
  settings: Settings,
  password: PasswordChange,
): Promise<Connection> => ConnectionService.Update(id, settings, password)
export const deleteConnection = (id: string): Promise<void> => ConnectionService.Delete(id)
export const duplicateConnection = (id: string, name: string): Promise<Connection> =>
  ConnectionService.Duplicate(id, name)
export const arrangeConnections = (groupId: string, ids: string[]): Promise<void> =>
  ConnectionService.Arrange(groupId, ids)

// testConnection keeps the CancellablePromise so the dialog can cancel a test.
export const testConnection = (
  id: string,
  settings: Settings,
  password: PasswordChange,
): CancellablePromise<TestResult> => ConnectionService.Test(id, settings, password)

export const createGroup = (name: string): Promise<Group> => ConnectionService.CreateGroup(name)
export const renameGroup = (id: string, name: string): Promise<Group> =>
  ConnectionService.RenameGroup(id, name)
export const deleteGroup = (id: string): Promise<void> => ConnectionService.DeleteGroup(id)
export const arrangeGroups = (ids: string[]): Promise<void> => ConnectionService.ArrangeGroups(ids)

// chooseFile asks for one file with the native dialog; "" when cancelled.
export const chooseFile = (title: string): Promise<string> =>
  Dialogs.OpenFile({ Title: title, CanChooseFiles: true }) as Promise<string>
