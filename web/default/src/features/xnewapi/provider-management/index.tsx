/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Host provider management over fork-owned APIs in an isolated feature module.

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import {
  Building2,
  ChartNoAxesCombined,
  Plus,
  RefreshCw,
  Search,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import {
  adjustProviderAccountRecharge,
  deleteProvider,
  deleteProviderAccount,
  deleteProviderGroup,
  getProviderWorkspace,
  saveProvider,
  syncProvider,
  syncProviderAccount,
} from './api'
import { ChannelMarginRanking } from './channel-margin-ranking'
import { createProviderAmountFormatter } from './provider-amount'
import { ProviderDetailSheet } from './provider-detail-sheet'
import { ProviderMetrics } from './provider-metrics'
import { ProviderTable } from './provider-table'
import type { UpstreamProvider, UpstreamProviderStatus } from './types'

type StatusFilter = UpstreamProviderStatus | 'all'
const EMPTY_PROVIDERS: UpstreamProvider[] = []

export function ProviderManagement() {
  const { t, i18n } = useTranslation('xnewapi')
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const workspaceQuery = useQuery({
    queryKey: ['upstream-providers'],
    queryFn: getProviderWorkspace,
  })
  const providers = workspaceQuery.data?.providers ?? EMPTY_PROVIDERS
  const [query, setQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [expandedIds, setExpandedIds] = useState<Set<string>>(
    () => new Set<string>()
  )
  const [sheetOpen, setSheetOpen] = useState(false)
  const [selectedProvider, setSelectedProvider] =
    useState<UpstreamProvider | null>(null)

  const visibleProviders = useMemo(() => {
    const keyword = query.trim().toLowerCase()
    return providers.filter((provider) => {
      if (statusFilter !== 'all' && provider.status !== statusFilter) {
        return false
      }
      if (!keyword) return true

      const searchable = [
        provider.name,
        provider.endpoint,
        ...provider.accounts.flatMap((account) => [
          account.name,
          account.externalId,
          ...account.groups.flatMap((group) => [
            group.name,
            ...group.channels.map((channel) => channel.name),
            ...group.keys.flatMap((key) => [key.name, key.maskedKey]),
          ]),
        ]),
      ]
      return searchable.some((value) => value.toLowerCase().includes(keyword))
    })
  }, [providers, query, statusFilter])

  const amountFormatter = useMemo(
    () => createProviderAmountFormatter(i18n.resolvedLanguage || i18n.language),
    [i18n.language, i18n.resolvedLanguage]
  )

  const openCreateSheet = () => {
    setSelectedProvider(null)
    setSheetOpen(true)
  }

  const openEditSheet = (provider: UpstreamProvider) => {
    setSelectedProvider(provider)
    setSheetOpen(true)
  }

  const saveMutation = useMutation({
    mutationFn: saveProvider,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['upstream-providers'] })
      setSheetOpen(false)
      toast.success(t('Model provider saved'))
    },
    onError: (error) => toast.error(error.message),
  })
  const syncMutation = useMutation({
    mutationFn: (provider: UpstreamProvider) => syncProvider(provider.id),
    onSuccess: () => {
      toast.success(t('Synchronization completed'))
    },
    onError: (error) => toast.error(error.message),
    onSettled: async () => {
      await queryClient.invalidateQueries({ queryKey: ['upstream-providers'] })
    },
  })
  const syncAccountMutation = useMutation({
    mutationFn: syncProviderAccount,
    onSuccess: () => {
      toast.success(t('Synchronization completed'))
    },
    onError: (error) => toast.error(error.message),
    onSettled: async () => {
      await queryClient.invalidateQueries({ queryKey: ['upstream-providers'] })
    },
  })
  const adjustRechargeMutation = useMutation({
    mutationFn: ({ accountId, delta }: { accountId: string; delta: string }) =>
      adjustProviderAccountRecharge(accountId, delta),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['upstream-providers'] })
      toast.success(t('Recharged amount adjusted'))
    },
    onError: (error) => toast.error(error.message),
  })
  const deleteProviderMutation = useMutation({
    mutationFn: deleteProvider,
    onSuccess: () => {
      setSheetOpen(false)
      toast.success(t('Deleted successfully'))
    },
    onError: (error) => toast.error(error.message),
    onSettled: async () => {
      await queryClient.invalidateQueries({ queryKey: ['upstream-providers'] })
    },
  })
  const deleteAccountMutation = useMutation({
    mutationFn: deleteProviderAccount,
    onSuccess: () => {
      setSheetOpen(false)
      toast.success(t('Deleted successfully'))
    },
    onError: (error) => toast.error(error.message),
    onSettled: async () => {
      await queryClient.invalidateQueries({ queryKey: ['upstream-providers'] })
    },
  })
  const deleteGroupMutation = useMutation({
    mutationFn: deleteProviderGroup,
    onSuccess: () => {
      setSheetOpen(false)
      toast.success(t('Deleted successfully'))
    },
    onError: (error) => toast.error(error.message),
    onSettled: async () => {
      await queryClient.invalidateQueries({ queryKey: ['upstream-providers'] })
    },
  })

  const toggleProvider = (providerId: string) => {
    setExpandedIds((current) => {
      const next = new Set(current)
      if (next.has(providerId)) next.delete(providerId)
      else next.add(providerId)
      return next
    })
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          <span className='flex min-w-0 items-center gap-2'>
            <Building2 className='text-primary size-5 shrink-0' />
            <span className='truncate'>{t('Model Provider Management')}</span>
          </span>
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='icon'
              onClick={() => workspaceQuery.refetch()}
              aria-label={t('Refresh')}
              title={t('Refresh')}
            >
              <RefreshCw
                className={workspaceQuery.isFetching ? 'animate-spin' : ''}
              />
            </Button>
            <Button
              variant='outline'
              onClick={() => navigate({ to: '/providers/profit-details' })}
            >
              <ChartNoAxesCombined data-icon='inline-start' />
              {t('Profit details')}
            </Button>
            <Button onClick={openCreateSheet}>
              <Plus data-icon='inline-start' />
              {t('Add model provider')}
            </Button>
          </div>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex flex-col gap-4'>
            {workspaceQuery.error && (
              <div className='border-destructive/40 bg-destructive/5 text-destructive rounded-lg border px-4 py-3 text-sm'>
                {workspaceQuery.error.message}
              </div>
            )}
            <ProviderMetrics
              providers={providers}
              formatAmount={(value) => amountFormatter.format(value)}
            />

            <ChannelMarginRanking
              providers={providers}
              formatAmount={(value) => amountFormatter.format(value)}
              onViewProfitDetails={(filter) =>
                navigate({
                  to: '/providers/profit-details',
                  search: filter,
                })
              }
            />

            <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
              <InputGroup className='w-full sm:max-w-md'>
                <InputGroupAddon>
                  <Search aria-hidden />
                </InputGroupAddon>
                <InputGroupInput
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={t(
                    'Search model providers, accounts, keys, or channels...'
                  )}
                  aria-label={t('Search model providers')}
                />
              </InputGroup>
              <NativeSelect
                className='w-full sm:w-44'
                value={statusFilter}
                onChange={(event) =>
                  setStatusFilter(event.target.value as StatusFilter)
                }
                aria-label={t('Model provider status')}
              >
                <NativeSelectOption value='all'>
                  {t('All statuses')}
                </NativeSelectOption>
                <NativeSelectOption value='healthy'>
                  {t('Healthy')}
                </NativeSelectOption>
                <NativeSelectOption value='disabled'>
                  {t('Disabled')}
                </NativeSelectOption>
              </NativeSelect>
            </div>

            <ProviderTable
              providers={visibleProviders}
              expandedIds={expandedIds}
              formatAmount={(value) => amountFormatter.format(value)}
              onToggle={toggleProvider}
              onEdit={openEditSheet}
            />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {sheetOpen && (
        <ProviderDetailSheet
          key={selectedProvider?.id ?? 'new-provider'}
          open
          provider={selectedProvider}
          channels={workspaceQuery.data?.channelOptions ?? []}
          saving={saveMutation.isPending}
          formatAmount={(value) => amountFormatter.format(value)}
          onOpenChange={setSheetOpen}
          onSave={(provider) => saveMutation.mutate(provider)}
          onSync={(provider) => syncMutation.mutate(provider)}
          onSyncAccount={(account) => syncAccountMutation.mutate(account.id)}
          onAdjustRecharge={(accountId, delta) =>
            adjustRechargeMutation.mutateAsync({ accountId, delta })
          }
          onDelete={(provider) =>
            deleteProviderMutation.mutateAsync(provider.id)
          }
          onDeleteAccount={(accountId) =>
            deleteAccountMutation.mutateAsync(accountId)
          }
          onDeleteGroup={(groupId) => deleteGroupMutation.mutateAsync(groupId)}
          syncing={syncMutation.isPending || syncAccountMutation.isPending}
          adjustingRecharge={adjustRechargeMutation.isPending}
          deleting={
            deleteProviderMutation.isPending ||
            deleteAccountMutation.isPending ||
            deleteGroupMutation.isPending
          }
        />
      )}
    </>
  )
}
