/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Rank channel groups by reliability and provider profitability.

import { ArrowDown, ArrowUp, ArrowUpDown, ExternalLink } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { cn } from '@/lib/utils'

import { formatRate } from './channel-monitor-utils'
import {
  buildChannelPerformanceRows,
  sortChannelPerformanceRows,
  type RankingSortKey,
} from './channel-performance-ranking-utils'
import type { UpstreamProvider } from './provider-management/types'
import type { ChannelMonitorItem } from './types'

type ChannelPerformanceRankingProps = {
  items: ChannelMonitorItem[]
  providers: UpstreamProvider[]
  loading: boolean
}

function formatRevenue(value: number | null): string {
  if (value == null) return '-'
  return value.toFixed(2)
}

function providerLink(endpoint: string): string | null {
  try {
    const url = new URL(endpoint)
    return url.protocol === 'http:' || url.protocol === 'https:'
      ? url.toString()
      : null
  } catch {
    return null
  }
}

export function ChannelPerformanceRanking(
  props: ChannelPerformanceRankingProps
) {
  const { t } = useTranslation('xnewapi')
  const [sortKey, setSortKey] = useState<RankingSortKey>('request_count')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')

  const rows = useMemo(
    () =>
      sortChannelPerformanceRows(
        buildChannelPerformanceRows(props.items, props.providers),
        sortKey,
        sortOrder
      ),
    [props.items, props.providers, sortKey, sortOrder]
  )

  const sort = (key: RankingSortKey) => {
    if (sortKey === key) {
      setSortOrder((current) => (current === 'asc' ? 'desc' : 'asc'))
      return
    }
    setSortKey(key)
    setSortOrder('desc')
  }

  const sortableHead = (label: string, key: RankingSortKey) => {
    let Icon = ArrowUpDown
    if (sortKey === key) Icon = sortOrder === 'asc' ? ArrowUp : ArrowDown
    let ariaSort: 'ascending' | 'descending' | 'none' = 'none'
    if (sortKey === key) {
      ariaSort = sortOrder === 'asc' ? 'ascending' : 'descending'
    }
    return (
      <TableHead className='text-right' aria-sort={ariaSort}>
        <Button
          variant='ghost'
          size='sm'
          className='-mr-3 ml-auto h-8 px-3'
          onClick={() => sort(key)}
        >
          {label}
          <Icon className='size-3.5' aria-hidden />
        </Button>
      </TableHead>
    )
  }

  return (
    <section className='bg-card overflow-hidden rounded-lg border'>
      <header className='border-b px-4 py-3'>
        <h2 className='font-semibold'>{t('Channel ranking')}</h2>
      </header>
      {props.loading && (
        <div className='space-y-2 p-4'>
          {Array.from({ length: 4 }, (_, index) => (
            <Skeleton key={index} className='h-10 w-full' />
          ))}
        </div>
      )}
      {!props.loading && rows.length === 0 && (
        <Empty className='min-h-48'>
          <EmptyHeader>
            <EmptyTitle>{t('No channel monitoring data')}</EmptyTitle>
            <EmptyDescription>{t('No data')}</EmptyDescription>
          </EmptyHeader>
        </Empty>
      )}
      {!props.loading && rows.length > 0 && (
        <div className='overflow-x-auto'>
          <Table className='min-w-[980px]'>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Channel')}</TableHead>
                <TableHead>{t('Provider')}</TableHead>
                {sortableHead(t('Requests'), 'request_count')}
                {sortableHead(t('Failure count'), 'failure_count')}
                {sortableHead(t('Success rate'), 'success_rate')}
                {sortableHead(t('Failure rate'), 'failure_rate')}
                {sortableHead(t('Gross margin'), 'gross_margin')}
                {sortableHead(t('Revenue'), 'revenue')}
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => {
                const endpoint = providerLink(row.providerEndpoint)
                return (
                  <TableRow key={row.id}>
                    <TableCell className='max-w-72 font-medium'>
                      <span className='line-clamp-2'>
                        {row.channelNames.join(', ')}
                      </span>
                    </TableCell>
                    <TableCell>
                      {endpoint ? (
                        <a
                          href={endpoint}
                          target='_blank'
                          rel='noreferrer'
                          className='hover:text-primary inline-flex items-center gap-1 font-medium underline-offset-4 hover:underline'
                        >
                          {row.providerName}
                          <ExternalLink className='size-3.5' aria-hidden />
                        </a>
                      ) : (
                        row.providerName
                      )}
                    </TableCell>
                    <TableCell className='text-right tabular-nums'>
                      {row.requestCount.toLocaleString()}
                    </TableCell>
                    <TableCell className='text-right tabular-nums'>
                      {row.failureCount.toLocaleString()}
                    </TableCell>
                    <TableCell className='text-right tabular-nums'>
                      {formatRate(row.successRate)}
                    </TableCell>
                    <TableCell className='text-right tabular-nums'>
                      {formatRate(row.failureRate)}
                    </TableCell>
                    <TableCell
                      className={cn(
                        'text-right font-medium tabular-nums',
                        row.grossMargin != null &&
                          row.grossMargin < 0 &&
                          'text-destructive'
                      )}
                    >
                      {row.grossMargin == null
                        ? '-'
                        : `${row.grossMargin.toFixed(1)}%`}
                    </TableCell>
                    <TableCell className='text-right font-medium tabular-nums'>
                      {formatRevenue(row.revenue)}
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}
    </section>
  )
}
