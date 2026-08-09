/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Define the provider API view boundary without exposing stored credentials.

export type UpstreamProviderStatus = 'healthy' | 'disabled'
export type ProviderAccountStatus = 'healthy' | 'disabled'
export type ProviderKeyStatus = 'active' | 'disabled'
export type ProviderSyncStatus =
  | 'success'
  | 'partial_success'
  | 'syncing'
  | 'error'
  | 'never'
export type ProviderAuthenticationMethod = 'api-key' | 'password'
export type ProviderAdapterType = 'newapi' | 'sub2api'

export type ProviderChannel = {
  id: number
  name: string
  status: 'available' | 'disabled'
}

export type ProviderKey = {
  id: string
  externalId: string
  name: string
  maskedKey: string
  status: ProviderKeyStatus
}

export type ProviderGroup = {
  id: string
  name: string
  groupKey: string
  channels: ProviderChannel[]
  keys: ProviderKey[]
  profit: ProviderGroupProfit | null
}

export type ProviderGroupProfit = {
  groupKey: string
  groupId: number
  groupName: string
  providerId: number
  providerName: string
  accountId: number
  accountName: string
  keyIds: number[]
  revenue: number
  cost: number | null
  profit: number | null
  grossMargin: number | null
  costStatus: string
  lastSyncedAt: number
}

export type ProviderProfitDailyDetail = {
  date: string
  groupId: number
  groupName: string
  providerId: number
  providerName: string
  accountId: number
  accountName: string
  revenueQuota: number
  revenue: number
  providerUsageQuota: number
  cost: number | null
  profit: number | null
  grossMargin: number | null
  costStatus: string
  costObservedAt: number
}

export type ProviderProfitDailyDetailPage = {
  items: ProviderProfitDailyDetail[]
  total: number
  page: number
  pageSize: number
  revenue: number
  cost: number | null
  profit: number | null
  grossMargin: number | null
}

export type ProviderProfitDailyTrendPoint = {
  date: string
  revenue: number
  cost: number | null
  profit: number | null
  costStatus: string
}

export type ProviderProfitDateRange = {
  startDate: string
  endDate: string
}

export type ProviderProfitDetailsSearch = {
  startDate?: string
  endDate?: string
  providerId?: string
  groupId?: string
  page?: number
  pageSize?: number
}

export type ProviderAccount = {
  id: string
  name: string
  externalId: string
  loginUsername: string
  status: ProviderAccountStatus
  syncApiKey: string
  hasSyncApiKey?: boolean
  clearSyncApiKey?: boolean
  autoSync: boolean
  balance: number
  totalRecharge: number
  syncStatus?: ProviderSyncStatus
  lastSyncError?: string
  lastSyncedAt: string
  groups: ProviderGroup[]
}

export type UpstreamProvider = {
  id: string
  name: string
  endpoint: string
  status: UpstreamProviderStatus
  syncStatus: ProviderSyncStatus
  lastSyncedAt: string | null
  lastSyncError?: string
  authenticationMethod: ProviderAuthenticationMethod
  adapterType: ProviderAdapterType
  syncIntervalMinutes: number
  rechargeRatio: number
  quotaConversionBase: number
  currency: string
  accounts: ProviderAccount[]
}

export type ProviderTotals = {
  accountCount: number
  groupCount: number
  keyCount: number
  mappedGroupCount: number
  balance: number
  totalRecharge: number
  revenue: number
  cost: number | null
  profit: number | null
  margin: number | null
}
