/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: 运营中心 - 监控视图(整合成本数据)

import { ArrowDown01Icon, ArrowUp01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { lazy, Suspense, useMemo, useState } from 'react'

import { StatusBadge } from '@/components/status-badge'
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
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { getChannelMonitorChannels, getChannelMonitorTimeline } from '../api'
import { ChannelMonitorTable } from '../channel-monitor-table'
import {
  buildChannelMonitorRows,
  getChannelMonitorPeriodDateRange,
  getChannelMonitorPeriodDays,
  matchesChannelMonitorKeyword,
  normalizeGroupName,
  type ChannelMonitorPeriod,
  type ChannelMonitorSortKey,
  type ChannelMonitorSortOrder,
} from '../channel-monitor-utils'
import { getProviderWorkspace } from '../provider-management/api'
import type { ProviderProfitDateRange } from '../provider-management/types'

const DAY_OPTIONS = [
  { value: 1, label: '今天' },
  { value: 2, label: '昨天' },
  { value: 7, label: '近7天' },
  { value: 30, label: '近30天' },
] as const

const MONTH_OPTION = { value: 'month', label: '本月' } as const

const ALL_GROUPS = '__all__'

const SORT_OPTIONS: Array<{ labelKey: string; value: ChannelMonitorSortKey }> =
  [
    { labelKey: '优先级', value: 'priority' },
    { labelKey: '路由分数', value: 'routing_score' },
    { labelKey: '毛利率', value: 'gross_margin' },
    { labelKey: '稳定性', value: 'success_rate' },
    { labelKey: '平均延迟', value: 'avg_latency' },
    { labelKey: 'P95延迟', value: 'p95_latency' },
    { labelKey: '失败次数', value: 'failure_count' },
    { labelKey: '最后活跃', value: 'last_active' },
    { labelKey: '名称', value: 'name' },
    { labelKey: '请求次数', value: 'request_count' },
    { labelKey: '渠道分组', value: 'group_name' },
    { labelKey: '分组成功率', value: 'group_success_rate' },
  ]

const STATUS_OPTIONS = [
  { labelKey: '全部状态', value: 'all' },
  { labelKey: '已启用', value: '1' },
  { labelKey: '已禁用', value: '2' },
  { labelKey: '自动禁用', value: '3' },
] as const

const ChannelsProvider = lazy(() =>
  import('@/features/channels/components/channels-provider').then((module) => ({
    default: module.ChannelsProvider,
  }))
)

const ChannelMutateDrawer = lazy(() =>
  import('@/features/channels/components/drawers/channel-mutate-drawer').then(
    (module) => ({ default: module.ChannelMutateDrawer })
  )
)

export function OperationsMonitoring(props: {
  highlightChannelId: number | null
  onHighlighted: () => void
}) {
  const queryClient = useQueryClient()
  const [period, setPeriod] = useState<ChannelMonitorPeriod>(1)
  const [editingChannelId, setEditingChannelId] = useState<number | null>(null)

  // 筛选和排序状态
  const [keyword, setKeyword] = useState('')
  const [statusFilter, setStatusFilter] = useState('1')
  const [groupFilter, setGroupFilter] = useState(ALL_GROUPS)
  const [sortKey, setSortKey] = useState<ChannelMonitorSortKey>('priority')
  const [sortOrder, setSortOrder] = useState<ChannelMonitorSortOrder>('desc')
  const [grouped, setGrouped] = useState(false)

  const days = getChannelMonitorPeriodDays(period)
  const dateRange: ProviderProfitDateRange =
    getChannelMonitorPeriodDateRange(period)

  const channelsQuery = useQuery({
    queryKey: ['channel-monitor', 'channels', days],
    queryFn: () => getChannelMonitorChannels(days, ''),
  })

  const timelineQuery = useQuery({
    queryKey: ['channel-monitor', 'timeline', days],
    queryFn: () => getChannelMonitorTimeline(days, ''),
  })

  const workspaceQuery = useQuery({
    queryKey: ['upstream-providers', dateRange.startDate, dateRange.endDate],
    queryFn: () => getProviderWorkspace(dateRange),
  })

  const refreshMonitor = () =>
    queryClient.invalidateQueries({ queryKey: ['channel-monitor'] })

  // 构建渠道ID到成本数据的映射
  const channelCostMap = new Map<
    number,
    {
      providerName: string
      accountName: string
      groupName: string
      revenue: number
      cost: number
      profit: number
      grossMargin: number
    }
  >()

  workspaceQuery.data?.providers.forEach((provider) => {
    provider.accounts.forEach((account) => {
      account.groups.forEach((group) => {
        group.channels.forEach((channel) => {
          if (group.profit) {
            // 简化版: 平均分配Group利润到每个渠道
            const channelCount = group.channels.length
            channelCostMap.set(channel.id, {
              providerName: provider.name,
              accountName: account.name,
              groupName: group.name,
              revenue: (group.profit.revenue || 0) / channelCount,
              cost: (group.profit.cost || 0) / channelCount,
              profit: (group.profit.profit || 0) / channelCount,
              grossMargin: group.profit.grossMargin || 0,
            })
          }
        })
      })
    })
  })

  // 合并监控数据和成本数据
  const enrichedChannels = (channelsQuery.data?.items ?? []).map((channel) => {
    const costData = channelCostMap.get(channel.id)
    return {
      ...channel,
      // 添加成本相关字段
      provider_name: costData?.providerName,
      accountName: costData?.accountName,
      costGroupName: costData?.groupName,
      revenue: costData?.revenue,
      cost: costData?.cost,
      profit: costData?.profit,
      costGrossMargin: costData?.grossMargin,
    }
  })

  // 计算分组统计
  const groupCounts = useMemo(() => {
    const counts = new Map<string, number>()
    for (const item of enrichedChannels) {
      // 只统计符合状态筛选的渠道
      if (statusFilter !== 'all' && String(item.status) !== statusFilter) {
        continue
      }
      const groupName = normalizeGroupName(item.group_name)
      counts.set(groupName, (counts.get(groupName) ?? 0) + 1)
    }
    return counts
  }, [enrichedChannels, statusFilter])

  // 计算总数（也要根据状态筛选）
  const totalCount = useMemo(() => {
    if (statusFilter === 'all') return enrichedChannels.length
    return enrichedChannels.filter(
      (item) => String(item.status) === statusFilter
    ).length
  }, [enrichedChannels, statusFilter])

  const availableGroups = [...groupCounts.keys()].sort((left, right) =>
    left.localeCompare(right)
  )
  const effectiveGroupFilter = groupCounts.has(groupFilter)
    ? groupFilter
    : ALL_GROUPS

  // 应用所有筛选和排序
  const displayRows = useMemo(() => {
    const filteredItems = enrichedChannels.filter((item) => {
      if (
        effectiveGroupFilter !== ALL_GROUPS &&
        normalizeGroupName(item.group_name) !== effectiveGroupFilter
      ) {
        return false
      }
      if (statusFilter !== 'all' && String(item.status) !== statusFilter) {
        return false
      }
      if (!matchesChannelMonitorKeyword(item, keyword)) {
        return false
      }
      return true
    })
    const groupRows =
      grouped || sortKey === 'group_name' || sortKey === 'group_success_rate'
    return buildChannelMonitorRows(filteredItems, sortKey, sortOrder, groupRows)
  }, [
    enrichedChannels,
    effectiveGroupFilter,
    statusFilter,
    keyword,
    grouped,
    sortKey,
    sortOrder,
  ])

  return (
    <>
      <div className='space-y-4'>
        {/* 时间条件工具栏 */}
        <div className='flex items-center gap-2'>
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
          <Button
            variant='outline'
            size='icon'
            onClick={() => refreshMonitor()}
            aria-label='刷新'
            title='刷新'
          >
            <RefreshCw className='size-4' />
          </Button>
        </div>

        {/* 查询条件区域 */}
        <section className='space-y-3'>
          <div className='flex flex-wrap items-center gap-2'>
            <Input
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              placeholder='搜索渠道名称、类型、分组或供应商'
              className='w-full sm:w-52'
            />
            <Select
              items={STATUS_OPTIONS.map((option) => ({
                value: option.value,
                label: option.labelKey,
              }))}
              value={statusFilter}
              onValueChange={(value) => value && setStatusFilter(value)}
            >
              <SelectTrigger aria-label='状态' className='w-40'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {STATUS_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.labelKey}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <Select
              items={SORT_OPTIONS.map((option) => ({
                value: option.value,
                label: option.labelKey,
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
              <SelectTrigger aria-label='排序' className='w-44'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {SORT_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.labelKey}
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
              aria-label={sortOrder === 'asc' ? '升序' : '降序'}
            >
              <HugeiconsIcon
                icon={sortOrder === 'asc' ? ArrowUp01Icon : ArrowDown01Icon}
                strokeWidth={2}
                data-icon='inline-start'
              />
              {sortOrder === 'asc' ? '升序' : '降序'}
            </Button>
            <label className='text-muted-foreground flex items-center gap-2 text-sm'>
              <Switch checked={grouped} onCheckedChange={setGrouped} />
              按分组显示
            </label>
          </div>
          <div className='overflow-x-auto pb-1'>
            <Tabs
              value={effectiveGroupFilter}
              onValueChange={(value) => setGroupFilter(value)}
            >
              <TabsList>
                <TabsTrigger value={ALL_GROUPS}>
                  所有分组
                  <StatusBadge
                    label={String(totalCount)}
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
        </section>

        {/* 监控表格 */}
        <ChannelMonitorTable
          rows={displayRows}
          timeline={timelineQuery.data ?? []}
          loading={channelsQuery.isLoading}
          onEditChannel={setEditingChannelId}
          profitDateRange={dateRange}
          highlightChannelId={props.highlightChannelId}
          onHighlighted={props.onHighlighted}
        />

        {/* 提示信息 */}
        <div className='text-muted-foreground rounded-lg border border-dashed px-4 py-3 text-sm'>
          💡 提示:
          表格中的成本和利润数据来自供应商管理。点击渠道名展开可查看完整成本明细。
        </div>
      </div>

      {/* 编辑渠道抽屉:抽屉依赖 ChannelsProvider 上下文,需要自行包裹 */}
      {editingChannelId !== null && (
        <Suspense fallback={null}>
          <ChannelsProvider>
            <ChannelMutateDrawer
              open
              onOpenChange={(open) => {
                if (open) return
                setEditingChannelId(null)
                refreshMonitor()
              }}
              currentRow={{ id: editingChannelId }}
            />
          </ChannelsProvider>
        </Suspense>
      )}
    </>
  )
}
