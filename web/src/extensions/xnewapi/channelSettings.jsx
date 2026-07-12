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

// FORK-CUSTOM: Own third-party Codex channel form behavior outside the upstream modal.
import React from 'react';
import { Form } from '@douyinfe/semi-ui';

const DEFAULT_UPSTREAM_PROTOCOL = 'anthropic';

export const applyXNewApiChannelSettings = (data, settings = {}) => {
  data.upstream_protocol =
    settings.upstream_protocol || DEFAULT_UPSTREAM_PROTOCOL;
};

export const resetXNewApiChannelSettings = (data) => {
  data.upstream_protocol = DEFAULT_UPSTREAM_PROTOCOL;
};

export const saveXNewApiChannelSettings = (settings, inputs) => {
  if (inputs.type === 14) {
    settings.upstream_protocol =
      inputs.upstream_protocol === 'codex'
        ? 'codex'
        : DEFAULT_UPSTREAM_PROTOCOL;
  } else {
    delete settings.upstream_protocol;
  }
};

export const cleanupXNewApiChannelInputs = (inputs) => {
  delete inputs.upstream_protocol;
};

export const hasXNewApiAdvancedSettings = (data) =>
  data.upstream_protocol === 'codex';

export const XNewApiUpstreamProtocolField = ({ t, onChange }) => (
  <Form.Select
    field='upstream_protocol'
    label={t('上游协议类型')}
    optionList={[
      { label: t('Anthropic Claude'), value: 'anthropic' },
      { label: t('Codex'), value: 'codex' },
    ]}
    onChange={(value) => onChange(value || DEFAULT_UPSTREAM_PROTOCOL)}
    extraText={t(
      '客户端仍使用 Claude /v1/messages；当真实上游是 Codex 时选择 Codex 以启用 Codex 与 Claude 的直接转换',
    )}
  />
);
