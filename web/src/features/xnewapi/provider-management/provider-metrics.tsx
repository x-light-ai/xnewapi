/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Present profitability totals derived from provider API data.

import { ChartNoAxesCombined, ReceiptText, TrendingUp } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { getProviderTotals } from './provider-totals'
import type { UpstreamProvider } from './types'

type ProviderMetricsProps = {
  providers: UpstreamProvider[]
  formatAmount: (value: number) => string
  dateRangeLabel: string
  // 渲染在利润卡片之前、同一网格内的额外卡片
  leading?: ReactNode
}

export function ProviderMetrics(props: ProviderMetricsProps) {
  const { t } = useTranslation('xnewapi')
  const totals = props.providers.map(getProviderTotals)
  const revenue = totals.reduce((sum, item) => sum + item.revenue, 0)
  const costComplete = totals.every((item) => item.cost !== null)
  const cost = costComplete
    ? totals.reduce((sum, item) => sum + (item.cost ?? 0), 0)
    : null
  const profit = cost === null ? null : revenue - cost
  const margin =
    profit !== null && revenue > 0 ? (profit / revenue) * 100 : null
  const groupCount = totals.reduce((sum, item) => sum + item.groupCount, 0)
  const metrics = [
    {
      label: t('Total revenue'),
      value: props.formatAmount(revenue),
      detail: props.dateRangeLabel,
      icon: ChartNoAxesCombined,
    },
    {
      label: t('Total cost'),
      value: cost === null ? '-' : props.formatAmount(cost),
      detail: t('{{count}} provider groups', { count: groupCount }),
      icon: ReceiptText,
    },
    {
      label: t('Gross profit'),
      value: profit === null ? '-' : props.formatAmount(profit),
      detail: t('{{value}} gross margin', {
        value: margin === null ? '-' : `${margin.toFixed(1)}%`,
      }),
      icon: TrendingUp,
    },
  ]

  return (
    <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
      {props.leading}
      {metrics.map((metric) => (
        <div
          key={metric.label}
          className='bg-card flex min-h-24 items-center gap-3 rounded-lg border px-4 py-3'
        >
          <span className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-md'>
            <metric.icon className='text-muted-foreground size-4' aria-hidden />
          </span>
          <div className='min-w-0'>
            <div className='text-muted-foreground text-xs'>{metric.label}</div>
            <div className='mt-1 text-xl font-semibold tabular-nums'>
              {metric.value}
            </div>
            <div className='text-muted-foreground mt-0.5 truncate text-xs'>
              {metric.detail}
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
