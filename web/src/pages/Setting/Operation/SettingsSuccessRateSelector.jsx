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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  Col,
  Form,
  Row,
  Spin,
  Typography,
} from '@douyinfe/semi-ui';
import JSONEditor from '../../../components/common/ui/JSONEditor';
import {
  API,
  compareObjects,
  showError,
  showSuccess,
  showWarning,
  verifyJSON,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import {
  CHANNEL_SUCCESS_RATE_HEALTH_MANAGER_TEMPLATE,
  CHANNEL_SUCCESS_RATE_IMMEDIATE_DISABLE_TEMPLATE,
  CHANNEL_SUCCESS_RATE_JSON_OPTION_KEYS,
  CHANNEL_SUCCESS_RATE_KEYS,
  CHANNEL_SUCCESS_RATE_PRIORITY_WEIGHTS_TEMPLATE,
  getDefaultChannelSuccessRateOptions,
} from '../../../constants';

const KEY_ENABLED = CHANNEL_SUCCESS_RATE_KEYS.ENABLED;
const KEY_HALF_LIFE = CHANNEL_SUCCESS_RATE_KEYS.HALF_LIFE;
const KEY_EXPLORE_RATE = CHANNEL_SUCCESS_RATE_KEYS.EXPLORE_RATE;
const KEY_QUICK_DOWNGRADE = CHANNEL_SUCCESS_RATE_KEYS.QUICK_DOWNGRADE;
const KEY_CONSECUTIVE_FAIL_THRESHOLD = CHANNEL_SUCCESS_RATE_KEYS.CONSECUTIVE_FAIL_THRESHOLD;
const KEY_PRIORITY_WEIGHTS = CHANNEL_SUCCESS_RATE_KEYS.PRIORITY_WEIGHTS;
const KEY_IMMEDIATE_DISABLE = CHANNEL_SUCCESS_RATE_KEYS.IMMEDIATE_DISABLE;
const KEY_HEALTH_MANAGER = CHANNEL_SUCCESS_RATE_KEYS.HEALTH_MANAGER;

const IMMEDIATE_DISABLE_KEY_DESCRIPTIONS = {
  enabled: '开启后，命中下列任一状态码、错误码或错误类型时会立即触发禁用。',
  status_codes: '常见硬失败状态码：鉴权失败、权限不足、限流；可先填写 401、403、429。',
  error_codes:
    '可按上游错误码补充，例如 insufficient_quota、rate_limit_exceeded。',
  error_types: '可按上游错误类型补充，例如 billing_error。',
};

const HEALTH_MANAGER_KEY_DESCRIPTIONS = {
  circuit_scope:
    '熔断范围。model 表示按 group + model + channel 熔断；channel 表示某个 model 熔断后整条渠道一起熔断。',
  disable_threshold: '控制停用阈值。',
  enable_threshold: '兼容保留字段，当前不作为恢复判定主条件。',
  min_sample_size: '至少积累多少个样本再做自动禁用判断。',
  recovery_check_interval: '基础恢复间隔，单位为秒；到期后进入半开探测，而不是直接恢复。',
  half_open_success_threshold: '半开阶段连续成功多少次后恢复正常。',
};

export default function SettingsSuccessRateSelector(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const refForm = useRef();
  const readOnly = props.readOnly === true;
  const sectionTitle = props.sectionTitle || t('SuccessRateSelector 设置');
  const [inputs, setInputs] = useState(getDefaultChannelSuccessRateOptions);
  const [inputsRow, setInputsRow] = useState(inputs);

  const handleFieldChange = (fieldName) => (value) => {
    setInputs((prev) => ({ ...prev, [fieldName]: value }));
  };

  const jsonFields = useMemo(() => CHANNEL_SUCCESS_RATE_JSON_OPTION_KEYS, []);

  const validateJsonFields = () => {
    for (const field of jsonFields) {
      const raw = inputs[field] || '';
      if (!verifyJSON(raw || '{}')) {
        return { ok: false, field };
      }
    }
    return { ok: true };
  };

  const onSubmit = () => {
    if (readOnly) {
      return showWarning(t('当前账号仅可查看，无法保存此页面配置'));
    }
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));

    const validation = validateJsonFields();
    if (!validation.ok) {
      return showError(t('JSON 配置格式不正确'));
    }

    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (typeof inputs[item.key] === 'boolean') {
        value = String(inputs[item.key]);
      } else if (jsonFields.includes(item.key)) {
        value = JSON.stringify(JSON.parse(inputs[item.key] || '{}'));
      } else {
        value = String(inputs[item.key]);
      }
      return API.put('/api/option/', {
        key: item.key,
        value,
      });
    });

    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (requestQueue.length === 1) {
          if (res.includes(undefined)) return;
        } else if (requestQueue.length > 1) {
          if (res.includes(undefined)) {
            return showError(t('部分保存失败，请重试'));
          }
        }
        showSuccess(t('保存成功'));
        props.refresh?.();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  };

  useEffect(() => {
    const nextInputs = { ...inputs };
    Object.keys(nextInputs).forEach((key) => {
      if (!(key in (props.options || {}))) {
        return;
      }
      const value = props.options[key];
      if (jsonFields.includes(key)) {
        if (!value) {
          return;
        }
        try {
          nextInputs[key] = JSON.stringify(JSON.parse(value), null, 2);
        } catch {
          nextInputs[key] = value;
        }
        return;
      }
      nextInputs[key] = value;
    });
    setInputs(nextInputs);
    setInputsRow(structuredClone(nextInputs));
    refForm.current?.setValues(nextInputs);
  }, [props.options]);

  return (
    <Spin spinning={loading}>
      <Form
        values={inputs}
        getFormApi={(formAPI) => (refForm.current = formAPI)}
        style={{ marginBottom: 0 }}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <Banner
            type='info'
            bordered
            closeIcon={null}
            description={t(
              '用于配置成功率择优、即时熔断范围，以及基础间隔 + 半开探测恢复策略。priority_weights 仅在同一候选集合内生效。',
            )}
          />

          <div>
            <div
              style={{
                fontSize: 20,
                fontWeight: 600,
                marginBottom: 6,
                color: 'var(--semi-color-text-0)',
              }}
            >
              {sectionTitle}
            </div>
            <Typography.Text type='secondary'>
              {t('基础参数、即时熔断和恢复策略已按场景拆分为多行，便于分组查看与调整。')}
            </Typography.Text>
          </div>

          <Card
            bordered
            style={{ width: '100%', borderRadius: 16 }}
            bodyStyle={{ padding: 18 }}
            title={t('基础择优')}
          >
            <Row gutter={[16, 12]}>
              <Col xs={24} md={12} xl={8}>
                <Form.Switch
                  field={KEY_ENABLED}
                  label={t('启用 SuccessRateSelector')}
                  checkedText='｜'
                  uncheckedText='〇'
                  disabled={readOnly}
                  onChange={handleFieldChange(KEY_ENABLED)}
                />
              </Col>
              <Col xs={24} md={12} xl={8}>
                <Form.InputNumber
                  field={KEY_HALF_LIFE}
                  label={t('半衰期')}
                  min={1}
                  step={60}
                  suffix={t('秒')}
                  disabled={readOnly}
                  extraText={t('成功和失败样本的时间衰减窗口')}
                  onChange={handleFieldChange(KEY_HALF_LIFE)}
                />
              </Col>
              <Col xs={24} md={12} xl={8}>
                <Form.InputNumber
                  field={KEY_EXPLORE_RATE}
                  label={t('探索率')}
                  min={0}
                  max={1}
                  step={0.01}
                  disabled={readOnly}
                  extraText={t('小概率随机探索其它候选通道，例如 0.02 表示 2% 的请求会随机选渠道，给低分渠道重新积累样本的机会')}
                  onChange={handleFieldChange(KEY_EXPLORE_RATE)}
                />
              </Col>
              <Col xs={24} md={12} xl={8}>
                <Form.Switch
                  field={KEY_QUICK_DOWNGRADE}
                  label={t('启用快速降权')}
                  checkedText='｜'
                  uncheckedText='〇'
                  disabled={readOnly}
                  onChange={handleFieldChange(KEY_QUICK_DOWNGRADE)}
                />
              </Col>
              <Col xs={24} md={12} xl={8}>
                <Form.InputNumber
                  field={KEY_CONSECUTIVE_FAIL_THRESHOLD}
                  label={t('连续失败阈值')}
                  min={1}
                  step={1}
                  disabled={readOnly}
                  extraText={t('达到阈值后会追加连续失败惩罚')}
                  onChange={handleFieldChange(KEY_CONSECUTIVE_FAIL_THRESHOLD)}
                />
              </Col>
            </Row>
          </Card>

          <Card
            bordered
            style={{ width: '100%', borderRadius: 16 }}
            bodyStyle={{ padding: 18 }}
            title={t('进阶策略')}
          >
            <JSONEditor
              field={KEY_PRIORITY_WEIGHTS}
              label={t('priority_weights（按 priority 调整得分权重，键为优先级，值为加成比例）')}
              value={inputs[KEY_PRIORITY_WEIGHTS]}
              onChange={handleFieldChange(KEY_PRIORITY_WEIGHTS)}
              formApi={refForm.current}
              placeholder={t('请输入 JSON 对象')}
              template={CHANNEL_SUCCESS_RATE_PRIORITY_WEIGHTS_TEMPLATE}
              templateLabel={t('填入推荐模板')}
              disabled={readOnly}
              extraText={t('示例：10: 0.2 表示 priority=10 的渠道额外加 0.2 分；0: -0.1 表示 priority=0 的渠道额外减 0.1 分。')}
            />
          </Card>

          <Card
            bordered
            style={{ width: '100%', borderRadius: 16 }}
            bodyStyle={{ padding: 18 }}
            title={t('即时熔断')}
          >
            <JSONEditor
              field={KEY_IMMEDIATE_DISABLE}
              label={t('immediate_disable（命中状态码、错误码或错误类型后立即触发禁用）')}
              value={inputs[KEY_IMMEDIATE_DISABLE]}
              onChange={handleFieldChange(KEY_IMMEDIATE_DISABLE)}
              formApi={refForm.current}
              placeholder={t('请输入 JSON 对象')}
              template={CHANNEL_SUCCESS_RATE_IMMEDIATE_DISABLE_TEMPLATE}
              templateLabel={t('填入推荐模板')}
              disabled={readOnly}
              keyDescriptions={IMMEDIATE_DISABLE_KEY_DESCRIPTIONS}
            />
          </Card>

          <Card
            bordered
            style={{ width: '100%', borderRadius: 16 }}
            bodyStyle={{ padding: 18 }}
            title={t('自动恢复')}
          >
            <JSONEditor
              field={KEY_HEALTH_MANAGER}
              label={t('health_manager（熔断范围、临时熔断阈值与基础间隔 + 半开探测恢复策略）')}
              value={inputs[KEY_HEALTH_MANAGER]}
              onChange={handleFieldChange(KEY_HEALTH_MANAGER)}
              formApi={refForm.current}
              placeholder={t('请输入 JSON 对象')}
              template={CHANNEL_SUCCESS_RATE_HEALTH_MANAGER_TEMPLATE}
              templateLabel={t('填入推荐模板')}
              disabled={readOnly}
              keyDescriptions={HEALTH_MANAGER_KEY_DESCRIPTIONS}
            />
          </Card>

          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button type='primary' onClick={onSubmit} disabled={readOnly}>
              {readOnly ? t('当前账号仅可查看') : t('保存渠道设置')}
            </Button>
          </div>
        </div>
      </Form>
    </Spin>
  );
}
