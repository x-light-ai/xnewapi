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

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Banner, Button, Card, Select, Space, Tag, Typography } from '@douyinfe/semi-ui';
import { RefreshCw } from 'lucide-react';
import { CHANNEL_OPTIONS } from '../../constants';
import { getChannelIcon, renderNumber, timestamp2string } from '../../helpers';
import {
  fetchChannelMonitorChannels,
  fetchChannelMonitorTimeline,
} from '../../helpers/api/channel-monitor';
import ChannelTable from './ChannelTable';
import AvailabilityTimeline from './AvailabilityTimeline';

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
  { label: '名称', value: 'name' },
  { label: '请求数', value: 'request_count' },
];

const ORDER_OPTIONS = [
  { label: '降序', value: 'desc' },
  { label: '升序', value: 'asc' },
];

const TIMELINE_HOURS = 24;
const TIMELINE_BUCKET_MINUTES = 10;
const TIMELINE_LIMIT = 20;

const getChannelTypeMeta = (type) => {
  const option = CHANNEL_OPTIONS.find((item) => item.value === type);
  if (option) {
    return option;
  }
  return {
    label: '未知类型',
    color: 'grey',
  };
};

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

const renderRateTag = (rate) => {
  const percent = Number(rate || 0) * 100;
  let color = 'red';
  if (percent >= 90) {
    color = 'green';
  } else if (percent >= 70) {
    color = 'lime';
  } else if (percent >= 50) {
    color = 'yellow';
  }
  return (
    <Tag color={color} shape='circle'>
      {percent.toFixed(2)}%
    </Tag>
  );
};

const renderLatencyTag = (latency) => {
  const value = Number(latency || 0);
  let color = 'grey';
  if (value > 0 && value <= 1000) {
    color = 'green';
  } else if (value <= 3000) {
    color = 'lime';
  } else if (value <= 5000) {
    color = 'yellow';
  } else if (value > 5000) {
    color = 'red';
  }
  return (
    <Tag color={color} shape='circle'>
      {value.toFixed(0)} ms
    </Tag>
  );
};

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
      {value.toFixed(4)}
    </Tag>
  );
};

const getSuggestedWeightScore = (record) => {
  if (record.temporary_circuit_open) {
    return 0.05;
  }
  const successRate = Math.max(0, Math.min(1, Number(record.success_rate || 0)));
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
      detail: record.temporary_circuit_reason || '渠道处于临时熔断中，建议停用或大幅降权。',
    };
  }
  if (Number(record.success_rate || 0) < 0.7 || Number(record.p95_latency || 0) > 5000) {
    return {
      text: '建议降权',
      color: 'orange',
      score: suggestedWeightScore,
      detail: '成功率偏低或 P95 延迟偏高，建议下调权重。',
    };
  }
  if (Number(record.success_rate || 0) >= 0.9 && Number(record.avg_latency || 0) > 0 && Number(record.avg_latency || 0) <= 1000) {
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

const ChannelMonitorPage = () => {
  const [days, setDays] = useState(1);
  const [sortBy, setSortBy] = useState('success_rate');
  const [order, setOrder] = useState('desc');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [loadingChannels, setLoadingChannels] = useState(true);
  const [loadingTimeline, setLoadingTimeline] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [channels, setChannels] = useState([]);
  const [timeline, setTimeline] = useState([]);
  const [total, setTotal] = useState(0);
  const [errorMessage, setErrorMessage] = useState('');
  const [shouldLoadTimeline, setShouldLoadTimeline] = useState(false);
  const timelineSectionRef = useRef(null);

  const loadChannels = useCallback(async () => {
    setLoadingChannels(true);
    try {
      const channelData = await fetchChannelMonitorChannels({
        days,
        page,
        pageSize,
        sort: sortBy,
        order,
      });
      setChannels(channelData.items);
      setTotal(channelData.total);
      setErrorMessage('');
    } catch (error) {
      setChannels([]);
      setTotal(0);
      setErrorMessage(error.message || '加载渠道监控数据失败');
    } finally {
      setLoadingChannels(false);
    }
  }, [days, order, page, pageSize, sortBy]);

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

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    try {
      if (!shouldLoadTimeline) {
        setShouldLoadTimeline(true);
        await loadChannels();
        return;
      }
      await Promise.all([loadChannels(), loadTimeline()]);
    } finally {
      setRefreshing(false);
    }
  }, [loadChannels, loadTimeline, shouldLoadTimeline]);

  useEffect(() => {
    loadChannels();
  }, [loadChannels]);

  useEffect(() => {
    if (shouldLoadTimeline) {
      return undefined;
    }
    const node = timelineSectionRef.current;
    if (!node || typeof IntersectionObserver === 'undefined') {
      setShouldLoadTimeline(true);
      return undefined;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setShouldLoadTimeline(true);
          observer.disconnect();
        }
      },
      { rootMargin: '200px 0px' },
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [shouldLoadTimeline]);

  useEffect(() => {
    if (!shouldLoadTimeline) {
      return undefined;
    }
    let cancelled = false;
    const load = async () => {
      setLoadingTimeline(true);
      try {
        const timelineData = await fetchChannelMonitorTimeline({
          hours: TIMELINE_HOURS,
          bucketMinutes: TIMELINE_BUCKET_MINUTES,
          limit: TIMELINE_LIMIT,
        });
        if (!cancelled) {
          setTimeline(timelineData);
        }
      } catch (error) {
        if (!cancelled) {
          setTimeline([]);
        }
      } finally {
        if (!cancelled) {
          setLoadingTimeline(false);
        }
      }
    };
    load();
    return () => {
      cancelled = true;
    };
  }, [shouldLoadTimeline]);

  const columns = useMemo(() => {
    return [
      {
        title: '渠道',
        dataIndex: 'name',
        key: 'name',
        render: (text, record) => {
          const meta = getChannelTypeMeta(record.type);
          const suggestion = getSuggestion(record);
          const showSuggestionTag = suggestion.text !== '保持当前';
          return (
            <div className='flex flex-col gap-1'>
              <div className='flex items-center gap-2 flex-wrap'>
                {getChannelIcon(record.type)}
                <span className='font-medium'>{text || '-'}</span>
                {showSuggestionTag ? (
                  <Tag color={suggestion.color} shape='circle' title={suggestion.detail}>
                    {suggestion.text}
                  </Tag>
                ) : null}
              </div>
              <div className='flex items-center gap-2 flex-wrap'>
                <Tag color={meta.color} shape='circle'>
                  {meta.label}
                </Tag>
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
        title: '成功率',
        dataIndex: 'success_rate',
        key: 'success_rate',
        render: (value) => renderRateTag(value),
      },
      {
        title: '平均延迟',
        dataIndex: 'avg_latency',
        key: 'avg_latency',
        render: (value) => renderLatencyTag(value),
      },
      {
        title: 'P95 延迟',
        dataIndex: 'p95_latency',
        key: 'p95_latency',
        render: (value) => renderLatencyTag(value),
      },
      {
        title: '请求 / 失败',
        dataIndex: 'request_count',
        key: 'request_count',
        render: (_, record) => (
          <div className='flex flex-col items-end gap-1'>
            <Text>{renderNumber(record.request_count || 0)}</Text>
            <Text type='secondary'>
              {'失败'} {renderNumber(record.failure_count || 0)}
            </Text>
          </div>
        ),
      },
      {
        title: '权重分数',
        dataIndex: 'current_weighted_score',
        key: 'weight_score',
        render: (_, record) => {
          const currentValue = record.current_weighted_score;
          const suggestion = getSuggestion(record);
          return (
            <div className='flex flex-col items-end gap-1'>
              {currentValue ? (
                <div title={record.temporary_circuit_reason || ''}>{renderScoreTag(currentValue)}</div>
              ) : (
                <Text type='secondary'>{record.temporary_circuit_open ? '当前 熔断中' : '当前 -'}</Text>
              )}
              <Text type='secondary'>
                {'建议 '}
                {suggestion.score.toFixed(4)}
              </Text>
            </div>
          );
        },
      },
      {
        title: '最近活跃',
        dataIndex: 'last_active',
        key: 'last_active',
        render: (value) => {
          if (!value) {
            return <Text type='secondary'>-</Text>;
          }
          const ts = Math.floor(new Date(value).getTime() / 1000);
          if (!ts || Number.isNaN(ts)) {
            return <Text type='secondary'>-</Text>;
          }
          return <Text>{timestamp2string(ts)}</Text>;
        },
      },
    ];
  }, []);

  return (
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
                        setPage(1);
                        setDays(item.value);
                      }}
                    >
                      {item.label}
                    </Button>
                  );
                })}
              </Space>
              <Select
                value={sortBy}
                optionList={SORT_OPTIONS.map((item) => ({
                  label: item.label,
                  value: item.value,
                }))}
                onChange={(value) => {
                  setPage(1);
                  setSortBy(value);
                }}
                style={{ width: 140 }}
              />
              <Select
                value={order}
                optionList={ORDER_OPTIONS.map((item) => ({
                  label: item.label,
                  value: item.value,
                }))}
                onChange={(value) => {
                  setPage(1);
                  setOrder(value);
                }}
                style={{ width: 120 }}
              />
              <Button
                theme='light'
                type='tertiary'
                icon={<RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} />}
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

        <div ref={timelineSectionRef}>
          <AvailabilityTimeline timeline={timeline} loading={loadingTimeline} />
        </div>

        <ChannelTable
          title={'渠道明细列表'}
          emptyDescription={'暂无渠道监控数据'}
          loading={loadingChannels}
          channels={channels}
          columns={columns}
          page={page}
          pageSize={pageSize}
          total={total}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size);
            setPage(1);
          }}
        />
      </div>
    </div>
  );
};

export default ChannelMonitorPage;
