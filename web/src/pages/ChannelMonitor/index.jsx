/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Switch,
  TabPane,
  Tabs,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { RefreshCw } from 'lucide-react';
import {
  getChannelIcon,
  renderGroup,
  renderNumber,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import {
  fetchChannelMonitorChannels,
  fetchChannelMonitorHealth,
  fetchChannelMonitorSummary,
  fetchChannelMonitorTimeline,
  setChannelScoreOverride,
  updateChannelPriority,
} from '../../helpers/channelMonitor';
import EditChannelModal from '../../components/table/channels/modals/EditChannelModal';
import ChannelTable from './ChannelTable';

const { Text } = Typography;

const DAY_OPTIONS = [
  { label: '24h', value: 1 },
  { label: '7d', value: 7 },
  { label: '14d', value: 14 },
  { label: '30d', value: 30 },
];

const SORT_OPTIONS = [
  { label: '稳定性', value: 'success_rate' },
  { label: '平均延迟', value: 'avg_latency' },
  { label: 'P95 延迟', value: 'p95_latency' },
  { label: '失败数', value: 'failure_count' },
  { label: '最近活跃时间', value: 'last_active' },
  { label: '名称', value: 'name' },
  { label: '请求数', value: 'request_count' },
  { label: '渠道分组', value: 'group_name' },
  { label: '组内成功率', value: 'group_success_rate' },
];

const STATUS_FILTER_OPTIONS = [
  { label: '全部状态', value: 'all' },
  { label: '已启用', value: '1' },
  { label: '已禁用', value: '2' },
  { label: '自动禁用', value: '3' },
];

const GROUP_FILTER_ALL = '__all__';

const TIMELINE_HOURS = 24;
const TIMELINE_BUCKET_MINUTES = 10;
const TIMELINE_LIMIT = 20;

const renderChannelStatus = (status) => {
  switch (status) {
    case 1:
      return (
        <Tag color='green' shape='circle'>
          {'已启用'}
        </Tag>
      );
    case 2:
      return (
        <Tag color='red' shape='circle'>
          {'已禁用'}
        </Tag>
      );
    case 3:
      return (
        <Tag color='yellow' shape='circle'>
          {'自动禁用'}
        </Tag>
      );
    default:
      return (
        <Tag color='grey' shape='circle'>
          {'未知状态'}
        </Tag>
      );
  }
};

const formatPercentage = (value, digits = 1) =>
  `${(Number(value || 0) * 100).toFixed(digits)}%`;

const formatLatencyValue = (value) => {
  const latency = Number(value || 0);
  if (latency <= 0) {
    return '--';
  }
  return `${latency.toFixed(0)}ms`;
};

const formatRequestSummary = (requestCount, failureCount) => {
  return `请求 ${renderNumber(requestCount || 0)} / 失败 ${renderNumber(failureCount || 0)}`;
};

const formatTimeRange = (timeRange, days) => {
  if (timeRange === 'last_24h' || days === 1) {
    return '24 小时';
  }
  const matched = String(timeRange || '').match(/^last_(\d+)d$/);
  if (matched) {
    return `${matched[1]} 天`;
  }
  return `${days} 天`;
};

const getAvailabilityMeta = (successRate, errorChannels) => {
  if (Number(successRate || 0) >= 0.98 && Number(errorChannels || 0) <= 0) {
    return { label: '稳定', color: 'green' };
  }
  if (Number(successRate || 0) < 0.9 || Number(errorChannels || 0) > 0) {
    return { label: '需处理', color: 'red' };
  }
  return { label: '关注', color: 'orange' };
};

const buildActionSuggestion = (summary, latestHealth) => {
  const requestCount = Number(summary?.total_requests || 0);
  const successRate = Number(summary?.success_rate || 0);
  const activeChannels = Number(summary?.active_channels || 0);
  const totalChannels = Number(summary?.total_channels || 0);
  const errorChannels = Number(latestHealth?.error_channels || 0);
  const warningChannels = Number(latestHealth?.warning_channels || 0);

  if (requestCount <= 0) {
    return '暂无足够请求样本，先观察实际流量或检查监控统计是否正常写入。';
  }
  if (successRate < 0.9) {
    return '优先排查失败数最高和成功率最低的渠道，必要时临时降权或禁用异常渠道。';
  }
  if (errorChannels > 0) {
    return '切到失败数排序，优先处理异常渠道；必要时启用自动禁用或降低路由评分。';
  }
  if (totalChannels > 0 && activeChannels / totalChannels < 0.3) {
    return '检查是否存在渠道长时间无流量、分组配置不均或路由过于集中。';
  }
  if (successRate >= 0.98 && errorChannels <= 0) {
    return '当前整体稳定，继续观察失败数突增和异常渠道变化即可。';
  }
  if (warningChannels > 0) {
    return '关注预警渠道的失败数变化，必要时调整路由评分。';
  }
  return '当前整体表现正常，建议保持现有路由策略并持续观察。';
};

const renderMetricValue = (loading, value) => (loading ? '--' : value);

const renderScoreTag = (score) => {
  const value = Number(score || 0);
  let color = 'grey';
  if (value >= 0.85) {
    color = 'green';
  } else if (value >= 0.65) {
    color = 'lime';
  } else if (value >= 0.4) {
    color = 'yellow';
  } else if (value > 0) {
    color = 'red';
  }
  return (
    <Tag color={color} shape='circle'>
      {value.toFixed(2)}
    </Tag>
  );
};

const getTimelineColor = (point) => {
  const requestCount = Number(point?.request_count || 0);
  const successCount = Number(point?.success_count || 0);
  const failureCount = Number(point?.failure_count || 0);
  if (requestCount <= 0) {
    return null;
  }
  if (failureCount <= 0) {
    return '#10b981';
  }
  if (successCount <= 0) {
    return '#f43f5e';
  }
  return '#f43f5e';
};

const getSparklinePoints = (points = []) => {
  const activePoints = points.filter(
    (point) => Number(point?.request_count || 0) > 0,
  );
  const maxRequestCount = activePoints.reduce(
    (max, point) => Math.max(max, Number(point?.request_count || 0)),
    0,
  );

  return points.map((point) => {
    const requestCount = Number(point?.request_count || 0);
    const color = getTimelineColor(point);
    if (requestCount <= 0 || !color) {
      return {
        ...point,
        active: false,
        color: 'var(--semi-color-text-3)',
        size: 4,
      };
    }
    const ratio = maxRequestCount > 0 ? requestCount / maxRequestCount : 0;
    return {
      ...point,
      active: true,
      color,
      size: ratio >= 0.66 ? 8 : ratio >= 0.33 ? 6 : 5,
    };
  });
};

const hasWeightScore = (score) =>
  score !== null && score !== undefined && score !== '';

const getSuggestedWeightScore = (record) => {
  if (record.temporary_circuit_open) {
    return 0.05;
  }
  const successRate = Math.max(
    0,
    Math.min(1, Number(record.success_rate || 0)),
  );
  const avgLatency = Number(record.avg_latency || 0);
  const p95Latency = Number(record.p95_latency || 0);
  let latencyScore = 0.5;
  if (avgLatency > 0) {
    latencyScore = Math.max(0, Math.min(1, 1 - avgLatency / 6000));
  }
  let p95Penalty = 0;
  if (p95Latency > 0) {
    p95Penalty = Math.max(0, Math.min(0.35, (p95Latency - 1500) / 10000));
  }
  let score = successRate * 0.7 + latencyScore * 0.3 - p95Penalty;
  if (successRate < 0.7) {
    score -= 0.15;
  }
  if (avgLatency > 5000) {
    score -= 0.1;
  }
  return Math.max(0.05, Math.min(1, score));
};

const getSuggestion = (record) => {
  const suggestedWeightScore = getSuggestedWeightScore(record);
  if (record.temporary_circuit_open) {
    return {
      text: '建议停用',
      color: 'red',
      score: suggestedWeightScore,
      detail:
        record.temporary_circuit_reason ||
        '渠道处于临时熔断中，建议停用或大幅降权。',
    };
  }
  if (
    Number(record.success_rate || 0) < 0.7 ||
    Number(record.p95_latency || 0) > 5000
  ) {
    return {
      text: '建议降权',
      color: 'orange',
      score: suggestedWeightScore,
      detail: '成功率偏低或 P95 延迟偏高，建议下调权重。',
    };
  }
  if (
    Number(record.success_rate || 0) >= 0.9 &&
    Number(record.avg_latency || 0) > 0 &&
    Number(record.avg_latency || 0) <= 1000
  ) {
    return {
      text: '建议提权',
      color: 'green',
      score: suggestedWeightScore,
      detail: '渠道成功率高且延迟低，建议适度提权。',
    };
  }
  return {
    text: '保持当前',
    color: 'grey',
    score: suggestedWeightScore,
    detail: '当前表现稳定，可保持现有权重。',
  };
};

const AvailabilityTrend = ({ item, loading = false, lastActive }) => {
  const points = item?.points || [];
  const sparklinePoints = getSparklinePoints(points);
  const hasData = sparklinePoints.some((point) => point.active);

  if (loading && points.length === 0) {
    return <Text type='secondary'>{'加载中...'}</Text>;
  }

  const trendSummaryText = hasData
    ? `${formatPercentage(item?.success_rate)} · P95 ${formatLatencyValue(item?.p95_latency)} · ${formatRequestSummary(
        item?.request_count,
        item?.failure_count,
      )}`
    : `${formatPercentage(item?.success_rate)} · P95 -- · ${formatRequestSummary(
        item?.request_count,
        item?.failure_count,
      )}`;
  let lastActiveText = '--';
  if (lastActive) {
    const ts = Math.floor(new Date(lastActive).getTime() / 1000);
    if (ts && !Number.isNaN(ts)) {
      lastActiveText = timestamp2string(ts);
    }
  }

  return (
    <div className='flex min-w-[300px] flex-col items-start gap-2'>
      <div className='flex min-w-[120px] items-center justify-start gap-1'>
        {hasData ? (
          sparklinePoints.map((point) => (
            <Tooltip
              key={`${item?.channel_id || 'channel'}-${point.time_bucket}`}
              content={
                <div className='text-xs leading-5'>
                  <div>{point.time_bucket || '--'}</div>
                  <div>{`成功率 ${formatPercentage(
                    Number(point.request_count || 0) > 0
                      ? Number(point.success_count || 0) /
                          Number(point.request_count || 0)
                      : 0,
                  )}`}</div>
                  <div>{`请求 ${renderNumber(point.request_count || 0)}`}</div>
                  <div>{`失败 ${renderNumber(point.failure_count || 0)}`}</div>
                </div>
              }
              position='top'
            >
              <span
                className={`inline-block rounded-full ${point.active ? '' : 'opacity-70'}`}
                style={{
                  width: point.size,
                  height: point.size,
                  backgroundColor: point.color,
                }}
              />
            </Tooltip>
          ))
        ) : (
          <Text type='tertiary' className='text-xs'>
            {'--'}
          </Text>
        )}
      </div>
      <div className='flex items-center justify-start gap-2 whitespace-nowrap text-[11px] leading-4 text-[var(--semi-color-text-2)]'>
        <span>{trendSummaryText}</span>
        <span>{lastActiveText}</span>
      </div>
    </div>
  );
};

const normalizeGroupName = (groupName) => {
  const value = String(groupName || '').trim();
  return value || 'default';
};

const buildGroupedChannels = (
  items = [],
  sortBy = 'request_count',
  order = 'desc',
) => {
  const groups = new Map();
  items.forEach((item) => {
    const groupName = normalizeGroupName(item.group_name);
    if (!groups.has(groupName)) {
      groups.set(groupName, []);
    }
    groups.get(groupName).push({
      ...item,
      __rowKey: `channel-${item.id}`,
    });
  });

  const groupedItems = Array.from(groups.entries()).map(
    ([groupName, groupItems]) => {
      const totalRequests = groupItems.reduce(
        (sum, item) => sum + Number(item.request_count || 0),
        0,
      );
      const totalFailures = groupItems.reduce(
        (sum, item) => sum + Number(item.failure_count || 0),
        0,
      );
      const weightedLatency = groupItems.reduce(
        (sum, item) =>
          sum + Number(item.avg_latency || 0) * Number(item.request_count || 0),
        0,
      );
      const latestActive = groupItems.reduce((latest, item) => {
        if (!item.last_active) {
          return latest;
        }
        if (!latest) {
          return item.last_active;
        }
        return new Date(item.last_active).getTime() > new Date(latest).getTime()
          ? item.last_active
          : latest;
      }, '');
      const summary = {
        id: `group-${groupName}`,
        __rowKey: `group-${groupName}`,
        __groupRow: true,
        name: groupName,
        group_name: groupName,
        request_count: totalRequests,
        failure_count: totalFailures,
        success_rate:
          totalRequests > 0
            ? (totalRequests - totalFailures) / totalRequests
            : 0,
        avg_latency: totalRequests > 0 ? weightedLatency / totalRequests : 0,
        p95_latency: Math.max(
          ...groupItems.map((item) => Number(item.p95_latency || 0)),
          0,
        ),
        last_active: latestActive,
        channel_count: groupItems.length,
      };
      return {
        summary,
        items: sortChannelMonitorItems(groupItems, sortBy, order).map(
          (item) => ({
            ...item,
            __rowKey: `channel-${item.id}`,
          }),
        ),
      };
    },
  );

  const sortedGroups = groupedItems.sort((left, right) => {
    const ascending = String(order || '').toLowerCase() === 'asc';
    if (sortBy === 'group_name') {
      return ascending
        ? String(left.summary.group_name || '').localeCompare(
            String(right.summary.group_name || ''),
          )
        : String(right.summary.group_name || '').localeCompare(
            String(left.summary.group_name || ''),
          );
    }
    if (sortBy === 'group_success_rate') {
      const leftRate = Number(left.summary.success_rate || 0);
      const rightRate = Number(right.summary.success_rate || 0);
      if (leftRate !== rightRate) {
        return ascending ? leftRate - rightRate : rightRate - leftRate;
      }
      return String(left.summary.group_name || '').localeCompare(
        String(right.summary.group_name || ''),
      );
    }
    const rankedSummaries = sortChannelMonitorItems(
      [left.summary, right.summary],
      sortBy,
      order,
    );
    if (
      rankedSummaries.length < 2 ||
      rankedSummaries[0].id === rankedSummaries[1].id
    ) {
      return 0;
    }
    return rankedSummaries[0].id === left.summary.id ? -1 : 1;
  });

  return sortedGroups.flatMap(({ summary, items: groupItems }) => [
    summary,
    ...groupItems,
  ]);
};

const isChannelMonitorItemLess = (left, right, sortBy) => {
  switch (sortBy) {
    case 'success_rate':
      if (left.success_rate !== right.success_rate) {
        return left.success_rate < right.success_rate;
      }
      if (left.failure_count !== right.failure_count) {
        return left.failure_count < right.failure_count;
      }
      return left.request_count < right.request_count;
    case 'avg_latency': {
      const leftHasLatency = Number(left.avg_latency || 0) > 0;
      const rightHasLatency = Number(right.avg_latency || 0) > 0;
      if (leftHasLatency !== rightHasLatency) {
        return leftHasLatency && !rightHasLatency;
      }
      if (left.avg_latency !== right.avg_latency) {
        return left.avg_latency < right.avg_latency;
      }
      if (left.p95_latency !== right.p95_latency) {
        return left.p95_latency < right.p95_latency;
      }
      return (
        String(left.name || '').toLowerCase() <
        String(right.name || '').toLowerCase()
      );
    }
    case 'p95_latency':
      if (left.p95_latency !== right.p95_latency) {
        return left.p95_latency < right.p95_latency;
      }
      return left.avg_latency < right.avg_latency;
    case 'failure_count':
      if (left.failure_count !== right.failure_count) {
        return left.failure_count < right.failure_count;
      }
      return left.request_count < right.request_count;
    case 'last_active': {
      const leftTime = left.last_active
        ? new Date(left.last_active).getTime()
        : 0;
      const rightTime = right.last_active
        ? new Date(right.last_active).getTime()
        : 0;
      if (leftTime !== rightTime) {
        return leftTime < rightTime;
      }
      return (
        String(left.name || '').toLowerCase() <
        String(right.name || '').toLowerCase()
      );
    }
    case 'group_name': {
      const leftGroup = String(left.group_name || '')
        .trim()
        .toLowerCase();
      const rightGroup = String(right.group_name || '')
        .trim()
        .toLowerCase();
      if (leftGroup !== rightGroup) {
        return leftGroup < rightGroup;
      }
      return (
        String(left.name || '').toLowerCase() <
        String(right.name || '').toLowerCase()
      );
    }
    case 'group_success_rate': {
      const leftGroup = String(left.group_name || '')
        .trim()
        .toLowerCase();
      const rightGroup = String(right.group_name || '')
        .trim()
        .toLowerCase();
      if (leftGroup !== rightGroup) {
        return leftGroup < rightGroup;
      }
      if (left.success_rate !== right.success_rate) {
        return left.success_rate > right.success_rate;
      }
      if (left.failure_count !== right.failure_count) {
        return left.failure_count < right.failure_count;
      }
      if (left.request_count !== right.request_count) {
        return left.request_count > right.request_count;
      }
      return (
        String(left.name || '').toLowerCase() <
        String(right.name || '').toLowerCase()
      );
    }
    case 'name':
      return (
        String(left.name || '').toLowerCase() <
        String(right.name || '').toLowerCase()
      );
    default:
      if (left.request_count !== right.request_count) {
        return left.request_count < right.request_count;
      }
      return (
        String(left.name || '').toLowerCase() <
        String(right.name || '').toLowerCase()
      );
  }
};

const sortChannelMonitorItems = (items, sortBy, order) => {
  const nextItems = [...items];
  const ascending = String(order || '').toLowerCase() === 'asc';
  const normalizedSortBy = String(sortBy || 'request_count').toLowerCase();

  if (
    normalizedSortBy === 'group_name' ||
    normalizedSortBy === 'group_success_rate'
  ) {
    nextItems.sort((left, right) => {
      const leftGroup = String(left.group_name || '')
        .trim()
        .toLowerCase();
      const rightGroup = String(right.group_name || '')
        .trim()
        .toLowerCase();
      const groupCompare = ascending
        ? leftGroup.localeCompare(rightGroup)
        : rightGroup.localeCompare(leftGroup);
      if (groupCompare !== 0) {
        return groupCompare;
      }
      if (
        normalizedSortBy === 'group_success_rate' &&
        left.success_rate !== right.success_rate
      ) {
        return ascending
          ? Number(left.success_rate || 0) - Number(right.success_rate || 0)
          : Number(right.success_rate || 0) - Number(left.success_rate || 0);
      }
      return String(left.name || '')
        .toLowerCase()
        .localeCompare(String(right.name || '').toLowerCase());
    });
    return nextItems;
  }

  nextItems.sort((left, right) => {
    if (ascending) {
      if (isChannelMonitorItemLess(left, right, normalizedSortBy)) {
        return -1;
      }
      if (isChannelMonitorItemLess(right, left, normalizedSortBy)) {
        return 1;
      }
      return 0;
    }
    if (isChannelMonitorItemLess(right, left, normalizedSortBy)) {
      return -1;
    }
    if (isChannelMonitorItemLess(left, right, normalizedSortBy)) {
      return 1;
    }
    return 0;
  });
  return nextItems;
};

const ChannelMonitorPage = () => {
  const [days, setDays] = useState(1);
  const [sortBy, setSortBy] = useState('success_rate');
  const [order, setOrder] = useState('desc');
  const [groupMode, setGroupMode] = useState('none');
  const [statusFilter, setStatusFilter] = useState('1');
  const [groupFilter, setGroupFilter] = useState(GROUP_FILTER_ALL);
  const [keyword, setKeyword] = useState('');
  const [groupOptions, setGroupOptions] = useState([]);
  const [loadingChannels, setLoadingChannels] = useState(true);
  const [loadingTimeline, setLoadingTimeline] = useState(false);
  const [loadingSummary, setLoadingSummary] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [channels, setChannels] = useState([]);
  const [timeline, setTimeline] = useState([]);
  const [summary, setSummary] = useState(null);
  const [health, setHealth] = useState([]);
  const [errorMessage, setErrorMessage] = useState('');
  const [overrideModal, setOverrideModal] = useState({
    visible: false,
    record: null,
    value: 0,
  });
  const [editModal, setEditModal] = useState({
    visible: false,
    channel: { id: undefined },
  });

  const updateChannelItem = useCallback((channelId, updateFn) => {
    setChannels((prevChannels) =>
      prevChannels.map((item) => {
        if (Number(item.id) !== Number(channelId)) {
          return item;
        }
        const nextItem = { ...item };
        updateFn(nextItem);
        return nextItem;
      }),
    );
  }, []);

  const handleScoreOverride = useCallback(async () => {
    const { record, value } = overrideModal;
    try {
      await setChannelScoreOverride(record.id, value);
      updateChannelItem(record.id, (item) => {
        item.current_weighted_score = value;
      });
      setOverrideModal((v) => ({ ...v, visible: false }));
    } catch (e) {
      showError(e.message);
    }
  }, [overrideModal, updateChannelItem]);

  const handleClearOverride = useCallback(async () => {
    const { record } = overrideModal;
    try {
      await setChannelScoreOverride(record.id, null);
      updateChannelItem(record.id, (item) => {
        item.current_weighted_score = null;
      });
      setOverrideModal((v) => ({ ...v, visible: false }));
    } catch (e) {
      showError(e.message);
    }
  }, [overrideModal, updateChannelItem]);

  const handlePriorityChange = useCallback(
    async (record, value) => {
      if (
        record.__groupRow ||
        value === '' ||
        value === undefined ||
        value === null
      ) {
        return;
      }
      const priority = parseInt(value, 10);
      if (Number.isNaN(priority)) {
        showError('优先级必须是整数！');
        return;
      }
      try {
        await updateChannelPriority(record.id, priority);
        updateChannelItem(record.id, (item) => {
          item.priority = priority;
        });
        showSuccess('操作成功完成！');
      } catch (e) {
        showError(e.message);
      }
    },
    [updateChannelItem],
  );

  const loadChannels = useCallback(async () => {
    setLoadingChannels(true);
    try {
      const channelData = await fetchChannelMonitorChannels({
        days,
        all: true,
      });
      setChannels(channelData.items);
      setGroupOptions(channelData.groups || []);
      setErrorMessage('');
    } catch (error) {
      setChannels([]);
      setErrorMessage(error.message || '加载渠道监控数据失败');
    } finally {
      setLoadingChannels(false);
    }
  }, [days]);

  const loadTimeline = useCallback(async () => {
    setLoadingTimeline(true);
    try {
      const timelineData = await fetchChannelMonitorTimeline({
        hours: TIMELINE_HOURS,
        bucketMinutes: TIMELINE_BUCKET_MINUTES,
        limit: TIMELINE_LIMIT,
      });
      setTimeline(timelineData);
    } catch (error) {
      setTimeline([]);
    } finally {
      setLoadingTimeline(false);
    }
  }, []);

  const loadSummary = useCallback(async () => {
    setLoadingSummary(true);
    try {
      const [summaryData, healthData] = await Promise.all([
        fetchChannelMonitorSummary(days),
        fetchChannelMonitorHealth(days),
      ]);
      setSummary(summaryData || null);
      setHealth(healthData || []);
    } catch (error) {
      setSummary(null);
      setHealth([]);
    } finally {
      setLoadingSummary(false);
    }
  }, [days]);

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    try {
      await Promise.all([loadChannels(), loadTimeline(), loadSummary()]);
    } finally {
      setRefreshing(false);
    }
  }, [loadChannels, loadTimeline, loadSummary]);

  const closeEditModal = useCallback(() => {
    setEditModal({ visible: false, channel: { id: undefined } });
  }, []);

  const refreshAfterEdit = useCallback(async () => {
    await loadChannels();
  }, [loadChannels]);

  useEffect(() => {
    loadChannels();
  }, [loadChannels]);

  useEffect(() => {
    loadSummary();
  }, [loadSummary]);

  useEffect(() => {
    loadTimeline();
  }, [loadTimeline]);

  useEffect(() => {
    if (groupFilter === GROUP_FILTER_ALL) {
      return;
    }
    if (!groupOptions.includes(groupFilter)) {
      setGroupFilter(GROUP_FILTER_ALL);
    }
  }, [groupFilter, groupOptions]);

  const groupedChannelCounts = useMemo(() => {
    return channels.reduce(
      (counts, item) => {
        const key = normalizeGroupName(item.group_name);
        counts.all += 1;
        counts[key] = (counts[key] || 0) + 1;
        return counts;
      },
      { all: 0 },
    );
  }, [channels]);

  const availableGroupTabs = useMemo(() => {
    return groupOptions.filter(
      (group) => groupedChannelCounts[normalizeGroupName(group)] > 0,
    );
  }, [groupOptions, groupedChannelCounts]);

  const timelineByChannelId = useMemo(() => {
    const map = new Map();
    timeline.forEach((item) => {
      map.set(Number(item.channel_id), item);
    });
    return map;
  }, [timeline]);

  const displayChannels = useMemo(() => {
    const normalizedKeyword = String(keyword || '')
      .trim()
      .toLowerCase();
    const filteredChannels = channels.filter((item) => {
      const matchesGroup =
        groupFilter === GROUP_FILTER_ALL ||
        normalizeGroupName(item.group_name) === groupFilter;
      const matchesKeyword =
        normalizedKeyword === '' ||
        String(item.name || '')
          .toLowerCase()
          .includes(normalizedKeyword);
      const matchesStatus =
        statusFilter === 'all' || String(item.status || '') === statusFilter;
      return matchesGroup && matchesKeyword && matchesStatus;
    });
    const sortedChannels = sortChannelMonitorItems(
      filteredChannels,
      sortBy,
      order,
    );
    if (groupMode === 'group') {
      return buildGroupedChannels(sortedChannels, sortBy, order);
    }
    return sortedChannels.map((item) => ({
      ...item,
      __rowKey: `channel-${item.id}`,
    }));
  }, [channels, groupFilter, groupMode, keyword, order, sortBy, statusFilter]);

  const latestHealth = useMemo(() => {
    if (!Array.isArray(health) || health.length === 0) {
      return null;
    }
    return health[health.length - 1];
  }, [health]);

  const availabilityMeta = useMemo(
    () => getAvailabilityMeta(summary?.success_rate, latestHealth?.error_channels),
    [latestHealth?.error_channels, summary?.success_rate],
  );

  const summaryDescription = useMemo(() => {
    if (!summary) {
      return loadingSummary ? '正在加载渠道概览...' : '暂无渠道概览数据。';
    }
    const timeRange = formatTimeRange(summary.time_range, days);
    const healthText = latestHealth
      ? `，当前健康 ${renderNumber(latestHealth.healthy_channels || 0)}、预警 ${renderNumber(
          latestHealth.warning_channels || 0,
        )}、异常 ${renderNumber(latestHealth.error_channels || 0)}`
      : '';
    return `近 ${timeRange} 内，系统可用性 ${formatPercentage(summary.success_rate)}，共处理 ${renderNumber(
      summary.total_requests || 0,
    )} 次请求，平均延迟 ${formatLatencyValue(summary.avg_latency)}，当前活跃渠道 ${renderNumber(
      summary.active_channels || 0,
    )}/${renderNumber(summary.total_channels || 0)}${healthText}。`;
  }, [days, latestHealth, loadingSummary, summary]);

  const summarySuggestion = useMemo(() => {
    if (!summary) {
      return '';
    }
    return buildActionSuggestion(summary, latestHealth);
  }, [latestHealth, summary]);

  const summaryMetrics = useMemo(
    () => [
      {
        label: '系统可用性',
        value: formatPercentage(summary?.success_rate),
        accent: true,
      },
      {
        label: '请求总量',
        value: renderNumber(summary?.total_requests || 0),
      },
      {
        label: '平均延迟',
        value: formatLatencyValue(summary?.avg_latency),
      },
      {
        label: '活跃渠道',
        value: `${renderNumber(summary?.active_channels || 0)}/${renderNumber(
          summary?.total_channels || 0,
        )}`,
      },
    ],
    [summary],
  );

  const columns = useMemo(() => {
    return [
      {
        title: '渠道',
        dataIndex: 'name',
        key: 'name',
        width: 260,
        render: (text, record) => {
          if (record.__groupRow) {
            return (
              <div className='flex flex-col gap-1 rounded-xl bg-[var(--semi-color-fill-0)] px-3 py-2'>
                <div className='flex items-center gap-2 flex-wrap'>
                  {renderGroup(record.group_name)}
                  <Text strong>{'分组汇总'}</Text>
                </div>
                <Text type='secondary'>
                  {`${renderNumber(record.channel_count || 0)} 个渠道 · ${renderNumber(record.request_count || 0)} 请求`}
                </Text>
              </div>
            );
          }
          return (
            <div className='flex flex-col gap-1'>
              <div className='flex items-center gap-2 flex-wrap'>
                {getChannelIcon(record.type)}
                <Text
                  link
                  className='!font-medium cursor-pointer'
                  onClick={() =>
                    setEditModal({ visible: true, channel: record })
                  }
                >
                  {text || '-'}
                </Text>
              </div>
              <div className='flex items-center gap-2 flex-wrap'>
                {groupFilter === GROUP_FILTER_ALL
                  ? renderGroup(record.group_name)
                  : null}
                {renderChannelStatus(record.status)}
                {record.temporary_circuit_open ? (
                  <Tag
                    color='orange'
                    shape='circle'
                    title={record.temporary_circuit_reason || '临时熔断中'}
                  >
                    {'临时熔断'}
                  </Tag>
                ) : null}
              </div>
            </div>
          );
        },
      },
      {
        title: '渠道优先级',
        dataIndex: 'priority',
        key: 'priority',
        width: 110,
        render: (_, record) => {
          if (record.__groupRow) {
            return <Text type='secondary'>-</Text>;
          }
          return (
            <InputNumber
              style={{ width: 70 }}
              name='priority'
              onBlur={(e) => handlePriorityChange(record, e.target.value)}
              keepFocus={true}
              innerButtons
              defaultValue={record.priority}
              min={-999}
              size='small'
            />
          );
        },
      },
      {
        title: (
          <Tooltip content='用于监控页排序和手动覆盖的路由选择评分，不等同于渠道配置中的权重。'>
            {'路由评分'}
          </Tooltip>
        ),
        dataIndex: 'current_weighted_score',
        key: 'weight_score',
        width: 120,
        render: (_, record) => {
          if (record.__groupRow) {
            return <Text type='secondary'>-</Text>;
          }
          const currentValue = record.current_weighted_score;
          const suggestion = getSuggestion(record);
          return (
            <div className='flex flex-col gap-1.5'>
              <div className='flex items-center gap-1.5 flex-wrap'>
                <Text type='secondary'>{'当前'}</Text>
                <div
                  className='cursor-pointer'
                  onClick={() =>
                    setOverrideModal({
                      visible: true,
                      record,
                      value: suggestion.score,
                    })
                  }
                  title='点击设置路由评分'
                >
                  {hasWeightScore(currentValue) ? (
                    <div title={record.temporary_circuit_reason || ''}>
                      {renderScoreTag(currentValue)}
                    </div>
                  ) : (
                    <Text type='secondary'>
                      {record.temporary_circuit_open ? '熔断中' : '-'}
                    </Text>
                  )}
                </div>
              </div>
              <div className='flex items-center gap-1.5 flex-wrap'>
                <Text type='secondary'>{'建议'}</Text>
                {renderScoreTag(suggestion.score)}
              </div>
            </div>
          );
        },
      },
      {
        title: '可用性趋势',
        key: 'availability_trend',
        render: (_, record) => {
          if (record.__groupRow) {
            return <Text type='secondary'>-</Text>;
          }
          return (
            <AvailabilityTrend
              item={timelineByChannelId.get(Number(record.id))}
              loading={loadingTimeline}
              lastActive={record.last_active}
            />
          );
        },
      },
    ];
  }, [
    handlePriorityChange,
    loadingTimeline,
    timelineByChannelId,
    setEditModal,
  ]);

  return (
    <>
      <EditChannelModal
        refresh={refreshAfterEdit}
        visible={editModal.visible}
        handleClose={closeEditModal}
        editingChannel={editModal.channel}
      />
      <div className='mt-[60px] px-2 pb-6'>
        <div className='space-y-4'>
          <Card
            bordered
            className='!rounded-2xl overflow-hidden'
            bodyStyle={{ padding: 20 }}
            style={{
              background:
                'linear-gradient(135deg, rgba(15,23,42,0.04) 0%, rgba(15,118,110,0.05) 35%, rgba(37,99,235,0.04) 100%)',
            }}
          >
            <div className='flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between'>
              <div>
                <div className='text-2xl font-semibold'>{'渠道监控'}</div>
              </div>
              <Space wrap>
                <Space spacing={8} wrap>
                  {DAY_OPTIONS.map((item) => {
                    const active = days === item.value;
                    return (
                      <Button
                        key={item.value}
                        theme={active ? 'solid' : 'light'}
                        type={active ? 'primary' : 'tertiary'}
                        onClick={() => {
                          setDays(item.value);
                        }}
                      >
                        {item.label}
                      </Button>
                    );
                  })}
                </Space>
                <div className='flex items-center gap-2'>
                  <Text type='secondary'>
                    {groupMode === 'group' ? '按渠道分组' : '不分组'}
                  </Text>
                  <Switch
                    checked={groupMode === 'group'}
                    onChange={(checked) => {
                      const value = checked ? 'group' : 'none';
                      setGroupMode(value);
                      if (
                        !checked &&
                        (sortBy === 'group_success_rate' ||
                          sortBy === 'group_name')
                      ) {
                        setSortBy('success_rate');
                      }
                    }}
                  />
                </div>
                <Input
                  value={keyword}
                  placeholder='搜索渠道名称'
                  onChange={(value) => {
                    setKeyword(value);
                  }}
                  showClear
                  style={{ width: 200 }}
                />
                <Select
                  value={statusFilter}
                  optionList={STATUS_FILTER_OPTIONS}
                  onChange={(value) => {
                    setStatusFilter(value || '1');
                  }}
                  style={{ width: 140 }}
                />
                <Select
                  value={sortBy}
                  optionList={SORT_OPTIONS.map((item) => ({
                    label: item.label,
                    value: item.value,
                  }))}
                  onChange={(value) => {
                    setSortBy(value);
                    if (
                      value === 'group_success_rate' ||
                      value === 'group_name'
                    ) {
                      setGroupMode('group');
                    }
                  }}
                  style={{ width: 140 }}
                />
                <div className='flex items-center gap-2'>
                  <Text type='secondary'>
                    {order === 'asc' ? '升序' : '降序'}
                  </Text>
                  <Switch
                    checked={order === 'asc'}
                    onChange={(checked) => {
                      setOrder(checked ? 'asc' : 'desc');
                    }}
                  />
                </div>
                <Button
                  theme='light'
                  type='tertiary'
                  icon={
                    <RefreshCw
                      size={14}
                      className={refreshing ? 'animate-spin' : ''}
                    />
                  }
                  onClick={handleRefresh}
                  loading={refreshing}
                />
              </Space>
            </div>
          </Card>

          {errorMessage ? (
            <Banner
              type='danger'
              fullMode={false}
              bordered
              title={'渠道监控加载失败'}
              description={errorMessage}
              closeIcon={null}
            />
          ) : null}

          <Card
            bordered
            loading={loadingSummary}
            className='!rounded-2xl overflow-hidden'
            bodyStyle={{ padding: 20 }}
          >
            <div className='flex flex-col gap-4'>
              <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4'>
                {summaryMetrics.map((metric) => (
                  <div
                    key={metric.label}
                    className={`rounded-2xl border border-[var(--semi-color-border)] bg-[var(--semi-color-bg-1)] p-4 ${
                      metric.accent
                        ? 'shadow-sm ring-1 ring-[rgba(var(--semi-green-5),0.16)]'
                        : ''
                    }`}
                  >
                    <Text type='secondary' size='small'>
                      {metric.label}
                    </Text>
                    <div
                      className={`mt-2 font-semibold ${
                        metric.accent
                          ? 'text-3xl text-[var(--semi-color-success)]'
                          : 'text-2xl'
                      }`}
                    >
                      {renderMetricValue(loadingSummary, metric.value)}
                    </div>
                  </div>
                ))}
              </div>
              <div className='flex flex-col gap-3 rounded-2xl bg-[var(--semi-color-fill-0)] p-4 lg:flex-row lg:items-start lg:justify-between'>
                <div className='flex-1 leading-6'>
                  <Text>{summaryDescription}</Text>
                  {summarySuggestion ? (
                    <div className='mt-2'>
                      <Text>{`建议：${summarySuggestion}`}</Text>
                    </div>
                  ) : null}
                </div>
                <Space wrap>
                  <Tag color={availabilityMeta.color} shape='circle'>
                    {availabilityMeta.label}
                  </Tag>
                  {latestHealth ? (
                    <>
                      <Tag color='green' shape='circle'>{`健康 ${renderNumber(
                        latestHealth.healthy_channels || 0,
                      )}`}</Tag>
                      <Tag color='orange' shape='circle'>{`预警 ${renderNumber(
                        latestHealth.warning_channels || 0,
                      )}`}</Tag>
                      <Tag color='red' shape='circle'>{`异常 ${renderNumber(
                        latestHealth.error_channels || 0,
                      )}`}</Tag>
                    </>
                  ) : null}
                </Space>
              </div>
            </div>
          </Card>

          <ChannelTable
            tabs={
              <Tabs
                activeKey={groupFilter}
                type='card'
                collapsible
                onChange={(key) => setGroupFilter(key || GROUP_FILTER_ALL)}
              >
                <TabPane
                  itemKey={GROUP_FILTER_ALL}
                  tab={
                    <span className='flex items-center gap-2'>
                      {'全部'}
                      <Tag
                        color={
                          groupFilter === GROUP_FILTER_ALL ? 'red' : 'grey'
                        }
                        shape='circle'
                      >
                        {groupedChannelCounts.all || 0}
                      </Tag>
                    </span>
                  }
                />
                {availableGroupTabs.map((group) => {
                  const key = normalizeGroupName(group);
                  return (
                    <TabPane
                      key={key}
                      itemKey={key}
                      tab={
                        <span className='flex items-center gap-2'>
                          <Tag color='grey' shape='circle'>
                            {group || 'default'}
                          </Tag>
                          <Tag
                            color={groupFilter === key ? 'red' : 'grey'}
                            shape='circle'
                          >
                            {groupedChannelCounts[key] || 0}
                          </Tag>
                        </span>
                      }
                    />
                  );
                })}
              </Tabs>
            }
            emptyDescription={'暂无渠道监控数据'}
            loading={loadingChannels}
            channels={displayChannels}
            columns={columns}
          />
        </div>
      </div>
      <Modal
        title='手动设置路由评分'
        visible={overrideModal.visible}
        onOk={handleScoreOverride}
        onCancel={() => setOverrideModal((v) => ({ ...v, visible: false }))}
      >
        <div className='flex flex-col gap-3'>
          <InputNumber
            min={0}
            max={1}
            step={0.01}
            precision={2}
            value={overrideModal.value}
            onChange={(v) =>
              setOverrideModal((prev) => ({ ...prev, value: v }))
            }
            style={{ width: '100%' }}
          />
          <Text type='tertiary' size='small'>
            {'注意：此设置仅在内存中生效，服务重启后将恢复自动计算。'}
          </Text>
          <Button
            type='danger'
            theme='borderless'
            size='small'
            onClick={handleClearOverride}
          >
            {'清除覆盖，恢复自动计算'}
          </Button>
        </div>
      </Modal>
    </>
  );
};

export default ChannelMonitorPage;
