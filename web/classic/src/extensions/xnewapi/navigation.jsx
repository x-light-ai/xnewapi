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

// FORK-CUSTOM: Own xnewapi admin routes and navigation outside upstream shell components.
import React from 'react';
import { Route } from 'react-router-dom';
import { BarChart3 } from 'lucide-react';
import { AdminRoute } from '../../helpers';
import ChannelMonitorPage from '../../pages/ChannelMonitor';
import ChannelSettingsPage from '../../pages/ChannelMonitor/ChannelSettingsPage';

export const xnewapiRouterMap = {
  channel_monitor: '/console/channel-monitor',
  channel_settings: '/console/channel-settings',
};

const renderXNewApiAdminIcon = (selected) => (
  <BarChart3
    size={16}
    strokeWidth={2}
    color={selected ? 'var(--semi-color-primary)' : 'currentColor'}
    className={`transition-colors duration-200 ${selected ? 'transition-transform duration-200 scale-105' : ''}`}
  />
);

export const getXNewApiAdminItems = (t, isAdmin) => [
  {
    text: t('渠道监控'),
    itemKey: 'channel_monitor',
    to: '/console/channel-monitor',
    className: isAdmin() ? '' : 'tableHiddle',
    icon: renderXNewApiAdminIcon,
  },
  {
    text: t('渠道设置'),
    itemKey: 'channel_settings',
    to: '/console/channel-settings',
    className: isAdmin() ? '' : 'tableHiddle',
    icon: renderXNewApiAdminIcon,
  },
];

export const xnewapiAdminRoutes = [
  <Route
    key='xnewapi-channel-monitor'
    path='/console/channel-monitor'
    element={
      <AdminRoute>
        <ChannelMonitorPage />
      </AdminRoute>
    }
  />,
  <Route
    key='xnewapi-channel-settings'
    path='/console/channel-settings'
    element={
      <AdminRoute>
        <ChannelSettingsPage />
      </AdminRoute>
    }
  />,
];
