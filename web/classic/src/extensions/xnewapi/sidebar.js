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

// FORK-CUSTOM: Own xnewapi sidebar defaults outside the upstream hook.
export const XNEWAPI_ADMIN_SIDEBAR_DEFAULTS = {
  channel_monitor: true,
  channel_settings: true,
};

export const mergeXNewApiSidebarConfig = (modules) => ({
  ...modules,
  admin: {
    ...(modules?.admin || {}),
    channel_monitor:
      modules?.admin?.channel_monitor ??
      XNEWAPI_ADMIN_SIDEBAR_DEFAULTS.channel_monitor,
    channel_settings:
      modules?.admin?.channel_settings ??
      XNEWAPI_ADMIN_SIDEBAR_DEFAULTS.channel_settings,
  },
});

export const getXNewApiSidebarSettingItems = (t) => [
  {
    key: 'channel_monitor',
    title: t('渠道监控'),
    description: t('渠道健康与成功率监控'),
  },
  {
    key: 'channel_settings',
    title: t('渠道设置'),
    description: t('渠道择优与恢复策略配置'),
  },
];
