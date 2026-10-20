// FORK-CUSTOM: Channel operations overview profitability section.

import { useQuery } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { useMemo, type ReactNode } from 'react'

import {
  getProviderProfitTrend,
  getProviderWorkspace,
} from '../provider-management/api'
import { ChannelMarginRanking } from '../provider-management/channel-margin-ranking'
import { createProviderAmountFormatter } from '../provider-management/provider-amount'
import { ProviderMetrics } from '../provider-management/provider-metrics'
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

function formatDateRange(range: ProviderProfitDateRange): string {
  if (range.startDate === range.endDate) return range.startDate
  return `${range.startDate} – ${range.endDate}`
}

export function ProviderProfitOverview(props: {
  dateRange: ProviderProfitDateRange
  leadingMetrics?: ReactNode
}) {
  const dateRange = props.dateRange
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
  const formatAmount = amountFormatter.format.bind(amountFormatter)

  return (
    <div className='space-y-6'>
      <ProviderMetrics
        providers={workspaceQuery.data?.providers ?? []}
        formatAmount={formatAmount}
        dateRangeLabel={formatDateRange(dateRange)}
        leading={props.leadingMetrics}
      />
      <ChannelMarginRanking
        profits={workspaceQuery.data?.profits ?? []}
        formatAmount={formatAmount}
      />
      <ProviderProfitTrend
        points={trendQuery.data ?? []}
        isLoading={trendQuery.isLoading}
        dateRangeLabel={formatDateRange(trendDateRange)}
        formatAmount={formatAmount}
      />
    </div>
  )
}
