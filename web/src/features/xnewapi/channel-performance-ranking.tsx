/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Rank channel groups by reliability and provider profitability.

import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  ExternalLink,
  Medal,
} from 'lucide-react'
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
  buildRecommendedChannelGroups,
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

  const performanceRows = useMemo(
    () => buildChannelPerformanceRows(props.items, props.providers),
    [props.items, props.providers]
  )
  const recommendedGroups = useMemo(
    () => buildRecommendedChannelGroups(performanceRows),
    [performanceRows]
  )
  const rows = useMemo(
    () => sortChannelPerformanceRows(performanceRows, sortKey, sortOrder),
    [performanceRows, sortKey, sortOrder]
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
      <TableHead className='text-right whitespace-normal' aria-sort={ariaSort}>
        <Button
          variant='ghost'
          size='sm'
          className='ml-auto h-auto min-h-8 w-full justify-end px-1 text-right whitespace-normal'
          onClick={() => sort(key)}
        >
          <span className='leading-tight'>{label}</span>
          <Icon className='size-3.5 shrink-0' aria-hidden />
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
        <div>
          {recommendedGroups.length > 0 && (
            <div className='border-b px-4 py-4'>
              <div className='mb-3 flex items-center gap-2'>
                <Medal className='text-primary size-4' aria-hidden />
                <h3 className='text-sm font-semibold'>
                  {t('Recommended channels')}
                </h3>
              </div>
              <div className='divide-y rounded-md border'>
                {recommendedGroups.map((group) => (
                  <section key={group.groupName}>
                    <h4 className='bg-muted/40 border-b px-3 py-2 text-sm font-medium break-words'>
                      {group.groupName}
                    </h4>
                    <Table className='table-fixed'>
                      <TableHeader>
                        <TableRow>
                          <TableHead className='w-12 text-center'>#</TableHead>
                          <TableHead className='whitespace-normal'>
                            {t('Channel')}
                          </TableHead>
                          <TableHead className='w-36 text-right whitespace-normal'>
                            {t('Recommendation score')}
                          </TableHead>
                          <TableHead className='hidden w-28 text-right xl:table-cell'>
                            {t('Success rate')}
                          </TableHead>
                          <TableHead className='hidden w-28 text-right xl:table-cell'>
                            {t('Gross margin')}
                          </TableHead>
                          <TableHead className='hidden w-28 text-right xl:table-cell'>
                            {t('Requests')}
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {group.rows.map((row, index) => (
                          <TableRow
                            key={`recommended-${group.groupName}-${row.id}`}
                          >
                            <TableCell className='text-center font-semibold tabular-nums'>
                              {index + 1}
                            </TableCell>
                            <TableCell className='font-medium'>
                              <span className='block [overflow-wrap:anywhere] whitespace-normal'>
                                {row.channelNames.join(', ')}
                              </span>
                            </TableCell>
                            <TableCell className='text-right font-semibold tabular-nums'>
                              {row.recommendationScore.toFixed(1)}
                            </TableCell>
                            <TableCell className='hidden text-right tabular-nums xl:table-cell'>
                              {formatRate(row.successRate)}
                            </TableCell>
                            <TableCell className='hidden text-right tabular-nums xl:table-cell'>
                              {row.grossMargin?.toFixed(1)}%
                            </TableCell>
                            <TableCell className='hidden text-right tabular-nums xl:table-cell'>
                              {row.requestCount.toLocaleString()}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </section>
                ))}
              </div>
            </div>
          )}
          <Table className='table-fixed [&_td]:whitespace-normal'>
            <TableHeader>
              <TableRow>
                <TableHead className='w-[22%] whitespace-normal'>
                  {t('Channel')}
                </TableHead>
                <TableHead className='w-[18%] whitespace-normal'>
                  {t('Provider')}
                </TableHead>
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
                    <TableCell className='font-medium'>
                      <span className='line-clamp-2 break-words'>
                        {row.channelNames.join(', ')}
                      </span>
                    </TableCell>
                    <TableCell>
                      {endpoint ? (
                        <a
                          href={endpoint}
                          target='_blank'
                          rel='noreferrer'
                          className='hover:text-primary inline-flex min-w-0 items-center gap-1 font-medium break-all underline-offset-4 hover:underline'
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
