/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Render the full channel health workspace with default-theme components.

import {
  Activity01Icon,
  ArrowDown01Icon,
  ArrowUp01Icon,
  RotateIcon,
  UnfoldMoreIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
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
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatRelativeTime, getChannelTypeIcon } from '@/features/channels/lib'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import {
  clearChannelCircuit,
  setChannelScore,
  updateChannelPriority,
} from './api'
import {
  buildChannelMonitorRows,
  formatLatency,
  formatRate,
  getSuggestedWeightScore,
  normalizeGroupName,
  type ChannelMonitorSortKey,
  type ChannelMonitorSortOrder,
} from './channel-monitor-utils'
import type {
  ChannelMonitorItem,
  ChannelTimeline,
  ChannelTimelinePoint,
} from './types'

const ALL_GROUPS = '__all__'

const SORT_OPTIONS: Array<{ labelKey: string; value: ChannelMonitorSortKey }> =
  [
    { labelKey: 'Priority', value: 'priority' },
    { labelKey: 'Routing score', value: 'routing_score' },
    { labelKey: 'Stability', value: 'success_rate' },
    { labelKey: 'Average latency', value: 'avg_latency' },
    { labelKey: 'P95 latency', value: 'p95_latency' },
    { labelKey: 'Failure count', value: 'failure_count' },
    { labelKey: 'Last active', value: 'last_active' },
    { labelKey: 'Name', value: 'name' },
    { labelKey: 'Request count', value: 'request_count' },
    { labelKey: 'Channel group', value: 'group_name' },
    { labelKey: 'Group success rate', value: 'group_success_rate' },
  ]

const STATUS_OPTIONS = [
  { labelKey: 'All Status', value: 'all' },
  { labelKey: 'Enabled', value: '1' },
  { labelKey: 'Disabled', value: '2' },
  { labelKey: 'Automatically disabled', value: '3' },
] as const

function SortableTableHead(props: {
  label: string
  sortKey: ChannelMonitorSortKey
  activeSortKey: ChannelMonitorSortKey
  sortOrder: ChannelMonitorSortOrder
  className?: string
  onSort: (sortKey: ChannelMonitorSortKey) => void
}) {
  const active = props.sortKey === props.activeSortKey
  let ariaSort: 'ascending' | 'descending' | 'none' = 'none'
  if (active) ariaSort = props.sortOrder === 'asc' ? 'ascending' : 'descending'
  let sortIcon = UnfoldMoreIcon
  if (active) {
    sortIcon = props.sortOrder === 'asc' ? ArrowUp01Icon : ArrowDown01Icon
  }

  return (
    <TableHead className={props.className} aria-sort={ariaSort}>
      <Button
        variant='ghost'
        size='sm'
        className='-ml-3 h-8 px-3'
        onClick={() => props.onSort(props.sortKey)}
      >
        {props.label}
        <HugeiconsIcon
          icon={sortIcon}
          strokeWidth={2}
          data-icon='inline-end'
        />
      </Button>
    </TableHead>
  )
}

function ChannelStatus(props: { status: number }) {
  const { t } = useTranslation()
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

function getTrendPointClassName(
  point: ChannelTimelinePoint,
  maxRequests: number
): string {
  if (point.request_count <= 0) return 'bg-muted-foreground/30 size-1'
  const ratio = maxRequests > 0 ? point.request_count / maxRequests : 0
  let sizeClass = 'size-1'
  if (ratio >= 0.66) sizeClass = 'size-2'
  else if (ratio >= 0.33) sizeClass = 'size-1.5'
  const stateClass = point.failure_count > 0 ? 'bg-destructive' : 'bg-success'
  return cn('shrink-0 rounded-full', sizeClass, stateClass)
}

function AvailabilityTrend(props: {
  item: ChannelMonitorItem
  timeline?: ChannelTimeline
  locale: string
}) {
  const { t } = useTranslation()
  const points = props.timeline?.points ?? []
  let maxRequests = 0
  for (const point of points) {
    maxRequests = Math.max(maxRequests, point.request_count)
  }
  const hasData = maxRequests > 0
  const lastActiveMs = Date.parse(props.item.last_active)
  let lastActive = '-'
  if (Number.isFinite(lastActiveMs) && lastActiveMs > 0) {
    const formattedLastActive = formatRelativeTime(
      Math.floor(lastActiveMs / 1000),
      props.locale
    )
    if (formattedLastActive !== 'Unknown' && formattedLastActive !== 'Never') {
      lastActive = formattedLastActive
    }
  }

  return (
    <div className='flex min-w-72 flex-col gap-2'>
      <TooltipProvider>
        <div className='flex min-h-2 items-center gap-1'>
          {hasData ? (
            points.map((point) => (
              <Tooltip key={point.time_bucket}>
                <TooltipTrigger
                  render={
                    <span
                      className={getTrendPointClassName(point, maxRequests)}
                    />
                  }
                />
                <TooltipContent>
                  <div className='flex flex-col gap-1 tabular-nums'>
                    <span>{point.time_bucket}</span>
                    <span>
                      {t('Success rate')}:{' '}
                      {formatRate(
                        point.request_count > 0
                          ? point.success_count / point.request_count
                          : 0
                      )}
                    </span>
                    <span>
                      {t('Requests')}: {point.request_count.toLocaleString()}
                    </span>
                    <span>
                      {t('Failure count')}:{' '}
                      {point.failure_count.toLocaleString()}
                    </span>
                  </div>
                </TooltipContent>
              </Tooltip>
            ))
          ) : (
            <span className='text-muted-foreground'>-</span>
          )}
        </div>
      </TooltipProvider>
      <div className='text-muted-foreground flex flex-wrap gap-x-2 gap-y-1 text-xs tabular-nums'>
        <span>{formatRate(props.item.success_rate)}</span>
        <span>P95 {formatLatency(props.item.p95_latency)}</span>
        <span>
          {t('Requests {{requests}} / failures {{failures}}', {
            requests: props.item.request_count.toLocaleString(),
            failures: props.item.failure_count.toLocaleString(),
          })}
        </span>
        <span>{lastActive}</span>
      </div>
    </div>
  )
}

type ChannelMonitorTableProps = {
  items: ChannelMonitorItem[]
  timeline: ChannelTimeline[]
  loading: boolean
  onEditChannel: (channelId: number) => void
}

export function ChannelMonitorTable(props: ChannelMonitorTableProps) {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [statusFilter, setStatusFilter] = useState('1')
  const [groupFilter, setGroupFilter] = useState(ALL_GROUPS)
  const [sortKey, setSortKey] = useState<ChannelMonitorSortKey>('priority')
  const [sortOrder, setSortOrder] = useState<ChannelMonitorSortOrder>('desc')
  const [grouped, setGrouped] = useState(false)
  const [scoreItem, setScoreItem] = useState<ChannelMonitorItem | null>(null)
  const [scoreDraft, setScoreDraft] = useState('')
  const [circuitItem, setCircuitItem] = useState<ChannelMonitorItem | null>(
    null
  )

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

  const groupCounts = useMemo(() => {
    const counts = new Map<string, number>()
    for (const item of props.items) {
      const groupName = normalizeGroupName(item.group_name)
      counts.set(groupName, (counts.get(groupName) ?? 0) + 1)
    }
    return counts
  }, [props.items])
  const availableGroups = [...groupCounts.keys()].sort((left, right) =>
    left.localeCompare(right)
  )
  const effectiveGroupFilter = groupCounts.has(groupFilter)
    ? groupFilter
    : ALL_GROUPS

  const displayRows = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase()
    const filteredItems: ChannelMonitorItem[] = []
    for (const item of props.items) {
      if (
        effectiveGroupFilter !== ALL_GROUPS &&
        normalizeGroupName(item.group_name) !== effectiveGroupFilter
      ) {
        continue
      }
      if (statusFilter !== 'all' && String(item.status) !== statusFilter) {
        continue
      }
      if (
        normalizedKeyword &&
        !item.name.toLowerCase().includes(normalizedKeyword)
      ) {
        continue
      }
      filteredItems.push(item)
    }
    const groupRows =
      grouped || sortKey === 'group_name' || sortKey === 'group_success_rate'
    return buildChannelMonitorRows(filteredItems, sortKey, sortOrder, groupRows)
  }, [
    effectiveGroupFilter,
    grouped,
    keyword,
    props.items,
    sortKey,
    sortOrder,
    statusFilter,
  ])

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

  const sortByColumn = (nextSortKey: ChannelMonitorSortKey) => {
    if (sortKey === nextSortKey) {
      setSortOrder((current) => (current === 'asc' ? 'desc' : 'asc'))
      return
    }
    setSortKey(nextSortKey)
    setSortOrder('desc')
  }

  const openScoreDialog = (item: ChannelMonitorItem) => {
    setScoreItem(item)
    setScoreDraft(getSuggestedWeightScore(item).toFixed(2))
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
      <header className='flex flex-col gap-3 border-b px-4 py-3'>
        <div>
          <h2 className='font-semibold'>{t('Channel health')}</h2>
          <p className='text-muted-foreground mt-0.5 text-xs'>
            {t('Inspect routing health and adjust channel selection')}
          </p>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <Input
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            placeholder={t('Search channels')}
            className='w-full sm:w-52'
          />
          <Select
            items={STATUS_OPTIONS.map((option) => ({
              value: option.value,
              label: t(option.labelKey),
            }))}
            value={statusFilter}
            onValueChange={(value) => value && setStatusFilter(value)}
          >
            <SelectTrigger aria-label={t('Status')} className='w-40'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {STATUS_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {t(option.labelKey)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Select
            items={SORT_OPTIONS.map((option) => ({
              value: option.value,
              label: t(option.labelKey),
            }))}
            value={sortKey}
            onValueChange={(value) => {
              if (!value) return
              const nextSortKey = value as ChannelMonitorSortKey
              setSortKey(nextSortKey)
              if (
                nextSortKey === 'group_name' ||
                nextSortKey === 'group_success_rate'
              ) {
                setGrouped(true)
              }
            }}
          >
            <SelectTrigger aria-label={t('Sort')} className='w-44'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {SORT_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {t(option.labelKey)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Button
            variant='outline'
            onClick={() =>
              setSortOrder((current) => (current === 'asc' ? 'desc' : 'asc'))
            }
            aria-label={t(sortOrder === 'asc' ? 'Ascending' : 'Descending')}
          >
            <HugeiconsIcon
              icon={sortOrder === 'asc' ? ArrowUp01Icon : ArrowDown01Icon}
              strokeWidth={2}
              data-icon='inline-start'
            />
            {t(sortOrder === 'asc' ? 'Ascending' : 'Descending')}
          </Button>
          <label className='text-muted-foreground flex items-center gap-2 text-sm'>
            <Switch checked={grouped} onCheckedChange={setGrouped} />
            {t('Group channels')}
          </label>
        </div>
        <div className='overflow-x-auto pb-1'>
          <Tabs
            value={effectiveGroupFilter}
            onValueChange={(value) => setGroupFilter(value)}
          >
            <TabsList>
              <TabsTrigger value={ALL_GROUPS}>
                {t('All groups')}
                <StatusBadge
                  label={String(props.items.length)}
                  variant='neutral'
                  copyable={false}
                />
              </TabsTrigger>
              {availableGroups.map((groupName) => (
                <TabsTrigger key={groupName} value={groupName}>
                  {groupName}
                  <StatusBadge
                    label={String(groupCounts.get(groupName) ?? 0)}
                    autoColor={groupName}
                    copyable={false}
                  />
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>
      </header>

      <div className='overflow-x-auto'>
        <Table className='min-w-[980px]'>
          <TableHeader>
            <TableRow>
              <TableHead className='w-72'>{t('Channel')}</TableHead>
              <SortableTableHead
                label={t('Priority')}
                sortKey='priority'
                activeSortKey={sortKey}
                sortOrder={sortOrder}
                className='w-32'
                onSort={sortByColumn}
              />
              <SortableTableHead
                label={t('Routing score')}
                sortKey='routing_score'
                activeSortKey={sortKey}
                sortOrder={sortOrder}
                className='w-44'
                onSort={sortByColumn}
              />
              <SortableTableHead
                label={t('Availability trend')}
                sortKey='success_rate'
                activeSortKey={sortKey}
                sortOrder={sortOrder}
                onSort={sortByColumn}
              />
            </TableRow>
          </TableHeader>
          <TableBody>
            {displayRows.map((row) => {
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
                    <TableCell>-</TableCell>
                    <TableCell>-</TableCell>
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
              const suggestedScore = getSuggestedWeightScore(item)
              return (
                <TableRow key={item.id}>
                  <TableCell>
                    <div className='flex flex-col gap-2'>
                      <div className='flex items-center gap-2'>
                        <span className='flex size-5 shrink-0 items-center justify-center'>
                          {getLobeIcon(getChannelTypeIcon(item.type), 18)}
                        </span>
                        <Button
                          variant='link'
                          className='h-auto min-w-0 justify-start p-0 font-medium'
                          onClick={() => props.onEditChannel(item.id)}
                        >
                          <span className='truncate'>{item.name}</span>
                        </Button>
                      </div>
                      <div className='flex flex-wrap items-center gap-1.5'>
                        {effectiveGroupFilter === ALL_GROUPS && (
                          <GroupBadge
                            group={normalizeGroupName(item.group_name)}
                          />
                        )}
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
                                {item.temporary_circuit_reason ||
                                  t('Circuit open')}
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
                  <TableCell>
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
                  <TableCell>
                    <Button
                      variant='ghost'
                      className='h-auto flex-col items-start gap-1.5 px-2 py-1.5'
                      onClick={() => openScoreDialog(item)}
                    >
                      <span className='flex items-center gap-2'>
                        <span className='text-muted-foreground w-16 text-right text-xs'>
                          {t('Current:')}
                        </span>
                        <span className='tabular-nums'>
                          <ScoreBadge score={item.current_weighted_score} />
                        </span>
                      </span>
                      <span className='flex items-center gap-2'>
                        <span className='text-muted-foreground w-16 text-right text-xs'>
                          {t('Suggested')}
                        </span>
                        <span className='tabular-nums'>
                          <ScoreBadge score={suggestedScore} />
                        </span>
                      </span>
                    </Button>
                  </TableCell>
                  <TableCell>
                    <AvailabilityTrend
                      item={item}
                      timeline={timelineByChannelId.get(item.id)}
                      locale={i18n.resolvedLanguage ?? i18n.language}
                    />
                  </TableCell>
                </TableRow>
              )
            })}
            {!props.loading && displayRows.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className='h-72'>
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
