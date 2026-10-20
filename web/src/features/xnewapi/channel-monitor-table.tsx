/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Render the full channel health workspace with default-theme components.

import { Activity01Icon, RotateIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getChannelTypeIcon } from '@/features/channels/lib'
import { formatNumber } from '@/lib/format'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import {
  clearChannelCircuit,
  getChannelMonitorCacheModelStats,
  setChannelScore,
  updateChannelPriority,
} from './api'
import {
  compressTimelinePoints,
  formatBeijingDateTime,
  formatLatency,
  formatRate,
  formatTimelinePointTime,
  normalizeGroupName,
  type ChannelMonitorDisplayRow,
} from './channel-monitor-utils'
import type { ProviderProfitDateRange } from './provider-management/types'
import type {
  ChannelMonitorItem,
  ChannelTimeline,
  ChannelTimelinePoint,
} from './types'

function ChannelStatus(props: { status: number }) {
  const { t } = useTranslation('xnewapi')
  if (props.status === 1) {
    return (
      <StatusBadge label={t('Enabled')} variant='success' copyable={false} />
    )
  }
  if (props.status === 3) {
    return (
      <StatusBadge
        label={t('Automatically disabled')}
        variant='warning'
        copyable={false}
      />
    )
  }
  return <StatusBadge label={t('Disabled')} variant='danger' copyable={false} />
}

function ScoreBadge(props: { score: number }) {
  let variant: StatusVariant = 'danger'
  if (props.score >= 0.85) variant = 'success'
  else if (props.score >= 0.65) variant = 'lime'
  else if (props.score >= 0.4) variant = 'warning'
  return (
    <StatusBadge
      label={props.score.toFixed(2)}
      variant={variant}
      copyable={false}
    />
  )
}

function AvailabilityTrend(props: {
  item: ChannelMonitorItem
  timeline?: ChannelTimeline
  onSelectPoint: (point: ChannelTimelinePoint) => void
}) {
  const { t } = useTranslation('xnewapi')
  const trendRef = useRef<HTMLDivElement>(null)
  const [maxPoints, setMaxPoints] = useState(40)
  useEffect(() => {
    const trend = trendRef.current
    if (!trend) return
    const updateMaxPoints = () => {
      const nextMaxPoints = Math.max(
        4,
        Math.min(40, Math.floor(trend.clientWidth / 12))
      )
      setMaxPoints((current) =>
        current === nextMaxPoints ? current : nextMaxPoints
      )
    }
    updateMaxPoints()
    const observer = new ResizeObserver(updateMaxPoints)
    observer.observe(trend)
    return () => observer.disconnect()
  }, [])
  const points = compressTimelinePoints(props.timeline?.points ?? [], maxPoints)
  let maxRequests = 0
  for (const point of points) {
    maxRequests = Math.max(maxRequests, point.request_count)
  }
  const hasData = maxRequests > 0
  const lastActive = formatBeijingDateTime(props.item.last_active)

  // 计算时间范围
  const oldestPoint = points[0]
  const midPoint = points[Math.floor(points.length / 2)]
  const oldestTime = oldestPoint
    ? formatTimelinePointTime(oldestPoint).split(' - ')[0]
    : ''
  const midTime = midPoint
    ? formatTimelinePointTime(midPoint).split(' - ')[0]
    : ''

  return (
    <div className='flex w-full min-w-0 flex-col gap-4'>
      {/* 上方指标卡片 */}
      <div className='grid grid-cols-3 gap-2'>
        <div className='flex flex-col gap-1'>
          <span className='text-muted-foreground text-xs font-medium'>
            成功率
          </span>
          <span className='text-lg font-bold tabular-nums'>
            {formatRate(props.item.success_rate)}
          </span>
        </div>
        <div className='flex flex-col gap-1'>
          <span className='text-muted-foreground text-xs font-medium'>
            请求数
          </span>
          <span className='text-lg font-bold tabular-nums'>
            {props.item.request_count.toLocaleString()}
          </span>
        </div>
        <div className='flex flex-col gap-1'>
          <span className='text-muted-foreground text-xs font-medium'>
            P95延迟
          </span>
          <span className='text-lg font-bold tabular-nums'>
            {formatLatency(props.item.p95_latency)}
          </span>
        </div>
      </div>

      {/* 中间彩色时间轴 */}
      <TooltipProvider>
        <div
          ref={trendRef}
          className='bg-muted/30 flex h-12 w-full items-stretch gap-px overflow-hidden rounded-md border'
        >
          {hasData ? (
            points.map((point) => {
              const successRate =
                point.request_count > 0
                  ? point.success_count / point.request_count
                  : 0
              let bgColor = 'bg-muted-foreground' // 无数据
              if (point.request_count > 0) {
                if (successRate >= 0.95) bgColor = 'bg-green-500'
                else if (successRate >= 0.85) bgColor = 'bg-emerald-400'
                else if (successRate >= 0.7) bgColor = 'bg-yellow-400'
                else if (successRate >= 0.5) bgColor = 'bg-orange-400'
                else bgColor = 'bg-red-500'
              }

              // 根据请求数计算高度（0% - 100%）
              const heightPercent = Math.max(
                10,
                (point.request_count / maxRequests) * 100
              )

              return (
                <Tooltip key={point.time_bucket}>
                  <TooltipTrigger
                    render={
                      <button
                        type='button'
                        className='flex flex-1 items-end justify-center pb-1 transition-opacity hover:opacity-80'
                        onClick={() => props.onSelectPoint(point)}
                        aria-label={`${formatTimelinePointTime(point)} ${t('Failure details')}`}
                      >
                        <div
                          className={cn(
                            'w-full rounded-t transition-all',
                            bgColor
                          )}
                          style={{
                            height: `${heightPercent}%`,
                            opacity: point.request_count <= 0 ? 0.3 : 1,
                          }}
                        />
                      </button>
                    }
                  />
                  <TooltipContent>
                    <div className='flex flex-col gap-1 text-sm tabular-nums'>
                      <span className='font-medium'>
                        {formatTimelinePointTime(point)}
                      </span>
                      <span>成功率: {formatRate(successRate)}</span>
                      <span>
                        请求数: {point.request_count.toLocaleString()}
                      </span>
                      <span>
                        失败数: {point.failure_count.toLocaleString()}
                      </span>
                      {(point.failure_reasons ?? [])
                        .slice(0, 2)
                        .map((reason) => (
                          <span
                            key={`${reason.reason}-${reason.status_code}`}
                            className='text-muted-foreground text-xs'
                          >
                            {reason.reason}: {reason.count}
                          </span>
                        ))}
                    </div>
                  </TooltipContent>
                </Tooltip>
              )
            })
          ) : (
            <div className='text-muted-foreground flex flex-1 items-center justify-center text-sm'>
              -
            </div>
          )}
        </div>
      </TooltipProvider>

      {/* 下方时间标注 */}
      {hasData && points.length > 0 && (
        <div className='text-muted-foreground relative text-xs tabular-nums'>
          <div className='flex justify-between px-0.5'>
            <span>{oldestTime}</span>
            {points.length > 2 && (
              <span className='absolute left-1/2 -translate-x-1/2'>
                {midTime}
              </span>
            )}
            <span>现在</span>
          </div>
        </div>
      )}

      {/* 底部补充信息 */}
      <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs'>
        <span>失败 {props.item.failure_count.toLocaleString()}</span>
        <span>·</span>
        <span>{lastActive}</span>
      </div>
    </div>
  )
}

type ChannelMonitorTableProps = {
  rows: ChannelMonitorDisplayRow[]
  timeline: ChannelTimeline[]
  loading: boolean
  onEditChannel: (channelId: number) => void
  profitDateRange: ProviderProfitDateRange
  highlightChannelId: number | null
  onHighlighted: () => void
}

export function ChannelMonitorTable(props: ChannelMonitorTableProps) {
  const { t } = useTranslation('xnewapi')
  const queryClient = useQueryClient()
  const [scoreItem, setScoreItem] = useState<ChannelMonitorItem | null>(null)
  const [scoreDraft, setScoreDraft] = useState('')
  const [circuitItem, setCircuitItem] = useState<ChannelMonitorItem | null>(
    null
  )
  const [trendDetail, setTrendDetail] = useState<{
    item: ChannelMonitorItem
    point: ChannelTimelinePoint
  } | null>(null)

  const cacheModelQuery = useQuery({
    queryKey: [
      'channel-monitor',
      'cache-models',
      trendDetail?.item.id,
      trendDetail?.item.group_name,
      trendDetail?.point.time_bucket,
      trendDetail?.point.time_bucket_end,
    ],
    queryFn: () => {
      if (!trendDetail) return Promise.resolve([])
      const start = new Date(trendDetail.point.time_bucket)
      const end = trendDetail.point.time_bucket_end
        ? new Date(trendDetail.point.time_bucket_end)
        : new Date(start.getTime() + 10 * 60 * 1000)
      return getChannelMonitorCacheModelStats(
        trendDetail.item.id,
        start.toISOString(),
        end.toISOString(),
        trendDetail.item.group_name
      )
    },
    enabled: trendDetail !== null,
    staleTime: 30_000,
  })

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
      setScoreItem(null)
      refreshMonitor()
    },
    onError: (error: Error) => toast.error(error.message),
  })
  const circuitMutation = useMutation({
    mutationFn: clearChannelCircuit,
    onSuccess: () => {
      toast.success(t('Temporary circuit cleared'))
      setCircuitItem(null)
      refreshMonitor()
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const timelineByChannelId = useMemo(() => {
    const map = new Map<number, ChannelTimeline>()
    for (const timeline of props.timeline) {
      map.set(timeline.channel_id, timeline)
    }
    return map
  }, [props.timeline])

  useEffect(() => {
    if (props.highlightChannelId === null) return
    const row = document.querySelector(
      `[data-channel-id="${props.highlightChannelId}"]`
    )
    if (!row) return
    row.scrollIntoView({ behavior: 'smooth', block: 'center' })
    props.onHighlighted()
  }, [props, props.rows])

  const commitPriority = (item: ChannelMonitorItem, rawValue: string) => {
    if (rawValue.trim() === '') return
    const priority = Number(rawValue)
    if (!Number.isInteger(priority)) {
      toast.error(t('Priority must be an integer'))
      return
    }
    if (priority === item.priority) return
    priorityMutation.mutate({ channelId: item.id, priority })
  }

  const openScoreDialog = (item: ChannelMonitorItem) => {
    setScoreItem(item)
    setScoreDraft(item.current_weighted_score.toFixed(2))
  }

  const saveScore = () => {
    if (!scoreItem) return
    const score = Number(scoreDraft)
    if (!Number.isFinite(score) || score < 0 || score > 1) {
      toast.error(t('Score must be between 0 and 1'))
      return
    }
    scoreMutation.mutate({ channelId: scoreItem.id, score })
  }

  return (
    <section className='bg-card overflow-hidden rounded-lg border'>
      <header className='border-b px-4 py-3'>
        <h2 className='font-semibold'>{t('Channel health')}</h2>
        <p className='text-muted-foreground mt-0.5 text-xs'>
          {t('Inspect routing health and adjust channel selection')}
        </p>
      </header>

      <div>
        <Table className='w-full table-fixed'>
          <TableHeader>
            <TableRow>
              <TableHead className='w-72'>{t('Channel')}</TableHead>
              <TableHead className='hidden w-24 md:table-cell'>
                {t('Priority')}
              </TableHead>
              <TableHead className='hidden w-28 text-left lg:table-cell'>
                {t('Routing score')}
              </TableHead>
              <TableHead className='hidden w-24 xl:table-cell'>
                {t('Gross margin')}
              </TableHead>
              <TableHead>{t('Availability trend')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.rows.map((row) => {
              if (row.kind === 'group') {
                return (
                  <TableRow
                    key={row.id}
                    className='bg-muted/40 hover:bg-muted/40'
                  >
                    <TableCell>
                      <div className='flex flex-col gap-1'>
                        <div className='flex items-center gap-2'>
                          <GroupBadge group={row.group_name} />
                          <span className='font-medium'>
                            {t('Group summary')}
                          </span>
                        </div>
                        <span className='text-muted-foreground text-xs'>
                          {t('{{count}} channels · {{requests}} requests', {
                            count: row.channel_count,
                            requests: row.request_count.toLocaleString(),
                          })}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell className='hidden md:table-cell'>-</TableCell>
                    <TableCell className='hidden lg:table-cell'>-</TableCell>
                    <TableCell className='hidden xl:table-cell'>-</TableCell>
                    <TableCell>
                      <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs tabular-nums'>
                        <span>{formatRate(row.success_rate)}</span>
                        <span>P95 {formatLatency(row.p95_latency)}</span>
                        <span>
                          {t('Requests {{requests}} / failures {{failures}}', {
                            requests: row.request_count.toLocaleString(),
                            failures: row.failure_count.toLocaleString(),
                          })}
                        </span>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              }

              const item = row.item
              const grossMargin = item.gross_margin
              return (
                <TableRow key={item.id} data-channel-id={item.id}>
                  <TableCell>
                    <div className='flex flex-col gap-2'>
                      <div className='flex min-w-0 items-center gap-2'>
                        <span className='flex size-5 shrink-0 items-center justify-center'>
                          {getLobeIcon(getChannelTypeIcon(item.type), 18)}
                        </span>
                        <Button
                          variant='link'
                          className='h-auto max-w-[calc(100%-1.75rem)] min-w-0 flex-none justify-start p-0 text-left font-medium whitespace-normal'
                          onClick={() => props.onEditChannel(item.id)}
                        >
                          <span className='block min-w-0 break-words'>
                            {item.name}
                          </span>
                        </Button>
                      </div>
                      <div className='flex flex-wrap items-center gap-1.5'>
                        <GroupBadge
                          group={normalizeGroupName(item.group_name)}
                        />
                        <ChannelStatus status={item.status} />
                        {item.temporary_circuit_open && (
                          <>
                            <Tooltip>
                              <TooltipTrigger
                                render={
                                  <StatusBadge
                                    label={t('Circuit open')}
                                    variant='warning'
                                    copyable={false}
                                  />
                                }
                              />
                              <TooltipContent>
                                <div className='flex flex-col gap-1'>
                                  <span>
                                    {item.temporary_circuit_reason ||
                                      t('Circuit open')}
                                  </span>
                                  <span className='tabular-nums'>
                                    {formatBeijingDateTime(
                                      item.temporary_circuit_until
                                    )}
                                  </span>
                                </div>
                              </TooltipContent>
                            </Tooltip>
                            <Button
                              variant='ghost'
                              size='icon-xs'
                              onClick={() => setCircuitItem(item)}
                              aria-label={t('Clear temporary circuit')}
                              title={t('Clear temporary circuit')}
                            >
                              <HugeiconsIcon
                                icon={RotateIcon}
                                strokeWidth={2}
                              />
                            </Button>
                          </>
                        )}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell className='hidden md:table-cell'>
                    <Input
                      key={`${item.id}-${item.priority}`}
                      type='number'
                      defaultValue={item.priority}
                      min={-999}
                      className='w-20'
                      onBlur={(event) =>
                        commitPriority(item, event.target.value)
                      }
                      onKeyDown={(event) => {
                        if (event.key === 'Enter') event.currentTarget.blur()
                      }}
                      aria-label={t('Priority for {{name}}', {
                        name: item.name,
                      })}
                    />
                  </TableCell>
                  <TableCell className='hidden lg:table-cell'>
                    <Button
                      variant='ghost'
                      className='h-auto justify-start px-2 py-1.5'
                      onClick={() => openScoreDialog(item)}
                    >
                      <ScoreBadge score={item.current_weighted_score} />
                    </Button>
                  </TableCell>
                  <TableCell className='hidden font-medium tabular-nums xl:table-cell'>
                    {grossMargin == null ? '-' : `${grossMargin.toFixed(1)}%`}
                  </TableCell>
                  <TableCell>
                    <AvailabilityTrend
                      item={item}
                      timeline={timelineByChannelId.get(item.id)}
                      onSelectPoint={(point) => setTrendDetail({ item, point })}
                    />
                  </TableCell>
                </TableRow>
              )
            })}
            {!props.loading && props.rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className='h-72'>
                  <Empty>
                    <EmptyHeader>
                      <EmptyMedia variant='icon'>
                        <HugeiconsIcon icon={Activity01Icon} strokeWidth={2} />
                      </EmptyMedia>
                      <EmptyTitle>{t('No channel monitoring data')}</EmptyTitle>
                      <EmptyDescription>
                        {t('Try changing the search or filter conditions')}
                      </EmptyDescription>
                    </EmptyHeader>
                  </Empty>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog
        open={scoreItem !== null}
        onOpenChange={(open) => !open && setScoreItem(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Set routing score')}</DialogTitle>
            <DialogDescription>
              {t(
                'Override the routing selection score used by channel monitoring. This does not change the channel weight.'
              )}
            </DialogDescription>
          </DialogHeader>
          <div className='flex flex-col gap-2'>
            <label
              htmlFor='channel-monitor-score'
              className='text-sm font-medium'
            >
              {t('Routing score')}
            </label>
            <Input
              id='channel-monitor-score'
              type='number'
              min={0}
              max={1}
              step={0.01}
              value={scoreDraft}
              onChange={(event) => setScoreDraft(event.target.value)}
            />
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() =>
                scoreItem &&
                scoreMutation.mutate({ channelId: scoreItem.id, score: null })
              }
              disabled={scoreMutation.isPending}
            >
              {t('Clear override')}
            </Button>
            <Button onClick={saveScore} disabled={scoreMutation.isPending}>
              {t('Save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Sheet
        open={trendDetail !== null}
        onOpenChange={(open) => !open && setTrendDetail(null)}
      >
        <SheetContent className='w-full sm:max-w-xl'>
          <SheetHeader className='border-b pr-12'>
            <SheetTitle>详情</SheetTitle>
            <SheetDescription>
              {trendDetail
                ? `${trendDetail.item.name} · ${formatTimelinePointTime(trendDetail.point)}`
                : ''}
            </SheetDescription>
          </SheetHeader>
          {trendDetail && (
            <div className='flex min-h-0 flex-1 flex-col overflow-y-auto'>
              <div className='grid grid-cols-3 gap-3 border-b px-4 py-3 text-sm tabular-nums'>
                <div>
                  <div className='text-muted-foreground text-xs'>
                    {t('Success rate')}
                  </div>
                  <div className='mt-1 font-medium'>
                    {formatRate(
                      trendDetail.point.request_count > 0
                        ? trendDetail.point.success_count /
                            trendDetail.point.request_count
                        : 0
                    )}
                  </div>
                </div>
                <div>
                  <div className='text-muted-foreground text-xs'>
                    {t('Requests')}
                  </div>
                  <div className='mt-1 font-medium'>
                    {trendDetail.point.request_count.toLocaleString()}
                  </div>
                </div>
                <div>
                  <div className='text-muted-foreground text-xs'>
                    {t('Failure count')}
                  </div>
                  <div className='mt-1 font-medium'>
                    {trendDetail.point.failure_count.toLocaleString()}
                  </div>
                </div>
              </div>
              <section className='border-b px-4 py-4'>
                <div className='mb-3 flex items-center justify-between gap-3'>
                  <h3 className='font-medium'>模型缓存</h3>
                  <span className='text-muted-foreground text-xs'>
                    上游 Usage
                  </span>
                </div>
                {cacheModelQuery.isLoading && (
                  <p className='text-muted-foreground text-sm'>
                    正在加载缓存统计...
                  </p>
                )}
                {!cacheModelQuery.isLoading && cacheModelQuery.isError && (
                  <p className='text-destructive text-sm'>缓存统计加载失败</p>
                )}
                {!cacheModelQuery.isLoading &&
                  !cacheModelQuery.isError &&
                  (cacheModelQuery.data ?? []).length === 0 && (
                    <p className='text-muted-foreground text-sm'>
                      暂无模型缓存数据
                    </p>
                  )}
                {!cacheModelQuery.isLoading &&
                  !cacheModelQuery.isError &&
                  (cacheModelQuery.data ?? []).length > 0 && (
                    <div className='overflow-x-auto'>
                      <table className='w-full min-w-[520px] text-sm'>
                        <thead className='text-muted-foreground text-left text-xs'>
                          <tr>
                            <th className='pr-3 pb-2 font-medium'>模型</th>
                            <th className='pr-3 pb-2 text-right font-medium'>
                              请求
                            </th>
                            <th className='pr-3 pb-2 text-right font-medium'>
                              输入 Token
                            </th>
                            <th className='pr-3 pb-2 text-right font-medium'>
                              缓存 Token
                            </th>
                            <th className='pb-2 text-right font-medium'>
                              缓存率
                            </th>
                          </tr>
                        </thead>
                        <tbody>
                          {(cacheModelQuery.data ?? []).map((stat) => (
                            <tr key={stat.model_name} className='border-t'>
                              <td
                                className='max-w-[220px] truncate py-2 pr-3 font-medium'
                                title={stat.model_name}
                              >
                                {stat.model_name}
                              </td>
                              <td className='py-2 pr-3 text-right tabular-nums'>
                                {formatNumber(stat.request_count)}
                              </td>
                              <td className='py-2 pr-3 text-right tabular-nums'>
                                {formatNumber(stat.input_tokens)}
                              </td>
                              <td className='py-2 pr-3 text-right tabular-nums'>
                                {formatNumber(stat.cached_tokens)}
                              </td>
                              <td className='py-2 text-right tabular-nums'>
                                {stat.cache_rate == null
                                  ? '-'
                                  : formatRate(stat.cache_rate)}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
              </section>
              {(trendDetail.point.failure_reasons ?? []).length === 0 ? (
                <div className='px-4 py-10 text-center text-sm'>
                  <p className='text-muted-foreground'>
                    {t('No failure details recorded')}
                  </p>
                  <p className='text-muted-foreground/70 mt-2 text-xs'>
                    {t(
                      'No aggregated error sample was retained for this time range'
                    )}
                  </p>
                </div>
              ) : (
                (trendDetail.point.failure_reasons ?? []).map((reason) => (
                  <section
                    key={`${reason.model_name ?? ''}-${reason.reason}-${reason.status_code}`}
                    className='border-b px-4 py-4 last:border-b-0'
                  >
                    <div className='flex items-start justify-between gap-3'>
                      <div className='min-w-0'>
                        <p className='text-muted-foreground mb-1 text-xs'>
                          {reason.model_name || '未知模型'}
                        </p>
                        <h3 className='truncate font-medium'>
                          {reason.reason}
                        </h3>
                        <p className='text-muted-foreground mt-1 text-xs'>
                          {[reason.error_type, reason.status_code || null]
                            .filter(Boolean)
                            .join(' · ')}
                        </p>
                      </div>
                      <StatusBadge
                        label={reason.count.toLocaleString()}
                        variant='danger'
                        copyable={false}
                      />
                    </div>
                    <div className='mt-3 space-y-3'>
                      {reason.details.map((detail) => (
                        <div
                          key={`${detail.occurred_at}-${detail.request_id}`}
                          className='border-l-2 pl-3'
                        >
                          <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs tabular-nums'>
                            <span>
                              {formatBeijingDateTime(detail.occurred_at)}
                            </span>
                            {detail.request_id && (
                              <span>{detail.request_id}</span>
                            )}
                          </div>
                          <p className='mt-1 text-sm break-words'>
                            {detail.message || reason.reason}
                          </p>
                        </div>
                      ))}
                    </div>
                  </section>
                ))
              )}
            </div>
          )}
        </SheetContent>
      </Sheet>

      <ConfirmDialog
        open={circuitItem !== null}
        onOpenChange={(open) => !open && setCircuitItem(null)}
        title={t('Clear temporary circuit')}
        desc={t(
          'Are you sure you want to clear the temporary circuit for this channel?'
        )}
        confirmText={t('Clear')}
        isLoading={circuitMutation.isPending}
        handleConfirm={() =>
          circuitItem && circuitMutation.mutate(circuitItem.id)
        }
      />
    </section>
  )
}
