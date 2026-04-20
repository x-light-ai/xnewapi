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

import React, { useMemo } from 'react';
import { Card, Empty, Tag, Typography } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { CHANNEL_OPTIONS } from '../../constants';
import { getChannelIcon, renderNumber } from '../../helpers';

const { Text } = Typography;

const HOURS = 24;
const TICK_COUNT = 7;

const getChannelTypeMeta = (type) => {
  const option = CHANNEL_OPTIONS.find((item) => item.value === type);
  return option || { label: '未知类型', color: 'grey' };
};

const formatTickLabel = (date) => {
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  return `${hours}:${minutes}`;
};

const getTimelineColor = (point) => {
  const requestCount = Number(point?.request_count || 0);
  const successCount = Number(point?.success_count || 0);
  const failureCount = Number(point?.failure_count || 0);
  if (requestCount <= 0) {
    return 'transparent';
  }
  if (failureCount <= 0) {
    return '#10b981';
  }
  if (successCount <= 0) {
    return '#f43f5e';
  }
  return '#f59e0b';
};

const getTimelineSize = (requestCount, maxRequestCount) => {
  if (requestCount <= 0 || maxRequestCount <= 0) {
    return 0;
  }
  const ratio = requestCount / maxRequestCount;
  return Math.max(8, Math.min(22, 8 + ratio * 14));
};

const AvailabilityTimeline = ({ timeline = [], loading = false }) => {
  const maxRequestCount = timeline.reduce(
    (max, item) => Math.max(max, ...((item.points || []).map((point) => Number(point.request_count || 0)))),
    0,
  );

  const timelineMeta = useMemo(() => {
    const end = new Date();
    const start = new Date(end.getTime() - HOURS * 60 * 60 * 1000);
    const span = end.getTime() - start.getTime();
    const ticks = Array.from({ length: TICK_COUNT }, (_, index) => {
      const time = new Date(start.getTime() + (span * index) / (TICK_COUNT - 1));
      return {
        key: time.toISOString(),
        label: formatTickLabel(time),
      };
    });
    return { start, end, span, ticks };
  }, []);

  const getPointLeft = (timeBucket) => {
    const current = new Date(timeBucket).getTime();
    if (Number.isNaN(current)) {
      return 0;
    }
    const raw = ((current - timelineMeta.start.getTime()) / timelineMeta.span) * 100;
    return Math.max(0, Math.min(100, raw));
  };

  return (
    <Card className='!rounded-2xl' title={'渠道可用性时间线'} bodyStyle={{ padding: 20 }}>
      {timeline.length === 0 && !loading ? (
        <Empty
          image={<IllustrationNoResult style={{ width: 120, height: 120 }} />}
          darkModeImage={<IllustrationNoResultDark style={{ width: 120, height: 120 }} />}
          description={'暂无渠道时间线数据'}
          style={{ padding: 30 }}
        />
      ) : (
        <div className='space-y-5'>
          <div className='flex items-center justify-between gap-4 text-[13px] text-[var(--semi-color-text-2)]'>
            <div className='grid flex-1 grid-cols-7 pl-[320px]'>
              {timelineMeta.ticks.map((tick) => (
                <div key={tick.key}>{tick.label}</div>
              ))}
            </div>
            <div>{'时间分段: 24 小时'}</div>
          </div>
          {timeline.map((item) => {
            const meta = getChannelTypeMeta(item.channel_type);
            return (
              <div key={item.channel_id} className='flex items-center gap-4'>
                <div className='flex w-[300px] shrink-0 items-center gap-3'>
                  <div className='h-3 w-3 rounded-full bg-emerald-500' />
                  <div className='min-w-0 flex-1'>
                    <div className='flex items-center gap-2'>
                      {getChannelIcon(item.channel_type)}
                      <Text strong ellipsis={{ showTooltip: true }} style={{ maxWidth: 190 }}>
                        {item.channel_name || '-'}
                      </Text>
                    </div>
                    <div className='mt-1 flex items-center gap-2'>
                      <Tag color={meta.color} shape='circle'>
                        {meta.label}
                      </Tag>
                    </div>
                  </div>
                </div>
                <div className='relative flex min-h-[72px] flex-1 items-center rounded-2xl border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] px-6'>
                  <div className='absolute inset-x-6 top-1/2 border-t border-[var(--semi-color-border)]' />
                  {(item.points || []).map((point) => {
                    const size = getTimelineSize(Number(point.request_count || 0), maxRequestCount);
                    if (size <= 0) {
                      return null;
                    }
                    const color = getTimelineColor(point);
                    return (
                      <div
                        key={`${item.channel_id}-${point.time_bucket}`}
                        className='absolute top-1/2 -translate-x-1/2 -translate-y-1/2 rounded-full'
                        style={{
                          left: `calc(${getPointLeft(point.time_bucket)}% + 24px)`,
                          width: size,
                          height: size,
                          background: color,
                        }}
                        title={`${point.time_bucket} | 请求 ${point.request_count} | 成功 ${point.success_count} | 失败 ${point.failure_count}`}
                      />
                    );
                  })}
                </div>
                <div className='w-[120px] shrink-0 text-right'>
                  <div className={`text-[18px] font-semibold ${Number(item.success_rate || 0) >= 0.9 ? 'text-emerald-500' : Number(item.success_rate || 0) >= 0.5 ? 'text-amber-500' : 'text-rose-500'}`}>
                    {item.request_count > 0 ? `${(Number(item.success_rate || 0) * 100).toFixed(1)}%` : '暂无数据'}
                  </div>
                  <Text type='secondary'>
                    {item.request_count > 0 ? `${renderNumber(item.request_count || 0)} 个请求` : '无请求'}
                  </Text>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </Card>
  );
};

export default AvailabilityTimeline;
