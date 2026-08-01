/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Keep provider and account sync failures discoverable without expanding the error UI.

import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import {
  getProviderSyncStatusLabel,
  providerSyncStatusVariant,
} from './provider-sync-status'
import type { ProviderSyncStatus } from './types'

type ProviderSyncStatusBadgeProps = {
  status: ProviderSyncStatus
  error?: string
}

export function ProviderSyncStatusBadge(props: ProviderSyncStatusBadgeProps) {
  const { t } = useTranslation('xnewapi')
  const badge = (
    <Badge variant={providerSyncStatusVariant[props.status]}>
      {getProviderSyncStatusLabel(props.status, t)}
    </Badge>
  )

  if (props.status !== 'error' || !props.error) return badge

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger render={badge} />
        <TooltipContent>{props.error}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
