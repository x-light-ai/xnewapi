import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getEffectiveProviderSyncStatus } from '../provider-sync-status'

describe('provider sync status freshness', () => {
  test('marks a successful sync as overdue after its interval', () => {
    const syncedAt = 1_000
    const intervalMinutes = 60

    assert.equal(
      getEffectiveProviderSyncStatus(
        'success',
        syncedAt,
        intervalMinutes,
        (syncedAt + intervalMinutes * 60) * 1000
      ),
      'stale'
    )
  })

  test('keeps errors and active syncs unchanged', () => {
    assert.equal(
      getEffectiveProviderSyncStatus(
        'error',
        1_000,
        1,
        Number.MAX_SAFE_INTEGER
      ),
      'error'
    )
    assert.equal(
      getEffectiveProviderSyncStatus(
        'syncing',
        1_000,
        1,
        Number.MAX_SAFE_INTEGER
      ),
      'syncing'
    )
  })
})
