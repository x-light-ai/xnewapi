/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Centralize model provider sync status presentation.

import type { TFunction } from 'i18next'

import type { ProviderSyncStatus } from './types'

export const providerSyncStatusVariant: Record<
  ProviderSyncStatus,
  'default' | 'secondary' | 'outline' | 'destructive'
> = {
  success: 'default',
  partial_success: 'outline',
  syncing: 'outline',
  error: 'destructive',
  stale: 'outline',
  never: 'secondary',
}

export function getProviderSyncStatusLabel(
  status: ProviderSyncStatus,
  t: TFunction
): string {
  if (status === 'success') return t('Synced')
  if (status === 'partial_success') return t('Partially synced')
  if (status === 'syncing') return t('Syncing...')
  if (status === 'error') return t('Sync failed')
  if (status === 'stale') return t('Sync overdue')
  return t('Not synced')
}

export function getEffectiveProviderSyncStatus(
  status: ProviderSyncStatus,
  lastSyncedAtTimestamp: number | null,
  syncIntervalMinutes: number,
  nowTimestamp = Date.now()
): ProviderSyncStatus {
  if (
    (status !== 'success' && status !== 'partial_success') ||
    !lastSyncedAtTimestamp ||
    syncIntervalMinutes < 1
  ) {
    return status
  }

  const expiresAt =
    lastSyncedAtTimestamp * 1000 + syncIntervalMinutes * 60 * 1000
  return nowTimestamp >= expiresAt ? 'stale' : status
}
