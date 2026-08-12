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
  groupNames: string[]
  providerName: string
  providerEndpoint: string
  requestCount: number
  failureCount: number
  successRate: number
  failureRate: number
  grossMargin: number | null
  revenue: number | null
}

export type RecommendedChannelRow = RankingRow & {
  recommendationScore: number
}

export type RecommendedChannelGroup = {
  groupName: string
  rows: RecommendedChannelRow[]
}

function channelGroupNames(items: ChannelMonitorItem[]): string[] {
  return [
    ...new Set(
      items.flatMap((item) =>
        item.group_name
          .split(',')
          .map((groupName) => groupName.trim())
          .filter(Boolean)
      )
    ),
  ].toSorted((left, right) => left.localeCompare(right))
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
          groupNames: channelGroupNames(monitoredChannels),
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
      groupNames: channelGroupNames([item]),
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

export function buildRecommendedChannelRows(
  rows: RankingRow[],
  limit: number = 5
): RecommendedChannelRow[] {
  if (limit <= 0) return []
  return rows
    .flatMap((row) => {
      if (
        row.requestCount <= 0 ||
        row.grossMargin == null ||
        !Number.isFinite(row.grossMargin)
      ) {
        return []
      }
      const successes = Math.max(0, row.requestCount - row.failureCount)
      const bayesianSuccessRate = (successes + 19) / (row.requestCount + 20)
      const confidence = row.requestCount / (row.requestCount + 50)
      const stabilityScore = bayesianSuccessRate * (0.5 + 0.5 * confidence)
      const valueScore = Math.min(1, Math.max(0, (row.grossMargin + 10) / 60))
      return [
        {
          ...row,
          recommendationScore: (stabilityScore * 0.5 + valueScore * 0.5) * 100,
        },
      ]
    })
    .toSorted((left, right) => {
      if (left.recommendationScore !== right.recommendationScore) {
        return right.recommendationScore - left.recommendationScore
      }
      if (left.requestCount !== right.requestCount) {
        return right.requestCount - left.requestCount
      }
      return left.channelNames
        .join(',')
        .localeCompare(right.channelNames.join(','))
    })
    .slice(0, limit)
}

export function buildRecommendedChannelGroups(
  rows: RankingRow[],
  limitPerGroup: number = 3
): RecommendedChannelGroup[] {
  if (limitPerGroup <= 0) return []
  const rowsByGroup = new Map<string, RankingRow[]>()
  for (const row of rows) {
    for (const groupName of row.groupNames) {
      const groupRows = rowsByGroup.get(groupName) ?? []
      groupRows.push(row)
      rowsByGroup.set(groupName, groupRows)
    }
  }
  return [...rowsByGroup.entries()]
    .toSorted(([left], [right]) => left.localeCompare(right))
    .flatMap(([groupName, groupRows]) => {
      const recommendedRows = buildRecommendedChannelRows(
        groupRows,
        limitPerGroup
      )
      return recommendedRows.length > 0
        ? [{ groupName, rows: recommendedRows }]
        : []
    })
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
