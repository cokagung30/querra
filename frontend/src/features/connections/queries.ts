import { useQuery } from '@tanstack/react-query'

import { listConnections, listEngines, listGroups } from './api'

// Query keys for data from ConnectionService. Every mutation invalidates the
// ones it changes; nothing else changes this data.
export const connectionKeys = {
  connections: ['connections'] as const,
  groups: ['groups'] as const,
  engines: ['engines'] as const,
}

export const useConnections = () =>
  useQuery({ queryKey: connectionKeys.connections, queryFn: listConnections })
export const useGroups = () => useQuery({ queryKey: connectionKeys.groups, queryFn: listGroups })
export const useEngines = () => useQuery({ queryKey: connectionKeys.engines, queryFn: listEngines })
