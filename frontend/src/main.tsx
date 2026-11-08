import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from '@/app/App'
import { registerShellCommands } from '@/app/commands'
import { followSystemTheme } from '@/app/theme'
import { commands, installKeybindings } from '@/lib/commands'

import '@/i18n'
import './index.css'

// App-lifetime setup, done once before the first render so the first frame
// already has its theme and commands.
followSystemTheme()
registerShellCommands(commands)
installKeybindings(commands)

createRoot(document.getElementById('root') as HTMLElement).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
