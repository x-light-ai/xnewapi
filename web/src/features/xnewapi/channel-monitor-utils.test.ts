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
  formatBeijingDateTime,
  getTrendPointSize,
  sortChannelMonitorItems,
} from './channel-monitor-utils'
import type { ChannelMonitorItem } from './types'

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
