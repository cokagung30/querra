import { useEffect, useState } from 'react'

import { getAppInfo, type AppInfo } from './api'

// Placeholder shell until M0.3 builds the real layout.
export function App() {
  const [info, setInfo] = useState<AppInfo | null>(null)

  useEffect(() => {
    getAppInfo()
      .then(setInfo)
      .catch((err: unknown) => console.error('load app info:', err))
  }, [])

  return (
    <main className="flex h-screen flex-col items-center justify-center gap-1">
      <h1 className="text-2xl font-semibold">Querra</h1>
      <p className="text-sm text-muted-foreground">{info ? `v${info.version}` : '…'}</p>
    </main>
  )
}
