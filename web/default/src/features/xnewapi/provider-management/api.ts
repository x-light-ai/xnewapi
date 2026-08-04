/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Connect provider management to the fork-owned backend contract.

import dayjs from 'dayjs'

import { api } from '@/lib/api'

import type {
  ProviderChannel,
  ProviderProfitDailyDetail,
  ProviderGroupProfit,
  UpstreamProvider,
} from './types'

type ApiResponse<T> = { success: boolean; message: string; data: T }
type Page<T> = { items: T[]; total: number; page: number; page_size: number }

type RawKey = {
  id: number
  account_id: number
  provider_group_id: number
  name: string
  external_id: string
  key_masked: string
  status: 'active' | 'disabled'
  last_usage_at: number
}

type RawGroup = {
  id: number
  account_id: number
  name: string
  channel_ids: number[]
  keys?: RawKey[]
}

type RawAccount = {
  id: number
  provider_id: number
  name: string
  external_id: string
  login_username: string
  status: 'enabled' | 'disabled'
  auto_sync: boolean
  has_sync_api_key: boolean
  balance: string | number
  total_recharge: string | number
  sync_status: 'success' | 'partial_success' | 'syncing' | 'error' | 'never'
  last_sync_error?: string
  last_synced_at: number
  groups?: RawGroup[]
}

type RawProvider = {
  id: number
  name: string
  endpoint: string
  status: 'enabled' | 'disabled'
  authentication_method: 'api_key' | 'password'
  adapter_type: string
  sync_interval_minutes: number
  recharge_ratio: string | number
  quota_conversion_base: string | number
  currency: string
  sync_status: 'success' | 'partial_success' | 'syncing' | 'error' | 'never'
  last_synced_at: number
  last_sync_error?: string
  accounts?: RawAccount[]
}

export type ProviderProfitItem = {
  group_key: string
  group_id: number
  group_name: string
  provider_id: number
  provider_name: string
  account_id: number
  account_name: string
  key_ids: number[]
  revenue: string | number
  cost: string | number | null
  profit: string | number | null
  gross_margin: string | number | null
  cost_status: string
  last_synced_at: number
}

type RawProviderProfitDailyDetail = {
  date: string
  group_id: number
  group_name: string
  provider_id: number
  provider_name: string
  account_id: number
  account_name: string
  revenue_quota: number
  revenue: string | number
  provider_usage_quota: string | number
  cost: string | number | null
  profit: string | number | null
  gross_margin: string | number | null
  cost_status: string
  cost_observed_at: number
}

function unwrap<T>(response: ApiResponse<T>): T {
  if (!response.success) throw new Error(response.message || 'Request failed')
  return response.data
}

function timestampLabel(value: number): string {
  if (!value) return ''
  return new Date(value * 1000).toLocaleString()
}

export async function getProviderWorkspace() {
  const channelOptionsPromise = api
    .get<ApiResponse<Array<{ id: number; name: string; status: number }>>>(
      '/api/xnewapi/providers/channels'
    )
    .then((response) => unwrap(response.data))
    .catch(() => [])
  const profitsPromise = api
    .get<ApiResponse<ProviderProfitItem[]>>(
      '/api/xnewapi/providers/profit-ranking',
      {
        params: { start_date: dayjs().startOf('month').format('YYYY-MM-DD') },
      }
    )
    .then((response) => unwrap(response.data))
    .catch(() => [])
  const [providersResponse, rawChannelOptions, profits] = await Promise.all([
    api.get<ApiResponse<Page<RawProvider>>>('/api/xnewapi/providers', {
      params: { p: 1, page_size: 100 },
    }),
    channelOptionsPromise,
    profitsPromise,
  ])
  const rawProviders = unwrap(providersResponse.data).items
  const channelOptions: ProviderChannel[] = rawChannelOptions.map(
    (channel) => ({
      id: channel.id,
      name: channel.name,
      status: channel.status === 1 ? 'available' : 'disabled',
    })
  )
  const channelMap = new Map(channelOptions.map((item) => [item.id, item]))
  const profitByGroup = new Map(profits.map((item) => [item.group_key, item]))

  const providers: UpstreamProvider[] = rawProviders.map((provider) => ({
    id: String(provider.id),
    name: provider.name,
    endpoint: provider.endpoint,
    status: provider.status === 'enabled' ? 'healthy' : 'disabled',
    syncStatus: provider.sync_status,
    lastSyncedAt: provider.last_synced_at
      ? timestampLabel(provider.last_synced_at)
      : null,
    lastSyncError: provider.last_sync_error,
    authenticationMethod:
      provider.authentication_method === 'password' ? 'password' : 'api-key',
    adapterType: provider.adapter_type === 'sub2api' ? 'sub2api' : 'newapi',
    syncIntervalMinutes: provider.sync_interval_minutes,
    rechargeRatio: Number(provider.recharge_ratio),
    quotaConversionBase: Number(provider.quota_conversion_base),
    currency: provider.currency,
    accounts: (provider.accounts ?? []).map((account) => ({
      id: String(account.id),
      name: account.name,
      externalId: account.external_id,
      loginUsername: account.login_username,
      status: account.status === 'enabled' ? 'healthy' : 'disabled',
      syncApiKey: '',
      hasSyncApiKey: account.has_sync_api_key,
      autoSync: account.auto_sync,
      balance: Number(account.balance),
      totalRecharge: Number(account.total_recharge),
      syncStatus: account.sync_status,
      lastSyncError: account.last_sync_error,
      lastSyncedAt: timestampLabel(account.last_synced_at),
      groups: (account.groups ?? []).map((group) => {
        const groupKey = `${provider.id}:${account.id}:${group.id}`
        const profit = profitByGroup.get(groupKey)
        const groupProfit: ProviderGroupProfit | null = profit
          ? {
              groupKey: profit.group_key,
              groupId: profit.group_id,
              groupName: profit.group_name,
              providerId: profit.provider_id,
              providerName: profit.provider_name,
              accountId: profit.account_id,
              accountName: profit.account_name,
              keyIds: profit.key_ids,
              revenue: Number(profit.revenue),
              cost: profit.cost == null ? null : Number(profit.cost),
              profit: profit.profit == null ? null : Number(profit.profit),
              grossMargin:
                profit.gross_margin == null
                  ? null
                  : Number(profit.gross_margin),
              costStatus: profit.cost_status,
              lastSyncedAt: profit.last_synced_at,
            }
          : null
        return {
          id: String(group.id),
          name: group.name,
          groupKey,
          channels: group.channel_ids.flatMap((channelId) => {
            const channel = channelMap.get(channelId)
            return channel ? [channel] : []
          }),
          keys: (group.keys ?? []).map((key) => ({
            id: String(key.id),
            externalId: key.external_id,
            name: key.name,
            maskedKey: key.key_masked,
            status: key.status,
          })),
          profit: groupProfit,
        }
      }),
    })),
  }))
  return { providers, channelOptions, profits }
}

export async function getProviderProfitDetails(params: {
  startDate: string
  endDate: string
  providerId?: string
  groupId?: string
}): Promise<ProviderProfitDailyDetail[]> {
  const response = await api.get<ApiResponse<RawProviderProfitDailyDetail[]>>(
    '/api/xnewapi/providers/profit-details',
    {
      params: {
        start_date: params.startDate,
        end_date: params.endDate,
        provider_id: params.providerId,
        group_id: params.groupId,
      },
    }
  )
  return unwrap(response.data).map((item) => ({
    date: item.date,
    groupId: item.group_id,
    groupName: item.group_name,
    providerId: item.provider_id,
    providerName: item.provider_name,
    accountId: item.account_id,
    accountName: item.account_name,
    revenueQuota: Number(item.revenue_quota),
    revenue: Number(item.revenue),
    providerUsageQuota: Number(item.provider_usage_quota),
    cost: item.cost == null ? null : Number(item.cost),
    profit: item.profit == null ? null : Number(item.profit),
    grossMargin: item.gross_margin == null ? null : Number(item.gross_margin),
    costStatus: item.cost_status,
    costObservedAt: item.cost_observed_at,
  }))
}

export async function saveProvider(provider: UpstreamProvider) {
  const providerId = Number(provider.id)
  const payload = {
    ...(Number.isFinite(providerId) ? { id: providerId } : {}),
    name: provider.name,
    endpoint: provider.endpoint,
    status: provider.status === 'healthy' ? 'enabled' : 'disabled',
    authentication_method:
      provider.authenticationMethod === 'password' ? 'password' : 'api_key',
    adapter_type: provider.adapterType,
    sync_interval_minutes: provider.syncIntervalMinutes,
    recharge_ratio: provider.rechargeRatio,
    quota_conversion_base:
      provider.adapterType === 'sub2api' ? 1 : provider.quotaConversionBase,
    currency: provider.currency,
    accounts: provider.accounts.map((account) => {
      const accountId = Number(account.id)
      let credentialUpdate: {
        login_password?: string
        sync_api_key?: string
      } = {}
      if (account.clearSyncApiKey) {
        credentialUpdate =
          provider.adapterType === 'sub2api'
            ? { login_password: '' }
            : { sync_api_key: '' }
      } else if (account.syncApiKey) {
        credentialUpdate =
          provider.adapterType === 'sub2api'
            ? { login_password: account.syncApiKey }
            : { sync_api_key: account.syncApiKey }
      }
      return {
        ...(Number.isFinite(accountId) ? { id: accountId } : {}),
        name: account.name,
        external_id: account.externalId,
        login_username: account.loginUsername,
        status: account.status === 'healthy' ? 'enabled' : 'disabled',
        auto_sync: account.autoSync,
        ...credentialUpdate,
        groups: account.groups.map((group) => {
          const groupId = Number(group.id)
          return {
            ...(Number.isFinite(groupId) ? { id: groupId } : {}),
            name: group.name,
            channel_ids: group.channels.map((channel) => channel.id),
          }
        }),
      }
    }),
  }
  unwrap(
    (
      await api.put<ApiResponse<RawProvider>>(
        '/api/xnewapi/providers/workspace',
        payload
      )
    ).data
  )
}

export async function syncProvider(providerId: string) {
  const response = await api.post<ApiResponse<unknown>>(
    `/api/xnewapi/providers/${providerId}/sync`
  )
  return unwrap(response.data)
}

export async function syncProviderAccount(accountId: string) {
  const response = await api.post<ApiResponse<unknown>>(
    `/api/xnewapi/providers/accounts/${accountId}/sync`
  )
  return unwrap(response.data)
}

export async function adjustProviderAccountRecharge(
  accountId: string,
  delta: string
) {
  const response = await api.post<ApiResponse<unknown>>(
    `/api/xnewapi/providers/accounts/${accountId}/recharge-adjust`,
    { delta }
  )
  return unwrap(response.data)
}

export async function deleteProvider(providerId: string) {
  const response = await api.delete<ApiResponse<unknown>>(
    `/api/xnewapi/providers/${providerId}`
  )
  return unwrap(response.data)
}

export async function deleteProviderAccount(accountId: string) {
  const response = await api.delete<ApiResponse<unknown>>(
    `/api/xnewapi/providers/accounts/${accountId}`
  )
  return unwrap(response.data)
}

export async function deleteProviderGroup(groupId: string) {
  const response = await api.delete<ApiResponse<unknown>>(
    `/api/xnewapi/providers/groups/${groupId}`
  )
  return unwrap(response.data)
}
