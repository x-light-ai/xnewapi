/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: 运营中心 - 概览 Dashboard

import { useQuery } from '@tanstack/react-query'
import {
  AlertTriangle,
  Gauge,
  Radio,
  ShieldCheck,
  Timer,
  TrendingUp,
} from 'lucide-react'
import { useState } from 'react'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

import { getChannelMonitorSummary, getChannelMonitorChannels } from '../api'
import {
  formatLatency,
  formatRate,
  getChannelMonitorPeriodDateRange,
  getChannelMonitorPeriodDays,
  type ChannelMonitorPeriod,
} from '../channel-monitor-utils'
import { getProviderWorkspace } from '../provider-management/api'
import type { ProviderProfitDateRange } from '../provider-management/types'
import { ProviderProfitOverview } from './provider-profit-overview'

const DAY_OPTIONS = [
  { value: 1, label: '今天' },
  { value: 2, label: '昨天' },
  { value: 7, label: '近7天' },
  { value: 30, label: '近30天' },
] as const

const MONTH_OPTION = { value: 'month', label: '本月' } as const

export function OperationsOverview(props: {
  onViewChannel: (channelId: number) => void
}) {
  const [period, setPeriod] = useState<ChannelMonitorPeriod>(1)

  const days = getChannelMonitorPeriodDays(period)
  const dateRange: ProviderProfitDateRange =
    getChannelMonitorPeriodDateRange(period)

  const summaryQuery = useQuery({
    queryKey: ['channel-monitor', 'summary', days],
    queryFn: () => getChannelMonitorSummary(days),
  })

  const channelsQuery = useQuery({
    queryKey: ['channel-monitor', 'channels', days],
    queryFn: () => getChannelMonitorChannels(days, ''),
  })

  const workspaceQuery = useQuery({
    queryKey: ['upstream-providers', dateRange.startDate, dateRange.endDate],
    queryFn: () => getProviderWorkspace(dateRange),
  })

  const summary = summaryQuery.data
  const channels = channelsQuery.data?.items ?? []
  const workspace = workspaceQuery.data
  const loading = summaryQuery.isLoading || workspaceQuery.isLoading

  // 计算总收入、成本、利润
  const financialMetrics = workspace?.providers.reduce(
    (acc, provider) => {
      provider.accounts.forEach((account) => {
        account.groups.forEach((group) => {
          if (group.profit) {
            acc.revenue += group.profit.revenue || 0
            acc.cost += group.profit.cost || 0
            acc.profit += group.profit.profit || 0
          }
        })
      })
      return acc
    },
    { revenue: 0, cost: 0, profit: 0 }
  ) || { revenue: 0, cost: 0, profit: 0 }

  const grossMargin =
    financialMetrics.revenue > 0
      ? ((financialMetrics.profit / financialMetrics.revenue) * 100).toFixed(1)
      : '0.0'

  // 识别需要关注的渠道
  const alertChannels = channels
    .filter(
      (ch) =>
        ch.success_rate < 90 ||
        ch.avg_latency > 500 ||
        ch.temporary_circuit_open
    )
    .slice(0, 5)

  return (
    <div className='space-y-6'>
      {/* 统一时间筛选：同时作用于监控指标与利润区块 */}
      <div className='flex flex-wrap items-center gap-2'>
        {DAY_OPTIONS.map((option) => (
          <Button
            key={option.value}
            variant={period === option.value ? 'default' : 'outline'}
            size='sm'
            onClick={() => setPeriod(option.value)}
          >
            {option.label}
          </Button>
        ))}
        <Button
          variant={period === MONTH_OPTION.value ? 'default' : 'outline'}
          size='sm'
          onClick={() => setPeriod(MONTH_OPTION.value)}
        >
          {MONTH_OPTION.label}
        </Button>
      </div>

      {/* 利润概览：监控指标与利润指标同一网格，每行 4 张 */}
      <ProviderProfitOverview
        dateRange={dateRange}
        leadingMetrics={
          <>
            <MetricCard
              label='活跃渠道'
              value={
                summary
                  ? `${summary.active_channels}/${summary.total_channels}`
                  : '-'
              }
              icon={Gauge}
              loading={loading}
            />
            <MetricCard
              label='总请求'
              value={summary?.total_requests.toLocaleString() ?? '-'}
              icon={Radio}
              loading={loading}
            />
            <MetricCard
              label='成功率'
              value={summary ? formatRate(summary.success_rate) : '-'}
              icon={ShieldCheck}
              loading={loading}
            />
            <MetricCard
              label='平均延迟'
              value={summary ? formatLatency(summary.avg_latency) : '-'}
              icon={Timer}
              loading={loading}
            />
            <MetricCard
              label='毛利率'
              value={`${grossMargin}%`}
              icon={TrendingUp}
              loading={loading}
            />
          </>
        }
      />

      {alertChannels.length > 0 && (
        <div className='rounded-lg border'>
          <div className='bg-muted/50 border-b px-4 py-3'>
            <div className='flex items-center gap-2'>
              <AlertTriangle className='size-4 text-amber-600' />
              <h3 className='font-semibold'>
                需要关注的渠道 ({alertChannels.length})
              </h3>
            </div>
          </div>
          <div className='divide-y'>
            {alertChannels.map((channel) => (
              <div key={channel.id} className='px-4 py-3'>
                <div className='flex items-start justify-between gap-4'>
                  <div className='min-w-0 flex-1'>
                    <div className='flex items-center gap-2'>
                      <div
                        className={`size-2 rounded-full ${
                          channel.success_rate < 90
                            ? 'bg-red-500'
                            : 'bg-amber-500'
                        }`}
                      />
                      <span className='font-medium'>{channel.name}</span>
                      <span className='text-muted-foreground text-sm'>
                        {channel.type}
                      </span>
                    </div>
                    <div className='text-muted-foreground mt-1 text-sm'>
                      {channel.success_rate < 90 &&
                        `成功率 ${formatRate(channel.success_rate)} (低于90%) `}
                      {channel.avg_latency > 500 &&
                        `延迟 ${formatLatency(channel.avg_latency)} (高于500ms) `}
                      {channel.temporary_circuit_open && '熔断中 '}
                      {channel.gross_margin !== null &&
                        channel.gross_margin !== undefined &&
                        `| 毛利率 ${channel.gross_margin.toFixed(1)}%`}
                    </div>
                  </div>
                  <div className='flex shrink-0 gap-2'>
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() => props.onViewChannel(channel.id)}
                    >
                      查看详情
                    </Button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function MetricCard(props: {
  label: string
  value: string
  icon: React.ElementType
  loading: boolean
}) {
  return (
    <div className='bg-card flex min-h-24 items-center gap-3 rounded-lg border px-4 py-3'>
      <span className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-md'>
        <props.icon className='text-muted-foreground size-4' />
      </span>
      <div className='min-w-0'>
        <div className='text-muted-foreground text-xs'>{props.label}</div>
        <div className='mt-1 text-xl font-semibold tabular-nums'>
          {props.loading ? <Skeleton className='h-6 w-20' /> : props.value}
        </div>
      </div>
    </div>
  )
}
