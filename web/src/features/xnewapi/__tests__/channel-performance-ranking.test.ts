/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Protect channel ranking aggregation and profitability sorting.

import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  buildChannelPerformanceRows,
  buildRecommendedChannelGroups,
  buildRecommendedChannelRows,
  sortChannelPerformanceRows,
} from '../channel-performance-ranking-utils'
import type { UpstreamProvider } from '../provider-management/types'
import type { ChannelMonitorItem } from '../types'

function channel(
  id: number,
  name: string,
  requestCount: number,
  failureCount: number,
  groupName: string = 'default'
): ChannelMonitorItem {
  return {
    id,
    name,
    group_name: groupName,
    type: 1,
    status: 1,
    priority: 0,
    success_rate:
      requestCount > 0 ? (requestCount - failureCount) / requestCount : 0,
    avg_latency: 100,
    p95_latency: 150,
    request_count: requestCount,
    failure_count: failureCount,
    last_active: '2026-08-11T06:30:00Z',
    health_trend: [],
    temporary_circuit_open: false,
    temporary_circuit_until: '0001-01-01T00:00:00Z',
    temporary_circuit_reason: '',
    current_weighted_score: 1,
  }
}

describe('channel performance ranking', () => {
  test('aggregates mapped channels once and preserves unmapped channels', () => {
    const providers = [
      {
        id: '1',
        name: 'Provider A',
        endpoint: 'https://provider.example/api',
        currency: 'USD',
        accounts: [
          {
            groups: [
              {
                groupKey: '1:1:1',
                channels: [
                  { id: 1, name: 'alpha', status: 'available' },
                  { id: 2, name: 'beta', status: 'available' },
                ],
                profit: { grossMargin: 25, revenue: 120 },
              },
            ],
          },
        ],
      },
    ] as UpstreamProvider[]

    const rows = buildChannelPerformanceRows(
      [
        channel(1, 'alpha', 80, 4),
        channel(2, 'beta', 20, 1),
        channel(3, 'unmapped', 10, 2),
      ],
      providers
    )

    assert.equal(rows.length, 2)
    assert.deepEqual(rows[0].channelNames, ['alpha', 'beta'])
    assert.deepEqual(rows[0].groupNames, ['default'])
    assert.equal(rows[0].requestCount, 100)
    assert.equal(rows[0].failureCount, 5)
    assert.equal(rows[0].successRate, 0.95)
    assert.equal(rows[0].grossMargin, 25)
    assert.equal(rows[0].revenue, 120)
    assert.equal(rows[0].providerEndpoint, 'https://provider.example/api')
    assert.deepEqual(rows[1].channelNames, ['unmapped'])
    assert.equal(rows[1].revenue, null)
  })

  test('weights margin and stability equally while penalizing tiny samples', () => {
    const rows = buildChannelPerformanceRows(
      [
        channel(1, 'balanced', 500, 5),
        channel(2, 'cheap-but-unstable', 500, 100),
        channel(3, 'tiny-sample', 1, 0),
        channel(4, 'missing-margin', 500, 0),
      ],
      []
    )
    rows[0].grossMargin = 30
    rows[1].grossMargin = 50
    rows[2].grossMargin = 50

    const recommended = buildRecommendedChannelRows(rows)

    assert.deepEqual(
      recommended.map((row) => row.channelNames[0]),
      ['cheap-but-unstable', 'balanced', 'tiny-sample']
    )
    assert.ok(
      recommended[0].recommendationScore > recommended[1].recommendationScore
    )
    assert.ok(
      recommended[1].recommendationScore > recommended[2].recommendationScore
    )
  })

  test('does not use latency in the recommendation score', () => {
    const fast = channel(1, 'fast', 500, 5)
    const slow = channel(2, 'slow', 500, 5)
    fast.avg_latency = 50
    slow.avg_latency = 5000
    const rows = buildChannelPerformanceRows([fast, slow], [])
    rows[0].grossMargin = 30
    rows[1].grossMargin = 30

    const recommended = buildRecommendedChannelRows(rows)

    assert.equal(
      recommended[0].recommendationScore,
      recommended[1].recommendationScore
    )
  })

  test('ranks the top three channels independently within each channel group', () => {
    const rows = buildChannelPerformanceRows(
      [
        channel(1, 'default-first', 500, 5, 'default'),
        channel(2, 'default-second', 500, 10, 'default'),
        channel(3, 'default-third', 500, 15, 'default'),
        channel(4, 'default-fourth', 500, 20, 'default'),
        channel(5, 'vip-first', 500, 0, 'vip'),
        channel(6, 'shared', 500, 5, 'default, vip'),
      ],
      []
    )
    for (const row of rows) row.grossMargin = 30

    const groups = buildRecommendedChannelGroups(rows)

    assert.deepEqual(
      groups.map((group) => group.groupName),
      ['default', 'vip']
    )
    assert.deepEqual(
      groups[0].rows.map((row) => row.channelNames[0]),
      ['default-first', 'shared', 'default-second']
    )
    assert.deepEqual(
      groups[1].rows.map((row) => row.channelNames[0]),
      ['vip-first', 'shared']
    )
  })

  test('keeps missing profitability values last in either sort direction', () => {
    const rows = buildChannelPerformanceRows(
      [channel(1, 'known', 10, 0), channel(2, 'unknown', 20, 0)],
      []
    )
    rows[0].grossMargin = 30

    assert.deepEqual(
      sortChannelPerformanceRows(rows, 'gross_margin', 'desc').map(
        (row) => row.channelNames[0]
      ),
      ['known', 'unknown']
    )
    assert.deepEqual(
      sortChannelPerformanceRows(rows, 'gross_margin', 'asc').map(
        (row) => row.channelNames[0]
      ),
      ['known', 'unknown']
    )
  })
})
