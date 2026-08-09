/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Present profitability totals derived from provider API data.

import { ChartNoAxesCombined, ReceiptText, TrendingUp } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

import { getProviderTotals } from './provider-totals'
import type { UpstreamProvider } from './types'

type ProviderMetricsProps = {
  providers: UpstreamProvider[]
  formatAmount: (value: number) => string
  dateRangeLabel: string
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
    <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3'>
      {metrics.map((metric) => (
        <Card key={metric.label} className='gap-3 py-4 shadow-none'>
          <CardHeader className='flex-row items-center justify-between px-4'>
            <CardTitle className='text-muted-foreground text-sm font-medium'>
              {metric.label}
            </CardTitle>
            <metric.icon className='text-muted-foreground size-4' aria-hidden />
          </CardHeader>
          <CardContent className='px-4'>
            <div className='text-xl font-semibold tabular-nums'>
              {metric.value}
            </div>
            <div className='text-muted-foreground mt-1 text-xs'>
              {metric.detail}
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
