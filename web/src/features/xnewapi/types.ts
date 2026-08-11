/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Define default-frontend contracts for fork-owned channel monitoring.

export type ApiResponse<T> = {
  success: boolean
  message?: string
  data: T
}

export type ChannelMonitorSummary = {
  total_requests: number
  success_rate: number
  avg_latency: number
  active_channels: number
  total_channels: number
  time_range: string
}

export type ChannelHealthPoint = {
  date: string
  total_requests: number
  success_rate: number
  avg_latency: number
  healthy_channels: number
  warning_channels: number
  error_channels: number
}

export type ChannelMonitorItem = {
  id: number
  name: string
  group_name: string
  type: number
  status: number
  priority: number
  success_rate: number
  avg_latency: number
  p95_latency: number
  request_count: number
  failure_count: number
  last_active: string
  health_trend: number[]
  temporary_circuit_open: boolean
  temporary_circuit_until: string
  temporary_circuit_reason: string
  current_weighted_score: number
  gross_margin?: number | null
}

export type ChannelTimelinePoint = {
  channel_id: number
  channel_name: string
  channel_type: number
  channel_status: number
  time_bucket: string
  request_count: number
  success_count: number
  failure_count: number
  failure_reasons: ChannelTimelineFailureReason[]
}

export type ChannelTimelineFailureReason = {
  reason: string
  error_code: string
  error_type: string
  status_code: number
  count: number
  details: ChannelTimelineFailureDetail[]
}

export type ChannelTimelineFailureDetail = {
  occurred_at: string
  message: string
  request_id: string
}

export type ChannelTimeline = {
  channel_id: number
  channel_name: string
  channel_type: number
  channel_status: number
  points: ChannelTimelinePoint[]
  request_count: number
  success_count: number
  failure_count: number
}

export type ChannelMonitorPageData = {
  items: ChannelMonitorItem[]
  total: number
  page: number
  page_size: number
  groups: string[]
}

export type ChannelRankings = {
  stability: ChannelMonitorItem[]
  latency: ChannelMonitorItem[]
}
