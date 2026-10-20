/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Keep provider workspace queries aligned with the selected profit range.

import { getProviderWorkspace } from './api'
import type { ProviderProfitDateRange } from './types'

export function createProviderWorkspaceQuery(
  dateRange: ProviderProfitDateRange
) {
  return {
    queryKey: [
      'upstream-providers',
      dateRange.startDate,
      dateRange.endDate,
    ] as const,
    queryFn: () => getProviderWorkspace(dateRange),
  }
}
