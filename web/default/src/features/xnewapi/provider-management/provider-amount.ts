/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Keep provider amounts symbol-free and ungrouped across the management UI.

import { toIntlLocale } from '@/i18n/languages'

export function createProviderAmountFormatter(
  language?: string | null
): Intl.NumberFormat {
  return new Intl.NumberFormat(toIntlLocale(language), {
    useGrouping: false,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}
