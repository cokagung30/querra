import { QueryClient } from '@tanstack/react-query'

// newQueryClient is for data from the local Go backend: it only changes through
// our own mutations, which invalidate it, so there is nothing to refetch on
// focus and no flaky network to retry.
export function newQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity, refetchOnWindowFocus: false },
      mutations: { retry: false },
    },
  })
}

export const queryClient = newQueryClient()
