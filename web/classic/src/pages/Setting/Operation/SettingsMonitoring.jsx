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

import React, { useEffect, useState, useRef } from 'react';
import {
  Button,
  Col,
  Form,
  Input,
  Row,
  Space,
  Spin,
  Switch,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
  parseHttpStatusCodeRules,
  toBoolean,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import HttpStatusCodeRulesInput from '../../../components/settings/HttpStatusCodeRulesInput';

const { Text } = Typography;

const secretOptionKeys = new Set([
  'monitor_setting.request_failure_webhook_secret',
  'monitor_setting.channel_disabled_webhook_secret',
]);

const trimOptionKeys = new Set([
  'monitor_setting.request_failure_webhook_url',
  'monitor_setting.request_failure_webhook_secret',
  'monitor_setting.channel_disabled_webhook_url',
  'monitor_setting.channel_disabled_webhook_secret',
]);

const createPolicyGroupId = () =>
  `policy_${Date.now().toString(36)}_${Math.random()
    .toString(36)
    .slice(2, 8)}`;

function normalizeKeywords(value) {
  return Array.isArray(value)
    ? value.map((item) => String(item || ''))
    : String(value || '').split('\n');
}

function normalizePolicyGroups(groups) {
  if (!Array.isArray(groups)) {
    throw new Error('policy groups must be an array');
  }
  const ids = new Set();
  const names = new Set();
  return groups.map((group, index) => {
    const id = String(group?.id || '').trim();
    const name = String(group?.name || '').trim();
    if (!id) {
      throw new Error(`policy group #${index + 1} id is required`);
    }
    if (!name) {
      throw new Error(`policy group #${index + 1} name is required`);
    }
    if (ids.has(id)) {
      throw new Error(`duplicate policy group id: ${id}`);
    }
    const nameKey = name.toLowerCase();
    if (names.has(nameKey)) {
      throw new Error(`duplicate policy group name: ${name}`);
    }
    ids.add(id);
    names.add(nameKey);
    return {
      id,
      name,
      enabled: group?.enabled !== false,
      keywords: normalizeKeywords(group?.keywords || []),
    };
  });
}

function normalizePolicyGroupsString(value) {
  const raw = String(value || '').trim();
  if (!raw) {
    return { ok: true, value: '[]', groups: [] };
  }
  try {
    const groups = normalizePolicyGroups(JSON.parse(raw));
    return {
      ok: true,
      value: JSON.stringify(groups, null, 2),
      groups,
    };
  } catch (error) {
    return {
      ok: false,
      value: raw,
      groups: [],
      error: error?.message || String(error),
    };
  }
}

function parsePolicyGroups(value) {
  const result = normalizePolicyGroupsString(value);
  return result.ok ? result.groups : [];
}

function serializePolicyGroups(groups) {
  return JSON.stringify(
    groups.map((group) => ({
      id: group.id,
      name: group.name,
      enabled: group.enabled !== false,
      keywords: Array.isArray(group.keywords)
        ? group.keywords.map((keyword) => String(keyword || ''))
        : [],
    })),
    null,
    2,
  );
}

const defaultMonitoringInputs = {
  ChannelDisableThreshold: '',
  QuotaRemindThreshold: '',
  AutomaticDisableChannelEnabled: false,
  AutomaticEnableChannelEnabled: false,
  AutomaticDisableKeywords: '',
  AutomaticDisablePolicyGroups: '[]',
  AutomaticDisableStatusCodes: '401',
  AutomaticRetryStatusCodes:
    '100-199,300-399,401-407,409-499,500-503,505-523,525-599',
  'monitor_setting.channel_failure_rate_disable_enabled': false,
  'monitor_setting.channel_failure_rate_window_minutes': 5,
  'monitor_setting.channel_failure_rate_threshold': 50,
  'monitor_setting.channel_failure_rate_min_requests': 20,
  'monitor_setting.request_failure_webhook_enabled': false,
  'monitor_setting.request_failure_webhook_url': '',
  'monitor_setting.request_failure_webhook_secret': '',
  'monitor_setting.channel_disabled_webhook_enabled': false,
  'monitor_setting.channel_disabled_webhook_url': '',
  'monitor_setting.channel_disabled_webhook_secret': '',
  'monitor_setting.auto_test_channel_enabled': false,
  'monitor_setting.auto_test_channel_minutes': 10,
};

function normalizeMonitoringInputs(values = {}) {
  const normalized = { ...defaultMonitoringInputs };

  for (const key of Object.keys(defaultMonitoringInputs)) {
    const fallback = defaultMonitoringInputs[key];
    const value = values[key];

    if (typeof fallback === 'boolean') {
      normalized[key] = toBoolean(value);
      continue;
    }

    if (typeof fallback === 'number') {
      const parsed = Number(value);
      normalized[key] = Number.isFinite(parsed) ? parsed : fallback;
      continue;
    }

    const stringValue =
      value === undefined || value === null ? '' : String(value);
    if (key === 'AutomaticDisablePolicyGroups') {
      const result = normalizePolicyGroupsString(stringValue);
      normalized[key] = result.ok ? result.value : stringValue;
      continue;
    }
    normalized[key] = trimOptionKeys.has(key)
      ? stringValue.trim()
      : stringValue;
  }

  return normalized;
}

function AutoDisablePolicyGroupsEditor({ value, onChange }) {
  const { t } = useTranslation();
  const groups = parsePolicyGroups(value);

  const updateGroups = (nextGroups) => {
    onChange(serializePolicyGroups(nextGroups));
  };

  const updateGroup = (index, patch) => {
    const nextGroups = groups.map((group, groupIndex) =>
      groupIndex === index ? { ...group, ...patch } : group,
    );
    updateGroups(nextGroups);
  };

  const addGroup = () => {
    const baseName = t('新策略组');
    const usedNames = new Set(groups.map((group) => group.name.toLowerCase()));
    let name = baseName;
    let suffix = 2;
    while (usedNames.has(name.toLowerCase())) {
      name = `${baseName} ${suffix}`;
      suffix += 1;
    }
    updateGroups([
      ...groups,
      {
        id: createPolicyGroupId(),
        name,
        enabled: true,
        keywords: [],
      },
    ]);
  };

  const removeGroup = (index) => {
    updateGroups(groups.filter((_, groupIndex) => groupIndex !== index));
  };

  return (
    <div
      style={{
        marginTop: 16,
        padding: 16,
        border: '1px solid var(--semi-color-border)',
        borderRadius: 8,
        background: 'var(--semi-color-fill-0)',
      }}
    >
      <div className='flex items-center justify-between gap-2 flex-wrap'>
        <Space vertical spacing={2} align='start'>
          <Text strong>{t('自动禁用策略组')}</Text>
          <Text type='tertiary' size='small'>
            {t('策略组用于让指定渠道使用专用失败关键词，不影响默认关键词')}
          </Text>
          <Text type='tertiary' size='small'>
            {t('已禁用的策略组不会生效，绑定该组的渠道会回退到默认关键词')}
          </Text>
        </Space>
        <Button theme='solid' type='primary' onClick={addGroup}>
          {t('添加策略组')}
        </Button>
      </div>

      {groups.length === 0 ? (
        <div style={{ paddingTop: 16 }}>
          <Text type='tertiary'>{t('暂无自动禁用策略组')}</Text>
        </div>
      ) : (
        <Space vertical style={{ width: '100%', marginTop: 16 }} spacing={12}>
          {groups.map((group, index) => (
            <div
              key={group.id}
              style={{
                width: '100%',
                padding: 12,
                border: '1px solid var(--semi-color-border)',
                borderRadius: 8,
                background: 'var(--semi-color-bg-0)',
              }}
            >
              <Row gutter={12}>
                <Col xs={24} sm={16}>
                  <Text size='small'>{t('策略组名称')}</Text>
                  <Input
                    value={group.name}
                    onChange={(name) => updateGroup(index, { name })}
                    placeholder={t('策略组名称')}
                    style={{ marginTop: 4 }}
                  />
                </Col>
                <Col xs={24} sm={8}>
                  <Text size='small'>{t('状态')}</Text>
                  <div style={{ marginTop: 8 }}>
                    <Switch
                      checked={group.enabled}
                      checkedText={t('开')}
                      uncheckedText={t('关')}
                      onChange={(enabled) => updateGroup(index, { enabled })}
                    />
                  </div>
                </Col>
              </Row>
              <div style={{ marginTop: 12 }}>
                <Text size='small'>{t('关键词')}</Text>
                <TextArea
                  value={group.keywords.join('\n')}
                  autosize={{ minRows: 4, maxRows: 10 }}
                  placeholder={t('一行一个，不区分大小写')}
                  onChange={(keywords) =>
                    updateGroup(index, {
                      keywords: normalizeKeywords(keywords),
                    })
                  }
                  style={{ marginTop: 4 }}
                />
                <Text type='tertiary' size='small'>
                  {t('每行一个关键词，仅对使用该策略组的渠道生效')}
                </Text>
              </div>
              <div style={{ marginTop: 12 }}>
                <Button
                  type='danger'
                  theme='borderless'
                  onClick={() => removeGroup(index)}
                >
                  {t('删除策略组')}
                </Button>
              </div>
            </div>
          ))}
        </Space>
      )}
    </div>
  );
}

export default function SettingsMonitoring(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState(defaultMonitoringInputs);
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);
  const parsedAutoDisableStatusCodes = parseHttpStatusCodeRules(
    inputs.AutomaticDisableStatusCodes || '',
  );
  const parsedAutoRetryStatusCodes = parseHttpStatusCodeRules(
    inputs.AutomaticRetryStatusCodes || '',
  );

  function onSubmit() {
    const normalizedInputs = normalizeMonitoringInputs(inputs);
    const policyGroupsResult = normalizePolicyGroupsString(
      inputs.AutomaticDisablePolicyGroups,
    );
    if (!policyGroupsResult.ok) {
      return showError(
        `${t('自动禁用策略组配置格式不正确')}: ${policyGroupsResult.error}`,
      );
    }
    normalizedInputs.AutomaticDisablePolicyGroups = policyGroupsResult.value;
    const normalizedBaseline = normalizeMonitoringInputs(inputsRow);
    const updateArray = compareObjects(
      normalizedInputs,
      normalizedBaseline,
    ).filter(
      (item) =>
        !(
          secretOptionKeys.has(item.key) &&
          !String(normalizedInputs[item.key] || '').trim()
        ),
    );
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    if (!parsedAutoDisableStatusCodes.ok) {
      const details =
        parsedAutoDisableStatusCodes.invalidTokens &&
        parsedAutoDisableStatusCodes.invalidTokens.length > 0
          ? `: ${parsedAutoDisableStatusCodes.invalidTokens.join(', ')}`
          : '';
      return showError(`${t('自动禁用状态码格式不正确')}${details}`);
    }
    if (!parsedAutoRetryStatusCodes.ok) {
      const details =
        parsedAutoRetryStatusCodes.invalidTokens &&
        parsedAutoRetryStatusCodes.invalidTokens.length > 0
          ? `: ${parsedAutoRetryStatusCodes.invalidTokens.join(', ')}`
          : '';
      return showError(`${t('自动重试状态码格式不正确')}${details}`);
    }
    const webhookValidation = validateWebhookSettings(normalizedInputs);
    if (webhookValidation) {
      return showError(webhookValidation);
    }
    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (typeof normalizedInputs[item.key] === 'boolean') {
        value = String(normalizedInputs[item.key]);
      } else {
        const normalizedMap = {
          AutomaticDisableStatusCodes: parsedAutoDisableStatusCodes.normalized,
          AutomaticRetryStatusCodes: parsedAutoRetryStatusCodes.normalized,
        };
        value = normalizedMap[item.key] ?? normalizedInputs[item.key];
      }
      return API.put('/api/option/', {
        key: item.key,
        value,
      });
    });
    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        const failedResponse = res.find((item) => !item?.data?.success);
        if (failedResponse) {
          return showError(
            failedResponse?.data?.message || t('部分保存失败，请重试'),
          );
        }
        showSuccess(t('保存成功'));
        const nextInputs = {
          ...normalizedInputs,
          'monitor_setting.request_failure_webhook_secret': '',
          'monitor_setting.channel_disabled_webhook_secret': '',
        };
        setInputs(nextInputs);
        setInputsRow(structuredClone(nextInputs));
        refForm.current?.setValues(nextInputs);
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  function validateWebhookSettings(values) {
    const configs = [
      {
        enabledKey: 'monitor_setting.request_failure_webhook_enabled',
        urlKey: 'monitor_setting.request_failure_webhook_url',
        label: t('请求失败 Webhook 地址'),
      },
      {
        enabledKey: 'monitor_setting.channel_disabled_webhook_enabled',
        urlKey: 'monitor_setting.channel_disabled_webhook_url',
        label: t('渠道禁用 Webhook 地址'),
      },
    ];
    for (const config of configs) {
      if (!values[config.enabledKey]) {
        continue;
      }
      const rawUrl = String(values[config.urlKey] || '').trim();
      if (rawUrl === '') {
        return `${config.label}${t('不能为空')}`;
      }
      if (!rawUrl.startsWith('https://')) {
        return `${config.label}${t('必须以https://开头')}`;
      }
    }
    return '';
  }

  useEffect(() => {
    const currentInputs = normalizeMonitoringInputs(props.options);
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    refForm.current?.setValues(currentInputs);
  }, [props.options]);

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('监控设置')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'monitor_setting.auto_test_channel_enabled'}
                  label={t('定时测试所有通道')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'monitor_setting.auto_test_channel_enabled': value,
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('自动测试所有通道间隔时间')}
                  step={1}
                  min={1}
                  suffix={t('分钟')}
                  extraText={t('每隔多少分钟测试一次所有通道')}
                  placeholder={''}
                  field={'monitor_setting.auto_test_channel_minutes'}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'monitor_setting.auto_test_channel_minutes':
                        parseInt(value),
                    })
                  }
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('测试所有渠道的最长响应时间')}
                  step={1}
                  min={0}
                  suffix={t('秒')}
                  extraText={t(
                    '当运行通道全部测试时，超过此时间将自动禁用通道',
                  )}
                  placeholder={''}
                  field={'ChannelDisableThreshold'}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      ChannelDisableThreshold: String(value),
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('额度提醒阈值')}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={t('低于此额度时将发送邮件提醒用户')}
                  placeholder={''}
                  field={'QuotaRemindThreshold'}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      QuotaRemindThreshold: String(value),
                    })
                  }
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'AutomaticDisableChannelEnabled'}
                  label={t('失败时自动禁用通道')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={(value) => {
                    setInputs({
                      ...inputs,
                      AutomaticDisableChannelEnabled: value,
                    });
                  }}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'AutomaticEnableChannelEnabled'}
                  label={t('成功时自动启用通道')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      AutomaticEnableChannelEnabled: value,
                    })
                  }
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'monitor_setting.channel_failure_rate_disable_enabled'}
                  label={t('按失败率自动禁用渠道')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  extraText={t(
                    '窗口内请求数达到最小样本数，且失败率达到阈值后自动禁用渠道',
                  )}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'monitor_setting.channel_failure_rate_disable_enabled':
                        value,
                    })
                  }
                />
              </Col>
            </Row>
            {inputs['monitor_setting.channel_failure_rate_disable_enabled'] && (
              <Row gutter={16}>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    label={t('失败率统计窗口')}
                    step={1}
                    min={1}
                    suffix={t('分钟')}
                    field={
                      'monitor_setting.channel_failure_rate_window_minutes'
                    }
                    onChange={(value) =>
                      setInputs({
                        ...inputs,
                        'monitor_setting.channel_failure_rate_window_minutes':
                          String(value),
                      })
                    }
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    label={t('失败率阈值')}
                    step={1}
                    min={1}
                    max={100}
                    suffix={'%'}
                    field={'monitor_setting.channel_failure_rate_threshold'}
                    onChange={(value) =>
                      setInputs({
                        ...inputs,
                        'monitor_setting.channel_failure_rate_threshold':
                          String(value),
                      })
                    }
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    label={t('最小请求数')}
                    step={1}
                    min={1}
                    field={'monitor_setting.channel_failure_rate_min_requests'}
                    onChange={(value) =>
                      setInputs({
                        ...inputs,
                        'monitor_setting.channel_failure_rate_min_requests':
                          String(value),
                      })
                    }
                  />
                </Col>
              </Row>
            )}
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'monitor_setting.request_failure_webhook_enabled'}
                  label={t('请求失败 Webhook')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'monitor_setting.request_failure_webhook_enabled': value,
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'monitor_setting.channel_disabled_webhook_enabled'}
                  label={t('渠道禁用 Webhook')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'monitor_setting.channel_disabled_webhook_enabled': value,
                    })
                  }
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12}>
                <Form.Input
                  field={'monitor_setting.request_failure_webhook_url'}
                  label={t('请求失败 Webhook 地址')}
                  placeholder={t(
                    '请输入Webhook地址，例如: https://example.com/webhook',
                  )}
                  extraText={t('每次上游请求失败都会异步发送通知')}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'monitor_setting.request_failure_webhook_url': value,
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12}>
                <Form.Input
                  field={'monitor_setting.request_failure_webhook_secret'}
                  label={t('请求失败 Webhook 密钥')}
                  placeholder={t('留空表示不修改')}
                  mode='password'
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'monitor_setting.request_failure_webhook_secret': value,
                    })
                  }
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12}>
                <Form.Input
                  field={'monitor_setting.channel_disabled_webhook_url'}
                  label={t('渠道禁用 Webhook 地址')}
                  placeholder={t(
                    '请输入Webhook地址，例如: https://example.com/webhook',
                  )}
                  extraText={t('渠道被自动禁用后会异步发送通知')}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'monitor_setting.channel_disabled_webhook_url': value,
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12}>
                <Form.Input
                  field={'monitor_setting.channel_disabled_webhook_secret'}
                  label={t('渠道禁用 Webhook 密钥')}
                  placeholder={t('留空表示不修改')}
                  mode='password'
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'monitor_setting.channel_disabled_webhook_secret': value,
                    })
                  }
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={16}>
                <HttpStatusCodeRulesInput
                  label={t('自动禁用状态码')}
                  placeholder={t('例如：401, 403, 429, 500-599')}
                  extraText={t(
                    '支持填写单个状态码或范围（含首尾），使用逗号分隔',
                  )}
                  field={'AutomaticDisableStatusCodes'}
                  onChange={(value) =>
                    setInputs({ ...inputs, AutomaticDisableStatusCodes: value })
                  }
                  parsed={parsedAutoDisableStatusCodes}
                  invalidText={t('自动禁用状态码格式不正确')}
                />
                <HttpStatusCodeRulesInput
                  label={t('自动重试状态码')}
                  placeholder={t('例如：401, 403, 429, 500-599')}
                  extraText={t(
                    '支持填写单个状态码或范围（含首尾），使用逗号分隔；504 和 524 始终不重试，不受此处配置影响',
                  )}
                  field={'AutomaticRetryStatusCodes'}
                  onChange={(value) =>
                    setInputs({ ...inputs, AutomaticRetryStatusCodes: value })
                  }
                  parsed={parsedAutoRetryStatusCodes}
                  invalidText={t('自动重试状态码格式不正确')}
                />
                <Form.TextArea
                  label={t('自动禁用关键词')}
                  placeholder={t('一行一个，不区分大小写')}
                  extraText={t(
                    '当上游通道返回错误中包含这些关键词时（不区分大小写），自动禁用通道',
                  )}
                  field={'AutomaticDisableKeywords'}
                  autosize={{ minRows: 6, maxRows: 12 }}
                  onChange={(value) =>
                    setInputs({ ...inputs, AutomaticDisableKeywords: value })
                  }
                />
                <AutoDisablePolicyGroupsEditor
                  value={inputs.AutomaticDisablePolicyGroups}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      AutomaticDisablePolicyGroups: value,
                    })
                  }
                />
              </Col>
            </Row>
            <Row>
              <Button size='default' onClick={onSubmit}>
                {t('保存监控设置')}
              </Button>
            </Row>
          </Form.Section>
        </Form>
      </Spin>
    </>
  );
}
