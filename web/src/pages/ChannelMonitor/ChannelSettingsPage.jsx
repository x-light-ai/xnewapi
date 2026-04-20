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

import React, { useEffect, useState } from 'react';
import { Banner, Card, Spin, Typography } from '@douyinfe/semi-ui';
import { API, isRoot, showError, toBoolean } from '../../helpers';
import SettingsSuccessRateSelector from '../Setting/Operation/SettingsSuccessRateSelector';

const ChannelSettingsPage = () => {
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    'channel_success_rate_setting.enabled': false,
    'channel_success_rate_setting.half_life_seconds': 1800,
    'channel_success_rate_setting.explore_rate': 0.02,
    'channel_success_rate_setting.quick_downgrade': true,
    'channel_success_rate_setting.consecutive_fail_threshold': 3,
    'channel_success_rate_setting.priority_weights': '',
    'channel_success_rate_setting.immediate_disable': '',
    'channel_success_rate_setting.health_manager': '',
    RetryTimes: 0,
  });

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      const nextInputs = {};
      data.forEach((item) => {
        if (typeof inputs[item.key] === 'boolean') {
          nextInputs[item.key] = toBoolean(item.value);
        } else {
          nextInputs[item.key] = item.value;
        }
      });
      setInputs((prev) => ({ ...prev, ...nextInputs }));
    } else {
      showError(message);
    }
  };

  const onRefresh = async () => {
    try {
      setLoading(true);
      await getOptions();
    } catch (error) {
      showError('刷新失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    onRefresh();
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
          <div>
            <div className='text-2xl font-semibold'>{'渠道设置'}</div>
            <Typography.Text type='secondary'>
              {'集中管理 SuccessRateSelector 的择优、立即禁用和自动恢复参数。'}
            </Typography.Text>
          </div>
        </Card>

        {!isRoot() ? (
          <Banner
            type='warning'
            fullMode={false}
            bordered
            closeIcon={null}
            title={'当前账号仅可查看'}
            description={'此页面读取的是 Root 级系统配置，非 Root 管理员无法保存修改。'}
          />
        ) : null}

        <Spin spinning={loading}>
          <SettingsSuccessRateSelector
            options={inputs}
            refresh={onRefresh}
            layout='page'
            readOnly={!isRoot()}
          />
        </Spin>
      </div>
    </div>
  );
};

export default ChannelSettingsPage;
