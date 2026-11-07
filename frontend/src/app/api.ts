// Backend calls for the app shell. Components import from here, never from the
// generated bindings, so binding paths live in one place and tests can mock it.
import { AppService, type AppInfo } from '../../bindings/github.com/cokagung30/querra/internal/app'

export type { AppInfo }

export function getAppInfo(): Promise<AppInfo> {
  return AppService.Info()
}
