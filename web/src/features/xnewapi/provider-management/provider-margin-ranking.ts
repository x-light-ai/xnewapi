/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Aggregate the complete provider profit feed into provider-level rankings.

import type { ProviderGroupProfit } from './types'

export type ProviderMarginRankingItem = {
  providerId: string
  providerName: string
  revenue: number
  cost: number
  profit: number
  margin: number
}

export function buildProviderMarginRanking(
  profits: ProviderGroupProfit[]
): ProviderMarginRankingItem[] {
  const totals = new Map<
    number,
    {
      providerName: string
      revenue: number
      cost: number
      costComplete: boolean
    }
  >()

  for (const item of profits) {
    let total = totals.get(item.providerId)
    if (!total) {
      total = {
        providerName: item.providerName,
        revenue: 0,
        cost: 0,
        costComplete: true,
      }
      totals.set(item.providerId, total)
    }
    total.revenue += item.revenue
    if (item.cost === null) {
      total.costComplete = false
    } else {
      total.cost += item.cost
    }
  }

  const rankings: ProviderMarginRankingItem[] = []
  for (const [providerId, total] of totals) {
    if (!total.costComplete || total.revenue <= 0) continue

    const profit = total.revenue - total.cost
    rankings.push({
      providerId: String(providerId),
      providerName: total.providerName,
      revenue: total.revenue,
      cost: total.cost,
      profit,
      margin: (profit / total.revenue) * 100,
    })
  }

  return rankings.toSorted(
    (left, right) =>
      right.margin - left.margin ||
      right.revenue - left.revenue ||
      left.providerName.localeCompare(right.providerName)
  )
}
