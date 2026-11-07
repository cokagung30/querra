import { useEffect, useState } from 'react'

import { SpikePage } from '@/features/spike/SpikePage'

import { getAppInfo, type AppInfo } from './api'

// Placeholder shell until M0.3 builds the real layout; it hosts the M0.2 spike.
export function App() {
  const [info, setInfo] = useState<AppInfo | null>(null)

  useEffect(() => {
    getAppInfo()
      .then(setInfo)
      .catch((err: unknown) => console.error('load app info:', err))
  }, [])

  return (
    <main className="flex h-screen flex-col gap-3 p-4">
      <header className="flex items-baseline gap-2">
        <h1 className="text-lg font-semibold">Querra</h1>
        <p className="text-sm text-muted-foreground">{info ? `v${info.version}` : '…'}</p>
        <p className="ml-auto text-xs text-muted-foreground">M0.2 stack spike</p>
      </header>
      <SpikePage />
    </main>
  )
}
