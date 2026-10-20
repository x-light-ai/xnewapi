/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Verify provider workspace profit queries preserve the selected date range.

import { describe, expect, test, vi } from 'vitest'

import { getProviderWorkspace } from '../api'
import { createProviderWorkspaceQuery } from '../provider-workspace-query'

vi.mock('../api', () => ({
  getProviderWorkspace: vi.fn(),
}))

describe('provider workspace query', () => {
  test('keeps the selected profit date range in the query and request', async () => {
    const getProviderWorkspaceMock = vi.mocked(getProviderWorkspace)
    const dateRange = { startDate: '2026-10-19', endDate: '2026-10-19' }
    const query = createProviderWorkspaceQuery(dateRange)

    expect(query.queryKey).toEqual([
      'upstream-providers',
      '2026-10-19',
      '2026-10-19',
    ])
    await query.queryFn()
    expect(getProviderWorkspaceMock).toHaveBeenCalledWith(dateRange)
  })
})
