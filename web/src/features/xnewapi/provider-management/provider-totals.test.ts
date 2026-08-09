/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Protect provider-group mapping and profitability totals.

import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { createProviderAmountFormatter } from './provider-amount'
import { getProviderTotals } from './provider-totals'
import type { UpstreamProvider } from './types'

describe('provider profitability totals', () => {
  test('formats amounts without currency symbols or grouping separators', () => {
    const formatter = createProviderAmountFormatter('zhCN')
    assert.equal(formatter.format(1234.5), '1234.50')
  })

  test('counts each group once and supports multiple mapped channels', () => {
    const provider: UpstreamProvider = {
      id: 'provider',
      name: 'Provider',
      endpoint: 'https://example.com',
      status: 'healthy',
      syncStatus: 'success',
      lastSyncedAt: '2026-07-29 12:00',
      authenticationMethod: 'api-key',
      adapterType: 'newapi',
      syncIntervalMinutes: 60,
      rechargeRatio: 4,
      quotaConversionBase: 500000,
      currency: 'USD',
      accounts: [
        {
          id: 'account',
          name: 'Account',
          externalId: 'external',
          loginUsername: '',
          status: 'healthy',
          syncApiKey: '',
          autoSync: true,
          balance: 100,
          totalRecharge: 150,
          lastSyncedAt: '2026-07-29 12:00',
          groups: [
            {
              id: '1',
              name: 'mapped-group',
              groupKey: '1:1:1',
              channels: [
                { id: 1, name: 'Primary', status: 'available' },
                { id: 2, name: 'Secondary', status: 'available' },
              ],
              keys: [
                {
                  id: '1',
                  externalId: '1',
                  name: 'First',
                  maskedKey: 'sk-****1234',
                  status: 'active',
                },
                {
                  id: '2',
                  externalId: '2',
                  name: 'Second',
                  maskedKey: 'sk-****2468',
                  status: 'active',
                },
              ],
              profit: {
                groupKey: '1:1:1',
                groupId: 1,
                groupName: 'mapped-group',
                providerId: 1,
                providerName: 'Provider',
                accountId: 1,
                accountName: 'Account',
                keyIds: [1, 2],
                revenue: 100,
                cost: 60,
                profit: 40,
                grossMargin: 40,
                costStatus: 'ready',
                lastSyncedAt: 0,
              },
            },
            {
              id: '2',
              name: 'unmapped-group',
              groupKey: '1:1:2',
              channels: [],
              keys: [
                {
                  id: '3',
                  externalId: '3',
                  name: 'Third',
                  maskedKey: 'sk-****5678',
                  status: 'active',
                },
              ],
              profit: {
                groupKey: '1:1:2',
                groupId: 2,
                groupName: 'unmapped-group',
                providerId: 1,
                providerName: 'Provider',
                accountId: 1,
                accountName: 'Account',
                keyIds: [3],
                revenue: 0,
                cost: 10,
                profit: -10,
                grossMargin: null,
                costStatus: 'ready',
                lastSyncedAt: 0,
              },
            },
          ],
        },
      ],
    }

    assert.deepEqual(getProviderTotals(provider), {
      accountCount: 1,
      groupCount: 2,
      keyCount: 3,
      mappedGroupCount: 1,
      balance: 100,
      totalRecharge: 150,
      revenue: 100,
      cost: 70,
      profit: 30,
      margin: 30,
    })
  })
})
