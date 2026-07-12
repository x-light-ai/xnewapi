/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Port the fork-owned channel monitoring workspace to the default frontend.

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { VChart } from '@visactor/react-vchart'
import {
  Activity,
  Gauge,
  Radio,
  RefreshCw,
  RotateCcw,
  Save,
  ShieldCheck,
  Timer,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useChartTheme } from '@/lib/use-chart-theme'

import {
  clearChannelCircuit,
  getChannelMonitorChannels,
  getChannelMonitorRankings,
  getChannelMonitorSummary,
  getChannelMonitorTimeline,
  setChannelScore,
  updateChannelPriority,
} from './api'
import type { ChannelMonitorItem } from './types'

const DAY_OPTIONS = [1, 7, 30] as const

function formatRate(value: number): string {
  if (!Number.isFinite(value)) return '0.0%'
  return `${value.toFixed(1)}%`
}

function formatLatency(value: number): string {
  if (!Number.isFinite(value)) return '0 ms'
  return `${Math.round(value)} ms`
}

function ChannelStatusBadge(props: { item: ChannelMonitorItem }) {
  const { t } = useTranslation()
  if (props.item.temporary_circuit_open) {
    return <Badge variant='destructive'>{t('Circuit open')}</Badge>
  }
  if (props.item.status === 1) {
    return <Badge variant='secondary'>{t('Healthy')}</Badge>
  }
  return <Badge variant='outline'>{t('Disabled')}</Badge>
}

export function ChannelMonitor() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { resolvedTheme, themeReady } = useChartTheme()
  const [days, setDays] = useState<number>(7)
  const [group, setGroup] = useState('all')
  const [priorityDrafts, setPriorityDrafts] = useState<Record<number, string>>(
    {}
  )
  const [scoreDrafts, setScoreDrafts] = useState<Record<number, string>>({})
  const groupParam = group === 'all' ? '' : group

  const summaryQuery = useQuery({
    queryKey: ['channel-monitor', 'summary', days],
    queryFn: () => getChannelMonitorSummary(days),
  })
  const channelsQuery = useQuery({
    queryKey: ['channel-monitor', 'channels', days, groupParam],
    queryFn: () => getChannelMonitorChannels(days, groupParam),
  })
  const timelineQuery = useQuery({
    queryKey: ['channel-monitor', 'timeline', groupParam],
    queryFn: () => getChannelMonitorTimeline(groupParam),
  })
  const rankingsQuery = useQuery({
    queryKey: ['channel-monitor', 'rankings', days],
    queryFn: () => getChannelMonitorRankings(days),
  })

  useEffect(() => {
    const nextPriorities: Record<number, string> = {}
    const nextScores: Record<number, string> = {}
    for (const item of channelsQuery.data?.items ?? []) {
      nextPriorities[item.id] = String(item.priority)
      nextScores[item.id] = String(item.current_weighted_score)
    }
    setPriorityDrafts(nextPriorities)
    setScoreDrafts(nextScores)
  }, [channelsQuery.data?.items])

  const refreshMonitor = () =>
    queryClient.invalidateQueries({ queryKey: ['channel-monitor'] })

  const priorityMutation = useMutation({
    mutationFn: (input: { channelId: number; priority: number }) =>
      updateChannelPriority(input.channelId, input.priority),
    onSuccess: () => {
      toast.success(t('Channel priority updated'))
      refreshMonitor()
    },
    onError: (error: Error) => toast.error(error.message),
  })
  const scoreMutation = useMutation({
    mutationFn: (input: { channelId: number; score: number | null }) =>
      setChannelScore(input.channelId, input.score),
    onSuccess: () => {
      toast.success(t('Channel score updated'))
      refreshMonitor()
    },
    onError: (error: Error) => toast.error(error.message),
  })
  const circuitMutation = useMutation({
    mutationFn: clearChannelCircuit,
    onSuccess: () => {
      toast.success(t('Temporary circuit cleared'))
      refreshMonitor()
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const savePriority = (item: ChannelMonitorItem) => {
    const priority = Number(priorityDrafts[item.id])
    if (!Number.isFinite(priority)) {
      toast.error(t('Please enter a valid number'))
      return
    }
    priorityMutation.mutate({ channelId: item.id, priority })
  }

  const saveScore = (item: ChannelMonitorItem) => {
    const raw = scoreDrafts[item.id]?.trim() ?? ''
    const score = raw === '' ? null : Number(raw)
    if (score !== null && !Number.isFinite(score)) {
      toast.error(t('Please enter a valid number'))
      return
    }
    scoreMutation.mutate({ channelId: item.id, score })
  }

  const chartSpec = useMemo(() => {
    const points = (timelineQuery.data ?? []).flatMap((series) =>
      series.points.map((point) => ({
        time: new Date(point.time_bucket).toLocaleTimeString([], {
          hour: '2-digit',
          minute: '2-digit',
        }),
        channel: series.channel_name,
        successRate:
          point.request_count > 0
            ? (point.success_count / point.request_count) * 100
            : 0,
      }))
    )
    if (points.length === 0) return null
    return {
      type: 'line' as const,
      data: [{ id: 'channel-availability', values: points }],
      xField: 'time',
      yField: 'successRate',
      seriesField: 'channel',
      point: { visible: false },
      legends: { visible: true, orient: 'bottom' as const },
      axes: [
        { orient: 'bottom' as const, label: { autoHide: true } },
        {
          orient: 'left' as const,
          min: 0,
          max: 100,
          label: { formatMethod: (value: number) => `${value}%` },
        },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: (datum: Record<string, unknown>) =>
                String(datum.channel ?? ''),
              value: (datum: Record<string, unknown>) =>
                formatRate(Number(datum.successRate)),
            },
          ],
        },
      },
      animationAppear: { duration: 300 },
    }
  }, [timelineQuery.data])

  const summary = summaryQuery.data
  const loading = summaryQuery.isLoading || channelsQuery.isLoading
  const queryError =
    summaryQuery.error || channelsQuery.error || timelineQuery.error

  return (
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
                    {loading ? <Skeleton className='h-6 w-20' /> : metric.value}
                  </div>
                </div>
              </div>
            ))}
          </div>

          <section className='bg-card rounded-lg border'>
            <header className='border-b px-4 py-3'>
              <h2 className='font-semibold'>{t('Availability timeline')}</h2>
              <p className='text-muted-foreground mt-0.5 text-xs'>
                {t('Success rate by channel over the last 24 hours')}
              </p>
            </header>
            <div className='h-72 p-4'>
              {themeReady && chartSpec ? (
                <VChart
                  key={`channel-monitor-${resolvedTheme}-${group}`}
                  spec={{
                    ...chartSpec,
                    theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                    background: 'transparent',
                  }}
                />
              ) : (
                <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
                  {t('No timeline data available')}
                </div>
              )}
            </div>
          </section>

          <section className='bg-card overflow-hidden rounded-lg border'>
            <header className='flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3'>
              <div>
                <h2 className='font-semibold'>{t('Channel health')}</h2>
                <p className='text-muted-foreground mt-0.5 text-xs'>
                  {t('Inspect routing health and adjust channel selection')}
                </p>
              </div>
              <Select
                items={[
                  { value: 'all', label: t('All groups') },
                  ...(channelsQuery.data?.groups ?? []).map((value) => ({
                    value,
                    label: value,
                  })),
                ]}
                value={group}
                onValueChange={(value) => {
                  if (value) setGroup(value)
                }}
              >
                <SelectTrigger aria-label={t('Channel group')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value='all'>{t('All groups')}</SelectItem>
                    {(channelsQuery.data?.groups ?? []).map((value) => (
                      <SelectItem key={value} value={value}>
                        {value}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </header>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Channel')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Success rate')}</TableHead>
                  <TableHead>{t('Requests')}</TableHead>
                  <TableHead>{t('Latency')}</TableHead>
                  <TableHead>{t('Priority')}</TableHead>
                  <TableHead>{t('Score')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(channelsQuery.data?.items ?? []).map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>
                      <div className='font-medium'>{item.name}</div>
                      <div className='text-muted-foreground text-xs'>
                        #{item.id} · {item.group_name || '-'}
                      </div>
                    </TableCell>
                    <TableCell>
                      <ChannelStatusBadge item={item} />
                    </TableCell>
                    <TableCell>{formatRate(item.success_rate)}</TableCell>
                    <TableCell>{item.request_count.toLocaleString()}</TableCell>
                    <TableCell>{formatLatency(item.avg_latency)}</TableCell>
                    <TableCell>
                      <div className='flex items-center gap-1'>
                        <Input
                          type='number'
                          className='h-8 w-20'
                          value={priorityDrafts[item.id] ?? ''}
                          onChange={(event) =>
                            setPriorityDrafts((current) => ({
                              ...current,
                              [item.id]: event.target.value,
                            }))
                          }
                          aria-label={t('Priority for {{name}}', {
                            name: item.name,
                          })}
                        />
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          onClick={() => savePriority(item)}
                          aria-label={t('Save priority')}
                          title={t('Save priority')}
                        >
                          <Save className='size-3.5' />
                        </Button>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className='flex items-center gap-1'>
                        <Input
                          type='number'
                          step='0.01'
                          className='h-8 w-24'
                          value={scoreDrafts[item.id] ?? ''}
                          onChange={(event) =>
                            setScoreDrafts((current) => ({
                              ...current,
                              [item.id]: event.target.value,
                            }))
                          }
                          aria-label={t('Score for {{name}}', {
                            name: item.name,
                          })}
                        />
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          onClick={() => saveScore(item)}
                          aria-label={t('Save score')}
                          title={t('Save score')}
                        >
                          <Save className='size-3.5' />
                        </Button>
                      </div>
                    </TableCell>
                    <TableCell className='text-right'>
                      <Button
                        variant='outline'
                        size='sm'
                        disabled={!item.temporary_circuit_open}
                        onClick={() => circuitMutation.mutate(item.id)}
                      >
                        <RotateCcw className='size-3.5' />
                        {t('Clear circuit')}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {!channelsQuery.isLoading &&
                  (channelsQuery.data?.items.length ?? 0) === 0 && (
                    <TableRow>
                      <TableCell colSpan={8} className='h-28 text-center'>
                        {t('No channel monitoring data')}
                      </TableCell>
                    </TableRow>
                  )}
              </TableBody>
            </Table>
          </section>

          <div className='grid gap-4 lg:grid-cols-2'>
            {[
              {
                title: t('Most stable channels'),
                items: rankingsQuery.data?.stability ?? [],
                value: (item: ChannelMonitorItem) =>
                  formatRate(item.success_rate),
              },
              {
                title: t('Lowest latency channels'),
                items: rankingsQuery.data?.latency ?? [],
                value: (item: ChannelMonitorItem) =>
                  formatLatency(item.avg_latency),
              },
            ].map((ranking) => (
              <section
                key={ranking.title}
                className='bg-card rounded-lg border'
              >
                <h2 className='border-b px-4 py-3 font-semibold'>
                  {ranking.title}
                </h2>
                <div className='divide-y'>
                  {ranking.items.slice(0, 8).map((item, index) => (
                    <div
                      key={item.id}
                      className='flex items-center justify-between gap-3 px-4 py-2.5'
                    >
                      <div className='min-w-0 truncate text-sm'>
                        <span className='text-muted-foreground mr-2 tabular-nums'>
                          {index + 1}
                        </span>
                        {item.name}
                      </div>
                      <span className='shrink-0 text-sm font-medium tabular-nums'>
                        {ranking.value(item)}
                      </span>
                    </div>
                  ))}
                </div>
              </section>
            ))}
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
