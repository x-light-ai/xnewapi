/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Aggregate provider groups into sortable channel performance rows.

import type { UpstreamProvider } from './provider-management/types'
import type { ChannelMonitorItem } from './types'

export type RankingSortKey =
  | 'request_count'
  | 'failure_count'
  | 'success_rate'
  | 'failure_rate'
  | 'gross_margin'
  | 'revenue'

export type RankingRow = {
  id: string
  channelNames: string[]
  providerName: string
  providerEndpoint: string
  requestCount: number
  failureCount: number
  successRate: number
  failureRate: number
  grossMargin: number | null
  revenue: number | null
}

function numericValue(row: RankingRow, key: RankingSortKey): number | null {
  switch (key) {
    case 'request_count':
      return row.requestCount
    case 'failure_count':
      return row.failureCount
    case 'success_rate':
      return row.successRate
    case 'failure_rate':
      return row.failureRate
    case 'gross_margin':
      return row.grossMargin
    case 'revenue':
      return row.revenue
  }
}

export function buildChannelPerformanceRows(
  items: ChannelMonitorItem[],
  providers: UpstreamProvider[]
): RankingRow[] {
  const monitorByChannel = new Map(items.map((item) => [item.id, item]))
  const mappedChannelIds = new Set<number>()
  const result: RankingRow[] = []
  for (const provider of providers) {
    for (const account of provider.accounts) {
      for (const group of account.groups) {
        const monitoredChannels = group.channels.flatMap((channel) => {
          const monitor = monitorByChannel.get(channel.id)
          return monitor ? [monitor] : []
        })
        if (monitoredChannels.length === 0) continue
        let requestCount = 0
        let failureCount = 0
        for (const channel of monitoredChannels) {
          mappedChannelIds.add(channel.id)
          requestCount += channel.request_count
          failureCount += channel.failure_count
        }
        const failureRate = requestCount > 0 ? failureCount / requestCount : 0
        result.push({
          id: group.groupKey,
          channelNames: monitoredChannels.map((channel) => channel.name),
          providerName: provider.name,
          providerEndpoint: provider.endpoint || '',
          requestCount,
          failureCount,
          successRate: requestCount > 0 ? 1 - failureRate : 0,
          failureRate,
          grossMargin: group.profit?.grossMargin ?? null,
          revenue: group.profit?.revenue ?? null,
        })
      }
    }
  }
  for (const item of items) {
    if (mappedChannelIds.has(item.id)) continue
    const failureRate =
      item.request_count > 0 ? item.failure_count / item.request_count : 0
    result.push({
      id: `channel-${item.id}`,
      channelNames: [item.name],
      providerName: '-',
      providerEndpoint: '',
      requestCount: item.request_count,
      failureCount: item.failure_count,
      successRate: item.success_rate,
      failureRate,
      grossMargin: item.gross_margin ?? null,
      revenue: null,
    })
  }
  return result
}

export function sortChannelPerformanceRows(
  rows: RankingRow[],
  sortKey: RankingSortKey,
  sortOrder: 'asc' | 'desc'
): RankingRow[] {
  return rows.toSorted((left, right) => {
    const leftValue = numericValue(left, sortKey)
    const rightValue = numericValue(right, sortKey)
    if (leftValue == null || rightValue == null) {
      if (leftValue == null && rightValue == null) {
        return left.channelNames
          .join(',')
          .localeCompare(right.channelNames.join(','))
      }
      return leftValue == null ? 1 : -1
    }
    const comparison = leftValue - rightValue
    if (comparison !== 0) {
      return sortOrder === 'asc' ? comparison : -comparison
    }
    return left.channelNames
      .join(',')
      .localeCompare(right.channelNames.join(','))
  })
}
