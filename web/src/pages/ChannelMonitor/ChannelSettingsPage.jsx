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
import {
  CHANNEL_SUCCESS_RATE_BOOLEAN_OPTION_KEYS,
  getDefaultChannelSuccessRateOptions,
} from '../../constants';
import SettingsSuccessRateSelector from '../Setting/Operation/SettingsSuccessRateSelector';

const ChannelSettingsPage = () => {
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState(getDefaultChannelSuccessRateOptions);

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      const nextInputs = {};
      data.forEach((item) => {
        if (CHANNEL_SUCCESS_RATE_BOOLEAN_OPTION_KEYS.includes(item.key)) {
          nextInputs[item.key] = toBoolean(item.value);
        } else if (item.key in getDefaultChannelSuccessRateOptions()) {
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
