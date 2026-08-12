/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Protect classic channel-monitor grouping and routing-score behavior.

import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  buildChannelMonitorRows,
  compressTimelinePoints,
  formatBeijingDateTime,
  getTrendPointSize,
  sortChannelMonitorItems,
} from './channel-monitor-utils'
import type { ChannelMonitorItem, ChannelTimelinePoint } from './types'

function channel(
  values: Partial<ChannelMonitorItem> & Pick<ChannelMonitorItem, 'id' | 'name'>
): ChannelMonitorItem {
  const { id, name, ...overrides } = values
  return {
    id,
    name,
    group_name: 'default',
    type: 1,
    status: 1,
    priority: 0,
    success_rate: 1,
    avg_latency: 100,
    p95_latency: 150,
    request_count: 10,
    failure_count: 0,
    last_active: '2026-07-15T08:00:00Z',
    health_trend: [],
    temporary_circuit_open: false,
    temporary_circuit_until: '0001-01-01T00:00:00Z',
    temporary_circuit_reason: '',
    current_weighted_score: 1,
    ...overrides,
  }
}

describe('channel monitor table helpers', () => {
  test('sorts higher-priority channels first by default', () => {
    const items = [
      channel({ id: 1, name: 'fallback', priority: 0 }),
      channel({ id: 2, name: 'primary', priority: 10 }),
      channel({ id: 3, name: 'secondary', priority: 5 }),
    ]

    assert.deepEqual(
      sortChannelMonitorItems(items, 'priority', 'desc').map(
        (item) => item.name
      ),
      ['primary', 'secondary', 'fallback']
    )
  })

  test('sorts stability from highest success rate to lowest', () => {
    const items = [
      channel({ id: 1, name: 'unstable', success_rate: 0.5 }),
      channel({ id: 2, name: 'stable', success_rate: 0.99 }),
    ]

    assert.deepEqual(
      sortChannelMonitorItems(items, 'success_rate', 'desc').map(
        (item) => item.name
      ),
      ['stable', 'unstable']
    )
  })

  test('sorts routing score from highest to lowest', () => {
    const items = [
      channel({ id: 1, name: 'low-score', current_weighted_score: 0.4 }),
      channel({ id: 2, name: 'high-score', current_weighted_score: 0.9 }),
    ]

    assert.deepEqual(
      sortChannelMonitorItems(items, 'routing_score', 'desc').map(
        (item) => item.name
      ),
      ['high-score', 'low-score']
    )
  })

  test('builds weighted group summaries and ranks groups by success rate', () => {
    const rows = buildChannelMonitorRows(
      [
        channel({
          id: 1,
          name: 'alpha-fast',
          group_name: 'alpha',
          request_count: 10,
          failure_count: 1,
          success_rate: 0.9,
          avg_latency: 100,
        }),
        channel({
          id: 2,
          name: 'alpha-slow',
          group_name: 'alpha',
          request_count: 30,
          failure_count: 3,
          success_rate: 0.9,
          avg_latency: 300,
        }),
        channel({
          id: 3,
          name: 'beta',
          group_name: 'beta',
          success_rate: 1,
        }),
      ],
      'group_success_rate',
      'desc',
      true
    )

    assert.equal(rows[0].kind, 'group')
    if (rows[0].kind !== 'group') return
    assert.equal(rows[0].group_name, 'beta')

    const alphaSummary = rows.find(
      (row) => row.kind === 'group' && row.group_name === 'alpha'
    )
    assert.ok(alphaSummary && alphaSummary.kind === 'group')
    assert.equal(alphaSummary.request_count, 40)
    assert.equal(alphaSummary.failure_count, 4)
    assert.equal(alphaSummary.success_rate, 0.9)
    assert.equal(alphaSummary.avg_latency, 250)
  })

  test('sorts channels by gross margin while keeping missing values last', () => {
    const items = [
      channel({ id: 1, name: 'unknown', gross_margin: null }),
      channel({ id: 2, name: 'lower', gross_margin: 12.5 }),
      channel({ id: 3, name: 'higher', gross_margin: 35.2 }),
    ]

    assert.deepEqual(
      sortChannelMonitorItems(items, 'gross_margin', 'desc').map(
        (item) => item.name
      ),
      ['higher', 'lower', 'unknown']
    )
  })

  test('formats ISO timestamps in Beijing time without timezone suffixes', () => {
    assert.equal(
      formatBeijingDateTime('2026-08-11T06:30:00Z'),
      '2026-08-11 14:30:00'
    )
    assert.equal(formatBeijingDateTime('0001-01-01T00:00:00Z'), '-')
  })

  test('scales trend points with request volume and caps their size', () => {
    assert.equal(getTrendPointSize(0), 4)
    assert.equal(getTrendPointSize(4), 6)
    assert.equal(getTrendPointSize(16), 8)
    assert.equal(getTrendPointSize(64), 12)
    assert.equal(getTrendPointSize(10000), 12)
  })
})
describe('availability trend compression', () => {
  function timelinePoint(
    values: Partial<ChannelTimelinePoint>
  ): ChannelTimelinePoint {
    return {
      channel_id: 1,
      channel_name: 'alpha',
      channel_type: 1,
      channel_status: 1,
      time_bucket: '2026-08-11T12:00:00Z',
      request_count: 0,
      success_count: 0,
      failure_count: 0,
      failure_reasons: [],
      ...values,
    }
  }

  test('keeps timeline points unchanged when within the cap', () => {
    const points = [
      timelinePoint({ time_bucket: '2026-08-11T12:00:00Z' }),
      timelinePoint({ time_bucket: '2026-08-11T12:10:00Z' }),
    ]
    assert.equal(compressTimelinePoints(points, 5), points)
  })

  test('compresses 144 points to 36 even groups of 4 buckets', () => {
    const points = Array.from({ length: 144 }, (_, index) =>
      timelinePoint({
        time_bucket: new Date(
          Date.UTC(2026, 7, 11, 12) + index * 600000
        ).toISOString(),
        request_count: 10,
        success_count: 8,
        failure_count: 2,
      })
    )

    const compressed = compressTimelinePoints(points)

    assert.equal(compressed.length, 36)
    assert.equal(compressed[0].time_bucket, points[0].time_bucket)
    assert.equal(compressed[0].request_count, 40)
    assert.equal(compressed[0].success_count, 32)
    assert.equal(compressed[0].failure_count, 8)
    assert.equal(compressed.at(-1)?.time_bucket, points[140].time_bucket)
    assert.equal(
      compressed[0].time_bucket_end,
      new Date(Date.UTC(2026, 7, 11, 12) + 4 * 600000).toISOString()
    )
  })

  test('keeps empty time divisions so visual spacing remains proportional', () => {
    const firstHalf = Array.from({ length: 21 }, (_, index) =>
      timelinePoint({
        time_bucket: new Date(
          Date.UTC(2026, 7, 11, 12) + index * 600000
        ).toISOString(),
        request_count: 1,
        success_count: 1,
      })
    )
    const secondHalf = Array.from({ length: 20 }, (_, index) =>
      timelinePoint({
        time_bucket: new Date(
          Date.UTC(2026, 7, 11, 22) + index * 600000
        ).toISOString(),
        request_count: 1,
        success_count: 1,
      })
    )

    const compressed = compressTimelinePoints([...firstHalf, ...secondHalf], 4)

    assert.equal(compressed.length, 4)
    assert.equal(compressed[2].request_count, 0)
    assert.equal(compressed[2].failure_reasons.length, 0)
  })

  test('merges failure reasons and keeps details across compressed buckets', () => {
    const points = [
      timelinePoint({
        time_bucket: '2026-08-11T12:00:00Z',
        failure_count: 3,
        failure_reasons: [
          {
            reason: 'rate_limit',
            error_code: 'rate_limit',
            error_type: 'upstream_error',
            status_code: 429,
            count: 2,
            details: [
              {
                occurred_at: '2026-08-11T12:00:10Z',
                message: 'too many requests',
                request_id: 'req-1',
              },
            ],
          },
        ],
      }),
      timelinePoint({
        time_bucket: '2026-08-11T12:10:00Z',
        failure_count: 4,
        failure_reasons: [
          {
            reason: 'rate_limit',
            error_code: 'rate_limit',
            error_type: 'upstream_error',
            status_code: 429,
            count: 3,
            details: [
              {
                occurred_at: '2026-08-11T12:10:10Z',
                message: 'retry after 30s',
                request_id: 'req-2',
              },
            ],
          },
          {
            reason: 'request_timeout',
            error_code: 'request_timeout',
            error_type: 'timeout_error',
            status_code: 504,
            count: 1,
            details: [],
          },
        ],
      }),
    ]

    const padding = Array.from({ length: 39 }, (_, index) =>
      timelinePoint({
        time_bucket: new Date(
          Date.UTC(2026, 7, 11, 12, 20) + index * 600000
        ).toISOString(),
      })
    )
    const compressed = compressTimelinePoints([...points, ...padding], 1)

    assert.equal(compressed.length, 1)
    assert.equal(compressed[0].failure_count, 7)
    assert.equal(compressed[0].failure_reasons.length, 2)
    assert.equal(compressed[0].failure_reasons[0].reason, 'rate_limit')
    assert.equal(compressed[0].failure_reasons[0].count, 5)
    assert.equal(compressed[0].failure_reasons[0].details.length, 2)
    assert.equal(compressed[0].failure_reasons[1].reason, 'request_timeout')
    assert.equal(compressed[0].failure_reasons[1].count, 1)
  })
})
