/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Provide default-frontend API access to fork-owned monitor routes.

import { api } from '@/lib/api'

import type {
  ApiResponse,
  ChannelHealthPoint,
  ChannelMonitorPageData,
  ChannelMonitorSummary,
  ChannelRankings,
  ChannelTimeline,
} from './types'

function unwrap<T>(response: ApiResponse<T>): T {
  if (!response.success) {
    throw new Error(response.message || 'Request failed')
  }
  return response.data
}

export async function getChannelMonitorSummary(days: number) {
  const response = await api.get<ApiResponse<ChannelMonitorSummary>>(
    '/api/channel_monitor/summary',
    { params: { days } }
  )
  return unwrap(response.data)
}

export async function getChannelMonitorHealth(days: number) {
  const response = await api.get<ApiResponse<ChannelHealthPoint[]>>(
    '/api/channel_monitor/health',
    { params: { days } }
  )
  return unwrap(response.data)
}

export async function getChannelMonitorChannels(days: number, group: string) {
  const response = await api.get<ApiResponse<ChannelMonitorPageData>>(
    '/api/channel_monitor/channels',
    { params: { days, group, all: true } }
  )
  return unwrap(response.data)
}

export async function getChannelMonitorTimeline(group: string) {
  const response = await api.get<ApiResponse<ChannelTimeline[]>>(
    '/api/channel_monitor/timeline',
    { params: { hours: 24, bucket_minutes: 10, limit: 20, group } }
  )
  return unwrap(response.data)
}

export async function getChannelMonitorRankings(days: number) {
  const response = await api.get<ApiResponse<ChannelRankings>>(
    '/api/channel_monitor/rankings',
    { params: { days, top: 10 } }
  )
  return unwrap(response.data)
}

export async function setChannelScore(channelId: number, score: number | null) {
  const response = await api.post<ApiResponse<null>>(
    `/api/channel_monitor/channels/${channelId}/score_override`,
    { score }
  )
  return unwrap(response.data)
}

export async function clearChannelCircuit(channelId: number) {
  const response = await api.post<ApiResponse<null>>(
    `/api/channel_monitor/channels/${channelId}/clear_circuit`
  )
  return unwrap(response.data)
}

export async function updateChannelPriority(
  channelId: number,
  priority: number
) {
  const response = await api.put<ApiResponse<null>>('/api/channel/', {
    id: channelId,
    priority,
  })
  return unwrap(response.data)
}
