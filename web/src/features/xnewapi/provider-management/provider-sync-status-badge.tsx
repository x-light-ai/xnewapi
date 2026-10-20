/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Keep provider and account sync failures discoverable without expanding the error UI.

import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import {
  getEffectiveProviderSyncStatus,
  getProviderSyncStatusLabel,
  providerSyncStatusVariant,
} from './provider-sync-status'
import type { ProviderSyncStatus } from './types'

type ProviderSyncStatusBadgeProps = {
  status: ProviderSyncStatus
  error?: string
  lastSyncedAtTimestamp?: number | null
  syncIntervalMinutes?: number
  nowTimestamp?: number
}

export function ProviderSyncStatusBadge(props: ProviderSyncStatusBadgeProps) {
  const { t } = useTranslation('xnewapi')
  const [currentTimestamp, setCurrentTimestamp] = useState(() => Date.now())
  useEffect(() => {
    if (
      (props.status !== 'success' && props.status !== 'partial_success') ||
      !props.lastSyncedAtTimestamp ||
      !props.syncIntervalMinutes
    ) {
      return
    }
    const timer = window.setInterval(
      () => setCurrentTimestamp(Date.now()),
      60_000
    )
    return () => window.clearInterval(timer)
  }, [props.status, props.lastSyncedAtTimestamp, props.syncIntervalMinutes])
  const status = getEffectiveProviderSyncStatus(
    props.status,
    props.lastSyncedAtTimestamp ?? null,
    props.syncIntervalMinutes ?? 0,
    props.nowTimestamp ?? currentTimestamp
  )
  const badge = (
    <Badge variant={providerSyncStatusVariant[status]}>
      {getProviderSyncStatusLabel(status, t)}
    </Badge>
  )

  if (status !== 'error' || !props.error) return badge

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger render={badge} />
        <TooltipContent>{props.error}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
