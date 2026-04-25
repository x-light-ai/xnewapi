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
import { Card, Empty, Tag, Typography } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import CardTable from '../../components/common/ui/CardTable';
import { CHANNEL_OPTIONS } from '../../constants';
import { getChannelIcon, renderNumber } from '../../helpers';

const { Text } = Typography;

const getChannelTypeMeta = (type) => {
  const option = CHANNEL_OPTIONS.find((item) => item.value === type);
  return option || { label: '未知类型', color: 'grey' };
};

const renderChannelStatus = (status) => {
  switch (status) {
    case 1:
      return <Tag color='green' shape='circle'>{'已启用'}</Tag>;
    case 2:
      return <Tag color='red' shape='circle'>{'已禁用'}</Tag>;
    case 3:
      return <Tag color='yellow' shape='circle'>{'自动禁用'}</Tag>;
    default:
      return <Tag color='grey' shape='circle'>{'未知状态'}</Tag>;
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
  return <Tag color={color} shape='circle'>{`${percent.toFixed(2)}%`}</Tag>;
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

const StabilityRankTable = ({ channels = [], loading = false }) => {
  const dataSource = channels.slice(0, 10).map((item, index) => ({
    ...item,
    rank: index + 1,
  }));

  const columns = [
    {
      title: '排名',
      dataIndex: 'rank',
      key: 'rank',
      width: 72,
      render: (value) => <Text strong>{`#${value}`}</Text>,
    },
    {
      title: '渠道',
      dataIndex: 'name',
      key: 'name',
      render: (text, record) => {
        const meta = getChannelTypeMeta(record.type);
        return (
          <div className='flex flex-col gap-1'>
            <div className='flex items-center gap-2'>
              {getChannelIcon(record.type)}
              <span className='font-medium'>{text || '-'}</span>
            </div>
            <div className='flex items-center gap-2 flex-wrap'>
              <Tag color={meta.color} shape='circle'>
                {meta.label}
              </Tag>
              {renderChannelStatus(record.status)}
              {record.temporary_circuit_open ? (
                <Tag color='orange' shape='circle'>
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
      width: 120,
      render: (value) => renderRateTag(value),
    },
    {
      title: '失败数',
      dataIndex: 'failure_count',
      key: 'failure_count',
      width: 100,
      render: (value) => renderNumber(value || 0),
    },
    {
      title: '建议权重',
      dataIndex: 'suggested_weight_score',
      key: 'suggested_weight_score',
      width: 120,
      render: (value) => renderScoreTag(value),
    },
    {
      title: '当前权重分数',
      dataIndex: 'current_weighted_score',
      key: 'current_weighted_score',
      width: 140,
      render: (value, record) => {
        if (!value) {
          return <Text type='secondary'>{record.temporary_circuit_open ? '熔断中' : '-'}</Text>;
        }
        return renderScoreTag(value);
      },
    },
  ];

  return (
    <Card className='!rounded-2xl' title={'稳定性排名'} bodyStyle={{ padding: 12 }}>
      {dataSource.length === 0 && !loading ? (
        <Empty
          image={<IllustrationNoResult style={{ width: 120, height: 120 }} />}
          darkModeImage={<IllustrationNoResultDark style={{ width: 120, height: 120 }} />}
          description={'暂无稳定性排名数据'}
          style={{ padding: 30 }}
        />
      ) : (
        <CardTable
          columns={columns}
          dataSource={dataSource}
          rowKey='id'
          loading={loading}
          pagination={false}
          scroll={{ x: 'max-content' }}
        />
      )}
    </Card>
  );
};

export default StabilityRankTable;
