/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Render provider profitability trends from server-side daily aggregates.

import { VChart } from '@visactor/react-vchart'
import { ChartNoAxesCombined } from 'lucide-react'
import { useMemo, type ReactElement, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useChartTheme } from '@/lib/use-chart-theme'
import { VCHART_OPTION } from '@/lib/vchart'

import type { ProviderProfitDailyTrendPoint } from './types'

type ProviderProfitTrendProps = {
  points: ProviderProfitDailyTrendPoint[]
  isLoading: boolean
  dateRangeLabel: string
  formatAmount: (value: number) => string
}

type TrendChartValue = {
  date: string
  metric: string
  value: number
}

export function ProviderProfitTrend(
  props: ProviderProfitTrendProps
): ReactElement {
  const { t } = useTranslation('xnewapi')
  const { resolvedTheme, themeReady } = useChartTheme()
  const formatAmount = props.formatAmount
  const chartValues = useMemo<TrendChartValue[]>(() => {
    const values: TrendChartValue[] = []
    for (const point of props.points) {
      values.push({
        date: point.date,
        metric: t('Revenue'),
        value: point.revenue,
      })
      if (point.cost !== null) {
        values.push({ date: point.date, metric: t('Cost'), value: point.cost })
      }
      if (point.profit !== null) {
        values.push({
          date: point.date,
          metric: t('Gross profit'),
          value: point.profit,
        })
      }
    }
    return values
  }, [props.points, t])
  const spec = useMemo(
    () => ({
      type: 'line',
      data: [{ id: 'provider-profit-trend', values: chartValues }],
      xField: 'date',
      yField: 'value',
      seriesField: 'metric',
      line: {
        style: {
          curveType: 'monotone',
        },
      },
      point: { visible: true },
      legends: { visible: true, orient: 'top', position: 'end' },
      axes: [
        { orient: 'bottom' },
        {
          orient: 'left',
          label: {
            formatMethod: (value: number | string) =>
              formatAmount(Number(value)),
          },
        },
      ],
    }),
    [chartValues, formatAmount]
  )
  let chartContent: ReactNode = null
  if (props.isLoading) {
    chartContent = <Skeleton className='h-full w-full' />
  } else if (chartValues.length === 0) {
    chartContent = (
      <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
        {t('No profitability data for this period')}
      </div>
    )
  } else if (themeReady) {
    chartContent = (
      <VChart
        key={`${props.dateRangeLabel}-${resolvedTheme}`}
        spec={{
          ...spec,
          theme: resolvedTheme === 'dark' ? 'dark' : 'light',
          background: 'transparent',
        }}
        option={VCHART_OPTION}
      />
    )
  }

  return (
    <Card className='gap-0 py-0 shadow-none'>
      <CardHeader className='flex-row items-start justify-between gap-4 px-4 py-4 sm:items-center'>
        <div>
          <CardTitle className='flex items-center gap-2 text-base'>
            <ChartNoAxesCombined className='text-primary size-4' aria-hidden />
            {t('Provider profit trend')}
          </CardTitle>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('Daily aggregation')} - {props.dateRangeLabel}
          </p>
        </div>
      </CardHeader>
      <CardContent className='h-72 px-2 pb-2 sm:h-80'>
        {chartContent}
      </CardContent>
    </Card>
  )
}
