/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Expose daily provider profitability through a shareable admin route.

import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import z from 'zod'

import { ProviderProfitDetails } from '@/features/xnewapi/provider-management/provider-profit-details'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

const providerProfitDetailsSearchSchema = z.object({
  startDate: z.string().optional().catch(undefined),
  endDate: z.string().optional().catch(undefined),
  providerId: z.string().optional().catch(undefined),
  groupId: z.string().optional().catch(undefined),
})

export const Route = createFileRoute(
  '/_authenticated/providers/profit-details'
)({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  validateSearch: providerProfitDetailsSearchSchema,
  component: ProviderProfitDetailsRoute,
})

function ProviderProfitDetailsRoute() {
  const search = Route.useSearch()
  const navigate = useNavigate()

  return (
    <ProviderProfitDetails
      search={search}
      onSearchChange={(next) =>
        navigate({ to: '/providers/profit-details', search: next })
      }
    />
  )
}
