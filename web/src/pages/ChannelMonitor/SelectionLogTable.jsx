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

import React from 'react';
import { Button, Card, Descriptions, Empty, Input, Select, Switch, Tag, Typography } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import CardTable from '../../components/common/ui/CardTable';
import { renderNumber, timestamp2string } from '../../helpers';

const { Text } = Typography;

const OUTCOME_OPTIONS = [
  { label: '全部结果', value: 'all' },
  { label: '已选中', value: 'selected' },
  { label: '无可用渠道', value: 'no_available' },
  { label: '临时熔断', value: 'temporary_circuit' },
  { label: '半开阻断', value: 'half_open_blocked' },
  { label: '请求失败', value: 'request_failed' },
];

const EVENT_META = {
  selection: { label: '选择', color: 'blue' },
  observation: { label: '结果', color: 'violet' },
};

const OUTCOME_META = {
  selected: { label: '已选中', color: 'green' },
  no_available: { label: '无可用', color: 'red' },
  temporary_circuit: { label: '临时熔断', color: 'orange' },
  half_open_blocked: { label: '半开阻断', color: 'yellow' },
  request_success: { label: '请求成功', color: 'green' },
  request_failed: { label: '请求失败', color: 'red' },
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
  return <Tag color={color} shape='circle'>{value.toFixed(2)}</Tag>;
};

const renderTimestamp = (value) => {
  const ts = Math.floor(new Date(value).getTime() / 1000);
  if (!ts || Number.isNaN(ts)) {
    return <Text type='secondary'>-</Text>;
  }
  return <Text className='whitespace-nowrap'>{timestamp2string(ts)}</Text>;
};

const renderOutcomeTags = (record) => {
  const eventMeta = EVENT_META[record.event_type] || EVENT_META.selection;
  const outcomeMeta = OUTCOME_META[record.outcome] || { label: record.outcome || '-', color: 'grey' };
  return (
    <div className='flex items-center gap-1.5 flex-wrap'>
      <Tag color={eventMeta.color} shape='circle'>{eventMeta.label}</Tag>
      <Tag color={outcomeMeta.color} shape='circle'>{outcomeMeta.label}</Tag>
      {record.has_circuit ? (
        <Tag color='orange' shape='circle'>{record.circuit_count > 1 ? `熔断×${record.circuit_count}` : '含熔断'}</Tag>
      ) : null}
    </div>
  );
};

const renderChannel = (record) => {
  if (!record.selected_channel_id) {
    return <Text type='danger'>无可用渠道</Text>;
  }
  return <Text>{`#${record.selected_channel_id} ${record.selected_channel_name || '-'}`}</Text>;
};

const SelectionLogTable = ({
  logs = [],
  loading = false,
  selectedChannel,
  filters,
  groupOptions = [],
  groupFilterAll = '__all__',
  onFiltersChange,
  onClearChannelFilter,
}) => {
  const updateFilter = (patch) => {
    onFiltersChange?.({
      model: filters?.model || '',
      group: filters?.group || groupFilterAll,
      outcome: filters?.outcome || 'all',
      abnormalOnly: Boolean(filters?.abnormalOnly),
      ...patch,
    });
  };

  const groupOptionList = [
    { label: '全部分组', value: groupFilterAll },
    ...groupOptions.map((group) => ({ label: group, value: group })),
  ];

  const columns = [
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 168,
      render: renderTimestamp,
    },
    {
      title: '模型 / 分组',
      key: 'model_group',
      width: 220,
      render: (_, record) => (
        <div className='flex flex-col gap-1'>
          <Text>{record.model_name || '-'}</Text>
          <Text type='secondary' size='small'>{record.group || '-'}</Text>
        </div>
      ),
    },
    {
      title: '渠道',
      key: 'selected_channel',
      width: 220,
      render: (_, record) => renderChannel(record),
    },
    {
      title: '结果',
      key: 'outcome',
      width: 220,
      render: (_, record) => renderOutcomeTags(record),
    },
    {
      title: '路由评分',
      dataIndex: 'selected_score',
      key: 'selected_score',
      width: 120,
      render: (value, record) => (record.selected_channel_id ? renderScoreTag(value) : <Text type='secondary'>-</Text>),
    },
    {
      title: '候选数',
      dataIndex: 'candidate_count',
      key: 'candidate_count',
      width: 100,
      render: (value) => renderNumber(value || 0),
    },
    {
      title: '说明',
      dataIndex: 'outcome_detail',
      key: 'outcome_detail',
      render: (value, record) => (
        <Text ellipsis={{ showTooltip: true }} style={{ maxWidth: 520 }}>
          {value || record.summary || '-'}
        </Text>
      ),
    },
  ];

  const expandRowRender = (record) => {
    const data = [
      {
        key: '事件类型',
        value: EVENT_META[record.event_type]?.label || record.event_type || '-',
      },
      {
        key: '处理结果',
        value: OUTCOME_META[record.outcome]?.label || record.outcome || '-',
      },
      {
        key: '说明',
        value: record.outcome_detail || record.summary || '-',
      },
      {
        key: '选中渠道',
        value: record.selected_channel_id ? `#${record.selected_channel_id} ${record.selected_channel_name || '-'}` : '无可用渠道',
      },
      {
        key: '路由评分',
        value: record.selected_channel_id && record.selected_score !== undefined ? Number(record.selected_score || 0).toFixed(4) : '-',
      },
      {
        key: '候选数',
        value: renderNumber(record.candidate_count || 0),
      },
    ];
    (record.candidates || []).forEach((candidate, index) => {
      const detail = candidate.reason
        ? candidate.reason
        : `priority=${candidate.priority} · score=${Number(candidate.score || 0).toFixed(4)}`;
      data.push({
        key: `候选 ${index + 1}`,
        value: `#${candidate.channel_id} ${candidate.name || '-'} · ${detail}`,
      });
    });
    return <Descriptions data={data} />;
  };

  return (
    <Card
      loading={loading}
      className='!rounded-2xl'
      title='最近路由日志'
      bodyStyle={{ padding: 12 }}
      headerExtraContent={
        <div className='flex items-center gap-2 flex-wrap justify-end'>
          <Text type='secondary' size='small'>
            {selectedChannel
              ? `渠道：#${selectedChannel.id} ${selectedChannel.name || '-'}`
              : '渠道：全部'}
          </Text>
          {selectedChannel ? (
            <Button theme='borderless' size='small' onClick={onClearChannelFilter}>
              {'清除渠道'}
            </Button>
          ) : null}
        </div>
      }
    >
      <div className='mb-3 flex items-center gap-2 flex-wrap'>
        <Input
          value={filters?.model || ''}
          placeholder='筛选模型'
          showClear
          onChange={(value) => updateFilter({ model: value })}
          style={{ width: 180 }}
        />
        <Select
          value={filters?.group || groupFilterAll}
          optionList={groupOptionList}
          onChange={(value) => updateFilter({ group: value || groupFilterAll })}
          style={{ width: 150 }}
        />
        <Select
          value={filters?.outcome || 'all'}
          optionList={OUTCOME_OPTIONS}
          onChange={(value) => updateFilter({ outcome: value || 'all' })}
          style={{ width: 150 }}
        />
        <div className='flex items-center gap-2'>
          <Switch
            checked={Boolean(filters?.abnormalOnly)}
            onChange={(checked) => updateFilter({ abnormalOnly: checked })}
          />
          <Text type='secondary'>{'只看异常'}</Text>
        </div>
      </div>
      {logs.length === 0 && !loading ? (
        <Empty
          image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
          darkModeImage={
            <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
          }
          description='暂无匹配的路由日志'
          style={{ padding: 30 }}
        />
      ) : (
        <CardTable
          columns={columns}
          dataSource={logs}
          rowKey={(record, index) => `${record.timestamp || 'ts'}-${record.event_type || 'event'}-${record.selected_channel_id || 0}-${index}`}
          loading={loading}
          className='rounded-xl overflow-hidden'
          size='middle'
          pagination={false}
          scroll={{ x: 'max-content' }}
          expandedRowRender={expandRowRender}
          rowExpandable={(record) => Array.isArray(record.candidates) && record.candidates.length > 0}
          expandRowByClick
        />
      )}
    </Card>
  );
};

export default SelectionLogTable;
