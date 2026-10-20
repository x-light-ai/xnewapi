/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: 运营中心 - 利润分析

import { useQuery } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { useState, useMemo, useCallback } from 'react'

import {
  getProviderWorkspace,
  getProviderProfitTrend,
} from '../provider-management/api'
import { ChannelMarginRanking } from '../provider-management/channel-margin-ranking'
import { createProviderAmountFormatter } from '../provider-management/provider-amount'
import { ProviderDateRangePicker } from '../provider-management/provider-date-range'
import { ProviderProfitTrend } from '../provider-management/provider-profit-trend'
import type { ProviderProfitDateRange } from '../provider-management/types'

const MINIMUM_TREND_DAYS = 7

function getProfitTrendDateRange(
  selectedRange: ProviderProfitDateRange
): ProviderProfitDateRange {
  const selectedDays =
    dayjs(selectedRange.endDate).diff(selectedRange.startDate, 'day') + 1
  if (selectedDays >= MINIMUM_TREND_DAYS) return selectedRange

  return {
    startDate: dayjs(selectedRange.endDate)
      .subtract(MINIMUM_TREND_DAYS - 1, 'day')
      .format('YYYY-MM-DD'),
    endDate: selectedRange.endDate,
  }
}

export function OperationsProfit() {
  const [dateRange, setDateRange] = useState<ProviderProfitDateRange>(() => {
    const today = dayjs().format('YYYY-MM-DD')
    return { startDate: today, endDate: today }
  })

  const trendDateRange = getProfitTrendDateRange(dateRange)

  const workspaceQuery = useQuery({
    queryKey: ['upstream-providers', dateRange.startDate, dateRange.endDate],
    queryFn: () => getProviderWorkspace(dateRange),
  })

  const trendQuery = useQuery({
    queryKey: [
      'upstream-provider-profit-trend',
      trendDateRange.startDate,
      trendDateRange.endDate,
    ],
    queryFn: () => getProviderProfitTrend(trendDateRange),
  })

  const amountFormatter = useMemo(() => createProviderAmountFormatter('zh'), [])
  const formatAmount = useCallback(
    (value: number) => amountFormatter.format(value),
    [amountFormatter]
  )

  // 从 workspace 中提取利润排行数据
  const profitGroups = useMemo(() => {
    if (!workspaceQuery.data) return []
    return workspaceQuery.data.profits ?? []
  }, [workspaceQuery.data])

  return (
    <div className='space-y-6'>
      {/* 工具栏 */}
      <div className='flex flex-col items-start gap-3'>
        <div className='flex items-center gap-2'>
          <ProviderDateRangePicker value={dateRange} onChange={setDateRange} />
          {/* TODO: 迁移利润详情页面到 /channel-operations/profit-details */}
          {/* <Button
            variant='outline'
            onClick={() => {
              window.open(
                `/channel-operations/profit-details?start_date=${dateRange.startDate}&end_date=${dateRange.endDate}`,
                '_blank'
              )
            }}
          >
            <ChartNoAxesCombined className='size-4' />
            查看详细报表
          </Button> */}
        </div>
      </div>

      {/* 利润趋势图 */}
      <div className='rounded-lg border'>
        <div className='bg-muted/50 border-b px-4 py-3'>
          <h3 className='font-semibold'>利润趋势</h3>
          <p className='text-muted-foreground text-sm'>
            {trendDateRange.startDate} 至 {trendDateRange.endDate}
          </p>
        </div>
        <div className='p-4'>
          <ProviderProfitTrend
            points={trendQuery.data ?? []}
            isLoading={trendQuery.isLoading}
            dateRangeLabel={`${trendDateRange.startDate} 至 ${trendDateRange.endDate}`}
            formatAmount={formatAmount}
          />
        </div>
      </div>

      {/* 渠道毛利排行 */}
      <div className='rounded-lg border'>
        <div className='bg-muted/50 border-b px-4 py-3'>
          <h3 className='font-semibold'>渠道毛利排行 Top 10</h3>
          <p className='text-muted-foreground text-sm'>
            {dateRange.startDate === dateRange.endDate
              ? dateRange.startDate
              : `${dateRange.startDate} 至 ${dateRange.endDate}`}
          </p>
        </div>
        <div className='p-4'>
          {/* TODO: 迁移利润详情页面后恢复 onViewProfitDetails 回调 */}
          <ChannelMarginRanking
            profits={profitGroups}
            formatAmount={formatAmount}
            onViewProfitDetails={undefined}
          />
        </div>
      </div>

      {/* 说明 */}
      <div className='text-muted-foreground rounded-lg border border-dashed px-4 py-3 text-sm'>
        💡 提示:
        <ul className='mt-2 list-inside list-disc space-y-1'>
          <li>利润趋势图显示每日的收入、成本和利润变化</li>
          <li>渠道毛利排行按 Group 维度统计，展示毛利率最高的前10个</li>
          <li>点击"查看详细报表"可查看按日期展开的完整利润明细</li>
        </ul>
      </div>
    </div>
  )
}
