// Test fixtures for saved connections and groups.
import type { Connection, Group, Settings } from '@/features/connections/api'

export function conn(
  id: string,
  groupId: string,
  sortOrder: number,
  // Plain strings are fine for the enum fields, e.g. { env: 'production' }.
  settings: { [K in keyof Settings]?: Settings[K] | string } = {},
  passwordState = 'none',
): Connection {
  return {
    id,
    sortOrder,
    passwordState,
    createdAt: '',
    updatedAt: '',
    settings: {
      groupId,
      name: id,
      engine: 'postgres',
      env: 'local',
      color: '',
      host: 'localhost',
      port: 0,
      username: 'app',
      database: id,
      ssl: { mode: 'prefer', caFile: '', certFile: '', keyFile: '' },
      readOnly: false,
      ...settings,
    },
  } as Connection
}

export const group = (id: string, sortOrder: number, name = id.toUpperCase()): Group => ({
  id,
  name,
  sortOrder,
})
