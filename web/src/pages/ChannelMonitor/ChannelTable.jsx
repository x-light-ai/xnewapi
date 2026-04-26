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
import { Card, Empty } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import CardTable from '../../components/common/ui/CardTable';

const ChannelTable = ({
  title,
  emptyDescription,
  loading,
  channels,
  columns,
}) => {
  return (
    <Card
      loading={loading}
      className='!rounded-2xl'
      title={title}
      bodyStyle={{ padding: 12 }}
    >
      {channels.length === 0 && !loading ? (
        <Empty
          image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
          darkModeImage={
            <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
          }
          description={emptyDescription}
          style={{ padding: 30 }}
        />
      ) : (
        <CardTable
          columns={columns}
          dataSource={channels}
          rowKey='id'
          loading={loading}
          className='rounded-xl overflow-hidden'
          size='middle'
          pagination={false}
          scroll={{ x: '100%' }}
        />
      )}
    </Card>
  );
};

export default ChannelTable;
