/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Port the fork-owned channel monitoring workspace to the default frontend.

import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Activity,
  Gauge,
  Radio,
  RefreshCw,
  ShieldCheck,
  Timer,
} from 'lucide-react'
import { lazy, Suspense, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'

import {
  getChannelMonitorChannels,
  getChannelMonitorSummary,
  getChannelMonitorTimeline,
} from './api'
import { ChannelMonitorTable } from './channel-monitor-table'
import { formatLatency, formatRate } from './channel-monitor-utils'
import { ChannelPerformanceRanking } from './channel-performance-ranking'
import { getProviderWorkspace } from './provider-management/api'

const DAY_OPTIONS = [1, 7, 14, 30] as const
const ChannelMutateDrawer = lazy(() =>
  import('@/features/channels/components/drawers/channel-mutate-drawer').then(
    (module) => ({ default: module.ChannelMutateDrawer })
  )
)

export function ChannelMonitor() {
  return <ChannelMonitorContent />
}

function ChannelMonitorContent() {
  const { t } = useTranslation('xnewapi')
  const queryClient = useQueryClient()
  const [days, setDays] = useState<number>(1)
  const [editingChannelId, setEditingChannelId] = useState<number | null>(null)

  const summaryQuery = useQuery({
    queryKey: ['channel-monitor', 'summary', days],
    queryFn: () => getChannelMonitorSummary(days),
  })
  const channelsQuery = useQuery({
    queryKey: ['channel-monitor', 'channels', days],
    queryFn: () => getChannelMonitorChannels(days, ''),
  })
  const timelineQuery = useQuery({
    queryKey: ['channel-monitor', 'timeline', days],
    queryFn: () => getChannelMonitorTimeline(days, ''),
  })
  const providerWorkspaceQuery = useQuery({
    queryKey: ['upstream-providers'],
    queryFn: () => getProviderWorkspace(),
  })

  const refreshMonitor = () =>
    queryClient.invalidateQueries({ queryKey: ['channel-monitor'] })

  const summary = summaryQuery.data
  const loading = summaryQuery.isLoading || channelsQuery.isLoading
  const queryError =
    summaryQuery.error ||
    channelsQuery.error ||
    timelineQuery.error ||
    providerWorkspaceQuery.error

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          <div className='flex min-w-0 items-center gap-2'>
            <Activity className='text-primary size-5 shrink-0' />
            <span className='truncate'>{t('Channel Monitor')}</span>
          </div>
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <div className='flex items-center gap-2'>
            <Select
              items={DAY_OPTIONS.map((value) => ({
                value: String(value),
                label: t('{{count}} days', { count: value }),
              }))}
              value={String(days)}
              onValueChange={(value) => setDays(Number(value))}
            >
              <SelectTrigger aria-label={t('Time range')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {DAY_OPTIONS.map((value) => (
                    <SelectItem key={value} value={String(value)}>
                      {t('{{count}} days', { count: value })}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <Button
              variant='outline'
              size='icon'
              onClick={() => refreshMonitor()}
              aria-label={t('Refresh')}
              title={t('Refresh')}
            >
              <RefreshCw className='size-4' />
            </Button>
          </div>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-5'>
            {queryError && (
              <div className='border-destructive/40 bg-destructive/5 text-destructive rounded-lg border px-4 py-3 text-sm'>
                {queryError.message}
              </div>
            )}

            <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
              {[
                {
                  label: t('Total requests'),
                  value: summary?.total_requests.toLocaleString() ?? '-',
                  icon: Radio,
                },
                {
                  label: t('Success rate'),
                  value: summary ? formatRate(summary.success_rate) : '-',
                  icon: ShieldCheck,
                },
                {
                  label: t('Average latency'),
                  value: summary ? formatLatency(summary.avg_latency) : '-',
                  icon: Timer,
                },
                {
                  label: t('Active channels'),
                  value: summary
                    ? `${summary.active_channels}/${summary.total_channels}`
                    : '-',
                  icon: Gauge,
                },
              ].map((metric) => (
                <div
                  key={metric.label}
                  className='bg-card flex min-h-24 items-center gap-3 rounded-lg border px-4 py-3'
                >
                  <span className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-md'>
                    <metric.icon className='text-muted-foreground size-4' />
                  </span>
                  <div className='min-w-0'>
                    <div className='text-muted-foreground text-xs'>
                      {metric.label}
                    </div>
                    <div className='mt-1 text-xl font-semibold tabular-nums'>
                      {loading ? (
                        <Skeleton className='h-6 w-20' />
                      ) : (
                        metric.value
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>

            <ChannelMonitorTable
              items={channelsQuery.data?.items ?? []}
              timeline={timelineQuery.data ?? []}
              loading={channelsQuery.isLoading}
              onEditChannel={setEditingChannelId}
            />

            <ChannelPerformanceRanking
              items={channelsQuery.data?.items ?? []}
              providers={providerWorkspaceQuery.data?.providers ?? []}
              loading={
                channelsQuery.isLoading || providerWorkspaceQuery.isLoading
              }
            />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
      {editingChannelId !== null && (
        <Suspense fallback={null}>
          <ChannelMutateDrawer
            open
            onOpenChange={(open) => {
              if (open) return
              setEditingChannelId(null)
              refreshMonitor()
            }}
            currentRow={{ id: editingChannelId }}
          />
        </Suspense>
      )}
    </>
  )
}
