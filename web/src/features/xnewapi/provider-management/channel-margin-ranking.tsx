/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Present the channel profitability leaderboard from normalized provider API data.

import { ArrowUpRight, Trophy } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { cn } from '@/lib/utils'

import type { UpstreamProvider } from './types'

type ChannelMarginRankingProps = {
  providers: UpstreamProvider[]
  formatAmount: (value: number) => string
  onViewProfitDetails: (filter: { providerId: string; groupId: string }) => void
}

type ChannelMarginRankingItem = {
  groupKey: string
  groupId: string
  groupName: string
  providerId: string
  providerName: string
  lastSyncedAt: string
  revenue: number
  cost: number
  profit: number
  margin: number
}

export function ChannelMarginRanking(props: ChannelMarginRankingProps) {
  const { t } = useTranslation('xnewapi')
  const rankings = useMemo<ChannelMarginRankingItem[]>(() => {
    const items: ChannelMarginRankingItem[] = []
    for (const provider of props.providers) {
      for (const account of provider.accounts) {
        for (const group of account.groups) {
          const groupProfit = group.profit
          if (
            !groupProfit ||
            groupProfit.revenue <= 0 ||
            groupProfit.cost === null
          ) {
            continue
          }
          const profit = groupProfit.revenue - groupProfit.cost
          items.push({
            groupKey: groupProfit.groupKey,
            groupId: String(groupProfit.groupId),
            groupName: groupProfit.groupName,
            providerId: String(groupProfit.providerId),
            providerName: provider.name,
            lastSyncedAt: account.lastSyncedAt,
            revenue: groupProfit.revenue,
            cost: groupProfit.cost,
            profit,
            margin: (profit / groupProfit.revenue) * 100,
          })
        }
      }
    }
    return items.sort(
      (left, right) =>
        right.margin - left.margin || right.revenue - left.revenue
    )
  }, [props.providers])

  return (
    <Card className='gap-0 py-0 shadow-none'>
      <CardHeader className='flex-row items-start justify-between gap-4 px-4 py-4 sm:items-center'>
        <CardTitle className='flex items-center gap-2 text-base'>
          <Trophy className='text-primary size-4' aria-hidden />
          {t('Gross margin ranking')}
        </CardTitle>
      </CardHeader>
      <CardContent className='p-0'>
        {rankings.length === 0 ? (
          <div className='text-muted-foreground border-t px-4 py-12 text-center text-sm'>
            {t('No model providers found')}
          </div>
        ) : (
          <div className='overflow-x-auto border-t'>
            <Table className='min-w-[720px]'>
              <TableHeader className='bg-muted/40'>
                <TableRow>
                  <TableHead className='w-14 pl-4'>#</TableHead>
                  <TableHead>{t('Provider group')}</TableHead>
                  <TableHead className='text-right'>
                    {t('Revenue / cost')}
                  </TableHead>
                  <TableHead className='text-right'>
                    {t('Gross profit')}
                  </TableHead>
                  <TableHead className='w-56 pr-4'>
                    {t('Gross margin')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rankings.map((item, index) => {
                  const barWidth = Math.min(Math.max(item.margin, 0), 100)
                  return (
                    <TableRow key={item.groupKey}>
                      <TableCell className='pl-4 font-medium tabular-nums'>
                        {index + 1}
                      </TableCell>
                      <TableCell>
                        <div className='flex min-w-0 items-center gap-1'>
                          <div className='truncate font-medium'>
                            {item.groupName || '-'}
                          </div>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            className='shrink-0'
                            onClick={() =>
                              props.onViewProfitDetails({
                                providerId: item.providerId,
                                groupId: item.groupId,
                              })
                            }
                            aria-label={t('View profit details')}
                            title={t('View profit details')}
                          >
                            <ArrowUpRight />
                          </Button>
                        </div>
                        <div className='text-muted-foreground mt-1 truncate text-xs'>
                          {item.providerName} / {item.lastSyncedAt}
                        </div>
                      </TableCell>
                      <TableCell className='text-right font-medium tabular-nums'>
                        {props.formatAmount(item.revenue)} /{' '}
                        {props.formatAmount(item.cost)}
                      </TableCell>
                      <TableCell
                        className={cn(
                          'text-right font-medium tabular-nums',
                          item.profit < 0 && 'text-destructive'
                        )}
                      >
                        {props.formatAmount(item.profit)}
                      </TableCell>
                      <TableCell className='pr-4'>
                        <div className='flex items-center gap-3'>
                          <div
                            className='bg-muted h-1.5 min-w-0 flex-1 overflow-hidden rounded-full'
                            aria-hidden
                          >
                            <div
                              className={cn(
                                'h-full rounded-full',
                                item.margin < 0
                                  ? 'bg-destructive'
                                  : 'bg-primary'
                              )}
                              style={{ width: `${barWidth}%` }}
                            />
                          </div>
                          <span
                            className={cn(
                              'w-14 text-right text-sm font-medium tabular-nums',
                              item.margin < 0 && 'text-destructive'
                            )}
                          >
                            {item.margin.toFixed(1)}%
                          </span>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
