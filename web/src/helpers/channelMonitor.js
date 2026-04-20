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

import { API } from './api';

export async function fetchChannelMonitorSummary(days = 7) {
  const response = await API.get('/api/channel_monitor/summary', {
    params: { days },
  });
  const { success, data, message } = response.data || {};
  if (!success) {
    throw new Error(message || 'Failed to fetch channel monitor summary');
  }
  return data;
}

export async function fetchChannelMonitorHealth(days = 7) {
  const response = await API.get('/api/channel_monitor/health', {
    params: { days },
  });
  const { success, data, message } = response.data || {};
  if (!success) {
    throw new Error(message || 'Failed to fetch channel monitor health');
  }
  return Array.isArray(data) ? data : [];
}

export async function fetchChannelMonitorChannels({
  days = 7,
  page = 1,
  pageSize = 20,
  sort = 'request_count',
  order = 'desc',
} = {}) {
  const response = await API.get('/api/channel_monitor/channels', {
    params: {
      days,
      page,
      page_size: pageSize,
      sort,
      order,
    },
  });
  const { success, data, message } = response.data || {};
  if (!success) {
    throw new Error(message || 'Failed to fetch channel monitor channels');
  }
  return {
    items: Array.isArray(data?.items) ? data.items : [],
    total: Number(data?.total || 0),
    page: Number(data?.page || page),
    pageSize: Number(data?.page_size || pageSize),
  };
}

export async function fetchChannelMonitorTimeline({
  hours = 24,
  bucketMinutes = 10,
  limit = 20,
} = {}) {
  const response = await API.get('/api/channel_monitor/timeline', {
    params: {
      hours,
      bucket_minutes: bucketMinutes,
      limit,
    },
  });
  const { success, data, message } = response.data || {};
  if (!success) {
    throw new Error(message || 'Failed to fetch channel monitor timeline');
  }
  return Array.isArray(data) ? data : [];
}

export async function fetchChannelMonitorRankings({ days = 1, top = 10 } = {}) {
  const response = await API.get('/api/channel_monitor/rankings', {
    params: { days, top },
  });
  const { success, data, message } = response.data || {};
  if (!success) {
    throw new Error(message || 'Failed to fetch channel monitor rankings');
  }
  return {
    stability: Array.isArray(data?.stability) ? data.stability : [],
    latency: Array.isArray(data?.latency) ? data.latency : [],
  };
}
