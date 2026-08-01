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
  return t('Not synced')
}
