/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Browse daily profitability with shareable provider and group filters.

import { useQuery } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { ChartNoAxesCombined } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { getProviderProfitDetails, getProviderWorkspace } from './api'
import { createProviderAmountFormatter } from './provider-amount'
import type {
  ProviderChannel,
  ProviderProfitDailyDetail,
  ProviderProfitDetailsSearch,
  UpstreamProvider,
} from './types'

const EMPTY_PROVIDERS: UpstreamProvider[] = []
const EMPTY_PROFIT_DETAILS: ProviderProfitDailyDetail[] = []

type ProviderProfitDetailsProps = {
  search: ProviderProfitDetailsSearch
  onSearchChange: (search: ProviderProfitDetailsSearch) => void
}

export function ProviderProfitDetails(props: ProviderProfitDetailsProps) {
  const { t, i18n } = useTranslation('xnewapi')
  const startDate =
    props.search.startDate ?? dayjs().startOf('month').format('YYYY-MM-DD')
  const endDate = props.search.endDate ?? dayjs().format('YYYY-MM-DD')
  const providerId = props.search.providerId ?? ''
  const groupId = props.search.groupId ?? ''
  const page = props.search.page ?? 1
  const pageSize = props.search.pageSize ?? 20
  const dateRangeIsValid = startDate <= endDate
  const workspaceQuery = useQuery({
    queryKey: ['upstream-providers'],
    queryFn: getProviderWorkspace,
  })
  const providers = workspaceQuery.data?.providers ?? EMPTY_PROVIDERS
  const groups = useMemo(() => {
    if (!providerId) return []
    const provider = providers.find((item) => item.id === providerId)
    return provider?.accounts.flatMap((account) => account.groups) ?? []
  }, [providerId, providers])
  const groupChannelsByID = useMemo(() => {
    const channelsByID = new Map<string, ProviderChannel[]>()
    for (const provider of providers) {
      for (const account of provider.accounts) {
        for (const group of account.groups) {
          channelsByID.set(group.id, group.channels)
        }
      }
    }
    return channelsByID
  }, [providers])
  const providerEndpointByID = useMemo(
    () =>
      new Map(providers.map((provider) => [provider.id, provider.endpoint])),
    [providers]
  )
  const detailsQuery = useQuery({
    queryKey: [
      'upstream-provider-profit-details',
      startDate,
      endDate,
      providerId,
      groupId,
    ],
    queryFn: () =>
      getProviderProfitDetails({
        startDate,
        endDate,
        providerId: providerId || undefined,
        groupId: groupId || undefined,
        page,
        pageSize,
      }),
    enabled: dateRangeIsValid,
  })
  const detailsPage = detailsQuery.data
  const details = detailsPage?.items ?? EMPTY_PROFIT_DETAILS
  const amountFormatter = useMemo(
    () => createProviderAmountFormatter(i18n.resolvedLanguage || i18n.language),
    [i18n.language, i18n.resolvedLanguage]
  )
  const totals = useMemo(
    () => ({
      revenue: detailsPage?.revenue ?? 0,
      cost: detailsPage?.cost ?? null,
      profit: detailsPage?.profit ?? null,
      margin: detailsPage?.grossMargin ?? null,
    }),
    [detailsPage]
  )

  const updateSearch = (
    changes: Partial<ProviderProfitDetailsSearch>,
    resetPage = true
  ) => {
    props.onSearchChange({
      ...props.search,
      ...changes,
      ...(resetPage ? { page: 1 } : {}),
    })
  }
  const total = detailsPage?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const displayFrom = total === 0 ? 0 : (page - 1) * pageSize + 1
  const displayTo = Math.min(page * pageSize, total)
  const setPage = (nextPage: number) => {
    if (nextPage < 1 || nextPage > totalPages || nextPage === page) return
    updateSearch({ page: nextPage }, false)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='flex min-w-0 items-center gap-2'>
          <ChartNoAxesCombined className='text-primary size-5 shrink-0' />
          <span className='truncate'>{t('Profit details')}</span>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-4'>
            <label className='flex flex-col gap-1.5 text-sm font-medium'>
              {t('Start date')}
              <Input
                type='date'
                value={startDate}
                max={endDate}
                onChange={(event) =>
                  updateSearch({ startDate: event.target.value || undefined })
                }
              />
            </label>
            <label className='flex flex-col gap-1.5 text-sm font-medium'>
              {t('End date')}
              <Input
                type='date'
                value={endDate}
                min={startDate}
                max={dayjs().format('YYYY-MM-DD')}
                onChange={(event) =>
                  updateSearch({ endDate: event.target.value || undefined })
                }
              />
            </label>
            <label className='flex flex-col gap-1.5 text-sm font-medium'>
              {t('Model provider')}
              <NativeSelect
                value={providerId}
                onChange={(event) =>
                  updateSearch({
                    providerId: event.target.value || undefined,
                    groupId: undefined,
                  })
                }
              >
                <NativeSelectOption value=''>
                  {t('All model providers')}
                </NativeSelectOption>
                {providers.map((provider) => (
                  <NativeSelectOption key={provider.id} value={provider.id}>
                    {provider.name}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </label>
            <label className='flex flex-col gap-1.5 text-sm font-medium'>
              {t('Provider group')}
              <NativeSelect
                value={groupId}
                disabled={!providerId}
                onChange={(event) =>
                  updateSearch({ groupId: event.target.value || undefined })
                }
              >
                <NativeSelectOption value=''>
                  {t('All provider groups')}
                </NativeSelectOption>
                {groups.map((group) => (
                  <NativeSelectOption key={group.id} value={group.id}>
                    {group.name}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </label>
          </div>

          <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
            <div className='border px-3 py-2'>
              <div className='text-muted-foreground text-xs'>
                {t('Total revenue')}
              </div>
              <div className='mt-1 font-medium tabular-nums'>
                {amountFormatter.format(totals.revenue)}
              </div>
            </div>
            <div className='border px-3 py-2'>
              <div className='text-muted-foreground text-xs'>
                {t('Total cost')}
              </div>
              <div className='mt-1 font-medium tabular-nums'>
                {totals.cost === null
                  ? '-'
                  : amountFormatter.format(totals.cost)}
              </div>
            </div>
            <div className='border px-3 py-2'>
              <div className='text-muted-foreground text-xs'>
                {t('Total profit')}
              </div>
              <div className='mt-1 font-medium tabular-nums'>
                {totals.profit === null
                  ? '-'
                  : amountFormatter.format(totals.profit)}
              </div>
            </div>
            <div className='border px-3 py-2'>
              <div className='text-muted-foreground text-xs'>
                {t('Gross margin')}
              </div>
              <div className='mt-1 font-medium tabular-nums'>
                {totals.margin === null ? '-' : `${totals.margin.toFixed(1)}%`}
              </div>
            </div>
          </div>

          <div className='overflow-x-auto border'>
            <Table className='min-w-[1080px]'>
              <TableHeader className='bg-muted/40'>
                <TableRow>
                  <TableHead className='pl-4'>{t('Date')}</TableHead>
                  <TableHead>{t('Provider and group')}</TableHead>
                  <TableHead>{t('Channel mappings')}</TableHead>
                  <TableHead className='text-right'>
                    {t('Revenue / cost')}
                  </TableHead>
                  <TableHead className='text-right'>
                    {t('Gross profit')}
                  </TableHead>
                  <TableHead className='text-right'>
                    {t('Gross margin')}
                  </TableHead>
                  <TableHead className='pr-4'>{t('Cost status')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {detailsQuery.isLoading && (
                  <TableRow>
                    <TableCell
                      colSpan={7}
                      className='text-muted-foreground py-12 text-center'
                    >
                      {t('Loading...')}
                    </TableCell>
                  </TableRow>
                )}
                {!detailsQuery.isLoading && detailsQuery.error && (
                  <TableRow>
                    <TableCell
                      colSpan={7}
                      className='text-destructive py-12 text-center'
                    >
                      {detailsQuery.error.message}
                    </TableCell>
                  </TableRow>
                )}
                {!detailsQuery.isLoading &&
                  !detailsQuery.error &&
                  details.length === 0 && (
                    <TableRow>
                      <TableCell
                        colSpan={7}
                        className='text-muted-foreground py-12 text-center'
                      >
                        {t('No profit details found')}
                      </TableCell>
                    </TableRow>
                  )}
                {!detailsQuery.isLoading &&
                  !detailsQuery.error &&
                  details.length > 0 &&
                  details.map((item) => {
                    const channels =
                      groupChannelsByID.get(String(item.groupId)) ?? []
                    const endpoint = providerEndpointByID.get(
                      String(item.providerId)
                    )
                    return (
                      <TableRow key={`${item.date}:${item.groupId}`}>
                        <TableCell className='pl-4 font-medium tabular-nums'>
                          {item.date}
                        </TableCell>
                        <TableCell>
                          <div className='font-medium'>{item.groupName}</div>
                          <div className='text-muted-foreground mt-1 text-xs'>
                            {endpoint ? (
                              <a
                                href={endpoint}
                                target='_blank'
                                rel='noreferrer'
                                className='hover:text-foreground hover:underline'
                                title={t('API endpoint')}
                              >
                                {item.providerName}
                              </a>
                            ) : (
                              item.providerName
                            )}{' '}
                            / {item.accountName}
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className='flex flex-wrap gap-1.5'>
                            {channels.length > 0 ? (
                              channels.map((channel) => (
                                <Badge
                                  key={channel.id}
                                  variant={
                                    channel.status === 'available'
                                      ? 'outline'
                                      : 'secondary'
                                  }
                                >
                                  #{channel.id} {channel.name}
                                </Badge>
                              ))
                            ) : (
                              <span className='text-muted-foreground'>-</span>
                            )}
                          </div>
                        </TableCell>
                        <TableCell className='text-right font-medium tabular-nums'>
                          {amountFormatter.format(item.revenue)} /{' '}
                          {item.cost === null
                            ? '-'
                            : amountFormatter.format(item.cost)}
                        </TableCell>
                        <TableCell className='text-right font-medium tabular-nums'>
                          {item.profit === null
                            ? '-'
                            : amountFormatter.format(item.profit)}
                        </TableCell>
                        <TableCell className='text-right tabular-nums'>
                          {item.grossMargin === null
                            ? '-'
                            : `${item.grossMargin.toFixed(1)}%`}
                        </TableCell>
                        <TableCell className='pr-4'>
                          <Badge
                            variant={
                              item.costStatus === 'ready'
                                ? 'outline'
                                : 'secondary'
                            }
                          >
                            {item.costStatus === 'ready'
                              ? t('Synced')
                              : t('Cost unavailable')}
                          </Badge>
                          {item.costObservedAt > 0 && (
                            <div className='text-muted-foreground mt-1 text-xs tabular-nums'>
                              {dayjs
                                .unix(item.costObservedAt)
                                .format('YYYY-MM-DD HH:mm')}
                            </div>
                          )}
                        </TableCell>
                      </TableRow>
                    )
                  })}
              </TableBody>
            </Table>
          </div>
          <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
            <div className='text-muted-foreground text-sm tabular-nums'>
              {t('Showing {{from}}-{{to}} of {{total}}', {
                from: displayFrom,
                to: displayTo,
                total,
              })}
            </div>
            <div className='flex items-center gap-3'>
              <label className='flex items-center gap-2 text-sm whitespace-nowrap'>
                {t('Rows per page')}
                <NativeSelect
                  className='w-20'
                  value={String(pageSize)}
                  onChange={(event) =>
                    updateSearch(
                      { pageSize: Number(event.target.value), page: 1 },
                      false
                    )
                  }
                >
                  <NativeSelectOption value='20'>20</NativeSelectOption>
                  <NativeSelectOption value='50'>50</NativeSelectOption>
                  <NativeSelectOption value='100'>100</NativeSelectOption>
                </NativeSelect>
              </label>
              {totalPages > 1 && (
                <Pagination className='mx-0 w-auto'>
                  <PaginationContent>
                    <PaginationItem>
                      <PaginationPrevious
                        href='#previous'
                        text={t('Previous')}
                        aria-disabled={page === 1}
                        className={
                          page === 1
                            ? 'pointer-events-none opacity-50'
                            : undefined
                        }
                        onClick={(event) => {
                          event.preventDefault()
                          setPage(page - 1)
                        }}
                      />
                    </PaginationItem>
                    <PaginationItem>
                      <PaginationLink href='#current' isActive>
                        {page}
                      </PaginationLink>
                    </PaginationItem>
                    <PaginationItem>
                      <PaginationNext
                        href='#next'
                        text={t('Next')}
                        aria-disabled={page === totalPages}
                        className={
                          page === totalPages
                            ? 'pointer-events-none opacity-50'
                            : undefined
                        }
                        onClick={(event) => {
                          event.preventDefault()
                          setPage(page + 1)
                        }}
                      />
                    </PaginationItem>
                  </PaginationContent>
                </Pagination>
              )}
            </div>
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
