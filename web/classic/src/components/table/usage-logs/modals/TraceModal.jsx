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
import { Collapse, Modal, Spin, Tag } from '@douyinfe/semi-ui';
import { API, showError } from '../../../../helpers';
import CodeViewer from '../../../playground/CodeViewer';

const getTraceBodyText = (part, t) => {
  if (!part) {
    return '';
  }
  if (part.body) {
    return part.body;
  }
  switch (part.storage_kind) {
    case 'omitted_multipart':
      return t('multipart 内容未内联展示');
    case 'omitted_binary':
      return t('二进制内容未内联展示');
    case 'empty':
      return t('空内容');
    default:
      return '';
  }
};

const getTraceLanguage = (part) => {
  const contentType = part?.content_type || '';
  const body = part?.body || '';
  if (
    contentType.includes('json') ||
    body.trim().startsWith('{') ||
    body.trim().startsWith('[')
  ) {
    return 'json';
  }
  return 'text';
};

const getTraceHeadersText = (part) => {
  if (!part?.headers || Object.keys(part.headers).length === 0) {
    return '';
  }
  try {
    return JSON.stringify(part.headers, null, 2);
  } catch (e) {
    return '';
  }
};

const TracePartViewer = ({ part, title, labels, t }) => {
  if (!part) {
    return null;
  }
  const headersText = getTraceHeadersText(part);
  const bodyText = getTraceBodyText(part, t);
  const meta = [
    part?.content_type || '',
    part?.body_size >= 0 ? `${t('大小')} ${part.body_size} B` : '',
    part?.truncated ? t('已截断') : '',
  ]
    .filter(Boolean)
    .join(' · ');

  return (
    <div style={{ minWidth: 0, width: '100%', maxWidth: 960 }}>
      {meta ? (
        <div
          style={{
            marginBottom: 8,
            color: 'var(--semi-color-text-2)',
            fontSize: 12,
          }}
        >
          {meta}
        </div>
      ) : null}
      {headersText ? (
        <Collapse keepDOM style={{ marginBottom: 12 }}>
          <Collapse.Panel header={labels.headers} itemKey='headers'>
            <CodeViewer content={headersText} title='headers' language='json' />
          </Collapse.Panel>
        </Collapse>
      ) : null}
      <div
        style={{
          marginTop: headersText ? 12 : 0,
          marginBottom: 8,
          color: 'var(--semi-color-text-1)',
          fontSize: 12,
          fontWeight: 600,
        }}
      >
        {labels.body}
      </div>
      <CodeViewer
        content={bodyText}
        title={title}
        language={getTraceLanguage(part)}
      />
    </div>
  );
};

const TraceModal = ({ showTraceModal, setShowTraceModal, traceTarget, t }) => {
  const [traceCache, setTraceCache] = useState({});
  const [loading, setLoading] = useState(false);

  const logId = traceTarget?.logId;
  const inlinePayload = traceTarget?.inlineTrace
    ? { trace: traceTarget.inlineTrace }
    : null;
  const payload = inlinePayload || (logId ? traceCache[logId] : null);
  const trace = payload?.trace;

  useEffect(() => {
    if (!showTraceModal || !logId || inlinePayload || traceCache[logId]) {
      return;
    }

    let cancelled = false;
    const loadTrace = async () => {
      setLoading(true);
      try {
        const res = await API.get(`/api/log/${logId}/trace`);
        const { success, message, data } = res.data;
        if (!cancelled) {
          if (success) {
            setTraceCache((prev) => ({ ...prev, [logId]: data }));
          } else {
            showError(message || t('Trace 加载失败'));
          }
        }
      } catch (error) {
        if (!cancelled) {
          showError(error?.message || t('Trace 加载失败'));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    loadTrace();
    return () => {
      cancelled = true;
    };
  }, [showTraceModal, logId, inlinePayload, traceCache, t]);

  return (
    <Modal
      title={t('请求 Trace')}
      visible={showTraceModal}
      onCancel={() => setShowTraceModal(false)}
      footer={null}
      width={1000}
    >
      <Spin spinning={loading}>
        {trace ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              {payload?.request_id ? (
                <Tag color='blue'>
                  {t('Request ID')}: {payload.request_id}
                </Tag>
              ) : null}
              {trace?.upstream_request_id || payload?.upstream_request_id ? (
                <Tag color='green'>
                  {t('上游请求 ID')}:{' '}
                  {trace?.upstream_request_id || payload?.upstream_request_id}
                </Tag>
              ) : null}
              {trace?.status_code ? (
                <Tag color='orange'>
                  {t('上游状态码')}: {trace.status_code}
                </Tag>
              ) : null}
            </div>
            <TracePartViewer
              part={trace?.request}
              title='request'
              labels={{ headers: t('请求头'), body: t('请求体') }}
              t={t}
            />
            <TracePartViewer
              part={trace?.response}
              title='response'
              labels={{ headers: t('响应头'), body: t('响应体') }}
              t={t}
            />
          </div>
        ) : (
          <div style={{ color: 'var(--semi-color-text-2)' }}>
            {loading ? t('加载中...') : t('暂无 Trace 数据')}
          </div>
        )}
      </Spin>
    </Modal>
  );
};

export default TraceModal;
