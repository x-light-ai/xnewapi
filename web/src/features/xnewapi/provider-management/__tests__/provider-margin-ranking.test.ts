/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Protect provider-level aggregation across the complete profit feed.

import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildProviderMarginRanking } from '../provider-margin-ranking'
import type { ProviderGroupProfit } from '../types'

function profit(
  providerId: number,
  providerName: string,
  groupId: number,
  revenue: number,
  cost: number | null
): ProviderGroupProfit {
  const grossProfit = cost === null ? null : revenue - cost
  return {
    groupKey: `${providerId}:1:${groupId}`,
    groupId,
    groupName: `Group ${groupId}`,
    providerId,
    providerName,
    accountId: 1,
    accountName: 'Account',
    keyIds: [],
    revenue,
    cost,
    profit: grossProfit,
    grossMargin:
      grossProfit !== null && revenue > 0
        ? (grossProfit / revenue) * 100
        : null,
    costStatus: cost === null ? 'pending' : 'ready',
    lastSyncedAt: 0,
  }
}

describe('provider gross margin ranking', () => {
  test('combines all groups and sorts every provider by aggregate margin', () => {
    const rankings = buildProviderMarginRanking([
      profit(1, 'Provider A', 1, 100, 10),
      profit(1, 'Provider A', 2, 100, 90),
      profit(2, 'Provider B', 3, 100, 40),
      profit(3, 'Provider C', 4, 200, 80),
    ])

    assert.deepEqual(
      rankings.map((item) => ({
        provider: item.providerName,
        revenue: item.revenue,
        cost: item.cost,
        margin: item.margin,
      })),
      [
        { provider: 'Provider C', revenue: 200, cost: 80, margin: 60 },
        { provider: 'Provider B', revenue: 100, cost: 40, margin: 60 },
        { provider: 'Provider A', revenue: 200, cost: 100, margin: 50 },
      ]
    )
  })

  test('omits providers whose aggregate cost is incomplete', () => {
    const rankings = buildProviderMarginRanking([
      profit(1, 'Complete', 1, 100, 50),
      profit(2, 'Incomplete', 2, 100, 20),
      profit(2, 'Incomplete', 3, 50, null),
    ])

    assert.deepEqual(
      rankings.map((item) => item.providerName),
      ['Complete']
    )
  })
})
