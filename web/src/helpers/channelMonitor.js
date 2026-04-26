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
  all = false,
  group = '',
} = {}) {
  const response = await API.get('/api/channel_monitor/channels', {
    params: {
      days,
      all,
      group,
    },
  });
  const { success, data, message } = response.data || {};
  if (!success) {
    throw new Error(message || 'Failed to fetch channel monitor channels');
  }
  return {
    items: Array.isArray(data?.items) ? data.items : [],
    total: Number(data?.total || 0),
    groups: Array.isArray(data?.groups) ? data.groups : [],
  };
}

export async function fetchChannelMonitorTimeline({
  hours = 24,
  bucketMinutes = 10,
  limit = 20,
  group = '',
} = {}) {
  const response = await API.get('/api/channel_monitor/timeline', {
    params: {
      hours,
      bucket_minutes: bucketMinutes,
      limit,
      group,
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

export async function fetchChannelMonitorSelectionLogs({
  channelId,
  model,
  group,
  outcome,
  abnormalOnly = false,
  limit = 100,
} = {}) {
  const response = await API.get('/api/channel_monitor/selection_logs', {
    params: {
      channel_id: channelId || undefined,
      model: model || undefined,
      group: group || undefined,
      outcome: outcome && outcome !== 'all' ? outcome : undefined,
      abnormal_only: abnormalOnly || undefined,
      limit,
    },
  });
  const { success, data, message } = response.data || {};
  if (!success) {
    throw new Error(message || 'Failed to fetch channel selection logs');
  }
  return Array.isArray(data) ? data : [];
}

export async function setChannelScoreOverride(channelId, score) {
  const response = await API.post(`/api/channel_monitor/channels/${channelId}/score_override`, { score });
  const { success, message } = response.data || {};
  if (!success) throw new Error(message || 'Failed to set score override');
}
