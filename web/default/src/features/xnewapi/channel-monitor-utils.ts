/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Preserve the classic channel-monitor table behavior in the default frontend.

import type { ChannelMonitorItem } from './types'

export type ChannelMonitorSortKey =
  | 'success_rate'
  | 'avg_latency'
  | 'p95_latency'
  | 'failure_count'
  | 'last_active'
  | 'name'
  | 'request_count'
  | 'group_name'
  | 'group_success_rate'

export type ChannelMonitorSortOrder = 'asc' | 'desc'

export type ChannelMonitorGroupRow = {
  kind: 'group'
  id: string
  group_name: string
  channel_count: number
  request_count: number
  failure_count: number
  success_rate: number
  avg_latency: number
  p95_latency: number
  last_active: string
}

export type ChannelMonitorDisplayRow =
  | { kind: 'channel'; item: ChannelMonitorItem }
  | ChannelMonitorGroupRow

export function formatRate(value: number): string {
  if (!Number.isFinite(value)) return '0.0%'
  return `${(value * 100).toFixed(1)}%`
}

export function formatLatency(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '-'
  return `${Math.round(value)} ms`
}

export function normalizeGroupName(groupName: string): string {
  return groupName.trim() || 'default'
}

export function getSuggestedWeightScore(item: ChannelMonitorItem): number {
  if (item.temporary_circuit_open) return 0.05

  const successRate = Math.max(0, Math.min(1, item.success_rate))
  let latencyScore = 0.5
  if (item.avg_latency > 0) {
    latencyScore = Math.max(0, Math.min(1, 1 - item.avg_latency / 6000))
  }

  let p95Penalty = 0
  if (item.p95_latency > 0) {
    p95Penalty = Math.max(0, Math.min(0.35, (item.p95_latency - 1500) / 10000))
  }

  let score = successRate * 0.7 + latencyScore * 0.3 - p95Penalty
  if (successRate < 0.7) score -= 0.15
  if (item.avg_latency > 5000) score -= 0.1
  return Math.max(0.05, Math.min(1, score))
}

function compareNumber(left: number, right: number): number {
  return left - right
}

function compareChannels(
  left: ChannelMonitorItem,
  right: ChannelMonitorItem,
  sortKey: ChannelMonitorSortKey
): number {
  switch (sortKey) {
    case 'success_rate':
      return (
        compareNumber(left.success_rate, right.success_rate) ||
        compareNumber(left.failure_count, right.failure_count) ||
        compareNumber(left.request_count, right.request_count)
      )
    case 'avg_latency': {
      const leftHasLatency = left.avg_latency > 0
      const rightHasLatency = right.avg_latency > 0
      if (leftHasLatency !== rightHasLatency) return leftHasLatency ? -1 : 1
      return (
        compareNumber(left.avg_latency, right.avg_latency) ||
        compareNumber(left.p95_latency, right.p95_latency)
      )
    }
    case 'p95_latency':
      return (
        compareNumber(left.p95_latency, right.p95_latency) ||
        compareNumber(left.avg_latency, right.avg_latency)
      )
    case 'failure_count':
      return (
        compareNumber(left.failure_count, right.failure_count) ||
        compareNumber(left.request_count, right.request_count)
      )
    case 'last_active':
      return (
        compareNumber(
          left.last_active ? new Date(left.last_active).getTime() : 0,
          right.last_active ? new Date(right.last_active).getTime() : 0
        ) || left.name.localeCompare(right.name)
      )
    case 'name':
      return left.name.localeCompare(right.name)
    case 'group_name':
    case 'group_success_rate':
      return (
        normalizeGroupName(left.group_name).localeCompare(
          normalizeGroupName(right.group_name)
        ) ||
        (sortKey === 'group_success_rate'
          ? compareNumber(left.success_rate, right.success_rate)
          : left.name.localeCompare(right.name))
      )
    default:
      return (
        compareNumber(left.request_count, right.request_count) ||
        left.name.localeCompare(right.name)
      )
  }
}

function compareGroupRows(
  left: ChannelMonitorGroupRow,
  right: ChannelMonitorGroupRow,
  sortKey: ChannelMonitorSortKey
): number {
  switch (sortKey) {
    case 'success_rate':
      return (
        compareNumber(left.success_rate, right.success_rate) ||
        compareNumber(left.failure_count, right.failure_count) ||
        compareNumber(left.request_count, right.request_count)
      )
    case 'group_success_rate':
      return compareNumber(left.success_rate, right.success_rate)
    case 'avg_latency': {
      const leftHasLatency = left.avg_latency > 0
      const rightHasLatency = right.avg_latency > 0
      if (leftHasLatency !== rightHasLatency) return leftHasLatency ? -1 : 1
      return (
        compareNumber(left.avg_latency, right.avg_latency) ||
        compareNumber(left.p95_latency, right.p95_latency)
      )
    }
    case 'p95_latency':
      return (
        compareNumber(left.p95_latency, right.p95_latency) ||
        compareNumber(left.avg_latency, right.avg_latency)
      )
    case 'failure_count':
      return (
        compareNumber(left.failure_count, right.failure_count) ||
        compareNumber(left.request_count, right.request_count)
      )
    case 'last_active':
      return compareNumber(
        left.last_active ? new Date(left.last_active).getTime() : 0,
        right.last_active ? new Date(right.last_active).getTime() : 0
      )
    case 'request_count':
      return compareNumber(left.request_count, right.request_count)
    default:
      return left.group_name.localeCompare(right.group_name)
  }
}

export function sortChannelMonitorItems(
  items: ChannelMonitorItem[],
  sortKey: ChannelMonitorSortKey,
  order: ChannelMonitorSortOrder
): ChannelMonitorItem[] {
  const direction = order === 'asc' ? 1 : -1
  return [...items].sort(
    (left, right) => compareChannels(left, right, sortKey) * direction
  )
}

function buildGroupRow(
  groupName: string,
  items: ChannelMonitorItem[]
): ChannelMonitorGroupRow {
  let requestCount = 0
  let failureCount = 0
  let weightedLatency = 0
  let p95Latency = 0
  let lastActive = ''

  for (const item of items) {
    requestCount += item.request_count
    failureCount += item.failure_count
    weightedLatency += item.avg_latency * item.request_count
    p95Latency = Math.max(p95Latency, item.p95_latency)
    if (
      item.last_active &&
      (!lastActive ||
        new Date(item.last_active).getTime() > new Date(lastActive).getTime())
    ) {
      lastActive = item.last_active
    }
  }

  return {
    kind: 'group',
    id: `group-${groupName}`,
    group_name: groupName,
    channel_count: items.length,
    request_count: requestCount,
    failure_count: failureCount,
    success_rate:
      requestCount > 0 ? (requestCount - failureCount) / requestCount : 0,
    avg_latency: requestCount > 0 ? weightedLatency / requestCount : 0,
    p95_latency: p95Latency,
    last_active: lastActive,
  }
}

export function buildChannelMonitorRows(
  items: ChannelMonitorItem[],
  sortKey: ChannelMonitorSortKey,
  order: ChannelMonitorSortOrder,
  grouped: boolean
): ChannelMonitorDisplayRow[] {
  const sortedItems = sortChannelMonitorItems(items, sortKey, order)
  if (!grouped) {
    return sortedItems.map((item) => ({ kind: 'channel', item }))
  }

  const groups = new Map<string, ChannelMonitorItem[]>()
  for (const item of sortedItems) {
    const groupName = normalizeGroupName(item.group_name)
    const groupItems = groups.get(groupName)
    if (groupItems) groupItems.push(item)
    else groups.set(groupName, [item])
  }

  const groupedRows = [...groups.entries()].map(([groupName, groupItems]) => ({
    summary: buildGroupRow(groupName, groupItems),
    items: groupItems,
  }))
  const direction = order === 'asc' ? 1 : -1
  groupedRows.sort((left, right) => {
    const metricCompare =
      compareGroupRows(left.summary, right.summary, sortKey) * direction
    if (metricCompare !== 0) return metricCompare
    return left.summary.group_name.localeCompare(right.summary.group_name)
  })

  return groupedRows.flatMap(({ summary, items: groupItems }) => [
    summary,
    ...groupItems.map((item) => ({ kind: 'channel' as const, item })),
  ])
}
