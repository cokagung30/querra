import type { LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { commands, useKeybindingLabel } from '@/lib/commands'
import { cn } from '@/lib/utils'

interface CommandButtonProps {
  commandId: string
  icon: LucideIcon
  // Set for toggles, e.g. whether the panel the command toggles is visible.
  pressed?: boolean
  arg?: unknown
  className?: string
}

// CommandButton is an icon button that runs a registered command. Its
// tooltip shows the command's title and current keybinding.
export function CommandButton({
  commandId,
  icon: Icon,
  pressed,
  arg,
  className,
}: CommandButtonProps) {
  const { t } = useTranslation()
  // Also re-renders when commands change, which keeps commands.get current.
  const keybinding = useKeybindingLabel(commandId)
  const command = commands.get(commandId)
  if (!command) return null

  const title = t(command.titleKey)
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label={title}
          aria-pressed={pressed}
          className={cn('text-muted-foreground', className)}
          onClick={() => commands.execute(commandId, arg)}
        >
          <Icon />
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        {keybinding ? t('shortcut.withKeybinding', { title, keybinding }) : title}
      </TooltipContent>
    </Tooltip>
  )
}
