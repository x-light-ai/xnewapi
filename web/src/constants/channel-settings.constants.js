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

export const CHANNEL_SUCCESS_RATE_KEYS = {
  ENABLED: 'channel_success_rate_setting.enabled',
  HALF_LIFE: 'channel_success_rate_setting.half_life_seconds',
  EXPLORE_RATE: 'channel_success_rate_setting.explore_rate',
  QUICK_DOWNGRADE: 'channel_success_rate_setting.quick_downgrade',
  CONSECUTIVE_FAIL_THRESHOLD: 'channel_success_rate_setting.consecutive_fail_threshold',
  PRIORITY_WEIGHTS: 'channel_success_rate_setting.priority_weights',
  IMMEDIATE_DISABLE: 'channel_success_rate_setting.immediate_disable',
  HEALTH_MANAGER: 'channel_success_rate_setting.health_manager',
};

export const CHANNEL_SUCCESS_RATE_BOOLEAN_OPTION_KEYS = [
  CHANNEL_SUCCESS_RATE_KEYS.ENABLED,
  CHANNEL_SUCCESS_RATE_KEYS.QUICK_DOWNGRADE,
];

export const CHANNEL_SUCCESS_RATE_JSON_OPTION_KEYS = [
  CHANNEL_SUCCESS_RATE_KEYS.PRIORITY_WEIGHTS,
  CHANNEL_SUCCESS_RATE_KEYS.IMMEDIATE_DISABLE,
  CHANNEL_SUCCESS_RATE_KEYS.HEALTH_MANAGER,
];

export const CHANNEL_SUCCESS_RATE_PRIORITY_WEIGHTS_TEMPLATE = {
  10: 0.2,
  5: 0,
  0: -0.1,
};

export const CHANNEL_SUCCESS_RATE_IMMEDIATE_DISABLE_TEMPLATE = {
  enabled: true,
  status_codes: [401, 403, 429],
  error_codes: [],
  error_types: [],
};

export const CHANNEL_SUCCESS_RATE_HEALTH_MANAGER_TEMPLATE = {
  circuit_scope: 'model',
  disable_threshold: 0.2,
  enable_threshold: 0.7,
  min_sample_size: 10,
  recovery_check_interval: 600,
  half_open_success_threshold: 2,
};

export const getDefaultChannelSuccessRateOptions = () => ({
  [CHANNEL_SUCCESS_RATE_KEYS.ENABLED]: false,
  [CHANNEL_SUCCESS_RATE_KEYS.HALF_LIFE]: 1800,
  [CHANNEL_SUCCESS_RATE_KEYS.EXPLORE_RATE]: 0.02,
  [CHANNEL_SUCCESS_RATE_KEYS.QUICK_DOWNGRADE]: true,
  [CHANNEL_SUCCESS_RATE_KEYS.CONSECUTIVE_FAIL_THRESHOLD]: 3,
  [CHANNEL_SUCCESS_RATE_KEYS.PRIORITY_WEIGHTS]: JSON.stringify(
    CHANNEL_SUCCESS_RATE_PRIORITY_WEIGHTS_TEMPLATE,
    null,
    2,
  ),
  [CHANNEL_SUCCESS_RATE_KEYS.IMMEDIATE_DISABLE]: JSON.stringify(
    CHANNEL_SUCCESS_RATE_IMMEDIATE_DISABLE_TEMPLATE,
    null,
    2,
  ),
  [CHANNEL_SUCCESS_RATE_KEYS.HEALTH_MANAGER]: JSON.stringify(
    CHANNEL_SUCCESS_RATE_HEALTH_MANAGER_TEMPLATE,
    null,
    2,
  ),
});

export const getDefaultChannelFormInputs = () => ({
  name: '',
  type: 1,
  key: '',
  openai_organization: '',
  max_input_tokens: 0,
  base_url: '',
  other: '',
  model_mapping: '',
  param_override: '',
  status_code_mapping: '',
  models: [],
  auto_ban: 1,
  test_model: '',
  groups: ['default'],
  priority: 0,
  weight: 0,
  tag: '',
  multi_key_mode: 'random',
  force_format: false,
  thinking_to_content: false,
  proxy: '',
  pass_through_body_enabled: false,
  system_prompt: '',
  system_prompt_override: false,
  settings: '',
  vertex_key_type: 'json',
  aws_key_type: 'ak_sk',
  is_enterprise_account: false,
  allow_service_tier: false,
  disable_store: false,
  allow_safety_identifier: false,
  allow_include_obfuscation: false,
  allow_inference_geo: false,
  allow_speed: false,
  claude_beta_query: false,
  upstream_model_update_check_enabled: false,
  upstream_model_update_auto_sync_enabled: false,
  upstream_model_update_last_check_time: 0,
  upstream_model_update_last_detected_models: [],
  upstream_model_update_ignored_models: '',
});
