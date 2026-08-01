/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Aggregate provider API data without owning fixtures or transport state.

import type { ProviderTotals, UpstreamProvider } from './types'

export function getProviderTotals(provider: UpstreamProvider): ProviderTotals {
  let keyCount = 0
  let groupCount = 0
  let mappedGroupCount = 0
  let balance = 0
  let totalRecharge = 0
  let revenue = 0
  let cost = 0
  let costComplete = true

  for (const account of provider.accounts) {
    balance += account.balance
    totalRecharge += account.totalRecharge
    for (const group of account.groups) {
      groupCount += 1
      keyCount += group.keys.length
      if (group.channels.length > 0) mappedGroupCount += 1
      const groupProfit = group.profit
      if (!groupProfit) {
        costComplete = false
        continue
      }
      revenue += groupProfit.revenue
      if (groupProfit.cost === null) {
        costComplete = false
      } else {
        cost += groupProfit.cost
      }
    }
  }

  const profit = costComplete ? revenue - cost : null
  return {
    accountCount: provider.accounts.length,
    groupCount,
    keyCount,
    mappedGroupCount,
    balance,
    totalRecharge,
    revenue,
    cost: costComplete ? cost : null,
    profit,
    margin: profit !== null && revenue > 0 ? (profit / revenue) * 100 : null,
  }
}
