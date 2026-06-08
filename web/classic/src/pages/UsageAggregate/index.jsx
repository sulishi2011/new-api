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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  DatePicker,
  Empty,
  Input,
  Select,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Download,
  RefreshCw,
  RotateCcw,
  Search,
  TableProperties,
} from 'lucide-react';
import dayjs from 'dayjs';
import utc from 'dayjs/plugin/utc';
import CardPro from '../../components/common/ui/CardPro';
import CardTable from '../../components/common/ui/CardTable';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import {
  API,
  createCardProPagination,
  renderNumber,
  renderQuota,
  showError,
  showSuccess,
} from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';

dayjs.extend(utc);

const { Text } = Typography;

const toPickerDate = (value) =>
  new Date(
    value.year(),
    value.month(),
    value.date(),
    value.hour(),
    value.minute(),
    value.second(),
  );

const getDefaultDayRange = () => {
  const end = dayjs().add(1, 'day').startOf('day');
  return [toPickerDate(end.subtract(30, 'day')), toPickerDate(end)];
};

const getDefaultHourRange = () => {
  const end = dayjs().startOf('hour');
  return [toPickerDate(end.subtract(24, 'hour')), toPickerDate(end)];
};

const createDefaultFilters = () => ({
  granularity: 'day',
  groupBy: 'channel',
  dateRange: getDefaultDayRange(),
  channel_id: '',
  provider_key_id: '',
  token_id: '',
  requested_model: '',
  actual_model: '',
});

const normalizePickerDate = (value) => {
  if (!value) return null;
  if (value instanceof Date) return value;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
};

const pickerDateToTimestamp = (value) => {
  const date = normalizePickerDate(value);
  if (!date) return 0;
  return Math.floor(date.getTime() / 1000);
};

const getLocalTimezoneOffsetSeconds = () =>
  -new Date().getTimezoneOffset() * 60;

const formatTimezoneOffsetLabel = (offsetSeconds) => {
  if (!offsetSeconds) return 'UTC';

  const sign = offsetSeconds >= 0 ? '+' : '-';
  const absolute = Math.abs(offsetSeconds);
  const hours = Math.floor(absolute / 3600);
  const minutes = Math.floor((absolute % 3600) / 60);

  return `UTC${sign}${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}`;
};

const formatBucket = (timestamp, granularity, timezoneOffsetSeconds) => {
  if (!timestamp) return '-';
  const start = dayjs.unix(timestamp + timezoneOffsetSeconds).utc();
  if (!start.isValid()) return '-';
  const end = start.add(1, granularity === 'hour' ? 'hour' : 'day');
  const endText = start.isSame(end, 'day')
    ? end.format('HH:mm')
    : end.format('YYYY-MM-DD HH:mm');
  return `${start.format('YYYY-MM-DD HH:mm')} - ${endText} ${formatTimezoneOffsetLabel(timezoneOffsetSeconds)}`;
};

const parseNumericFilter = (value) => {
  const trimmed = String(value || '').trim();
  if (!trimmed) return '';
  const parsed = parseInt(trimmed, 10);
  return Number.isNaN(parsed) ? '' : String(parsed);
};

const formatVendorChannelName = (record) => {
  const code = String(record?.vendor_profile_code || '').trim();
  const name = String(record?.channel_name || '').trim();
  if (code && name) return `${code} - ${name}`;
  return code || name || '-';
};

const getCsvFilename = (headers, fallback) => {
  const disposition = headers?.['content-disposition'] || '';
  const match = disposition.match(/filename="?([^"]+)"?/i);
  return match?.[1] || fallback;
};

const UsageAggregate = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const timezoneOffsetSeconds = getLocalTimezoneOffsetSeconds();

  const [filters, setFilters] = useState(createDefaultFilters);
  const [appliedFilters, setAppliedFilters] = useState(createDefaultFilters);
  const [items, setItems] = useState([]);
  const [summary, setSummary] = useState({});
  const [total, setTotal] = useState(0);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(
    parseInt(localStorage.getItem('page-size')) || ITEMS_PER_PAGE,
  );
  const [loading, setLoading] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [sortBy, setSortBy] = useState('bucket_start');
  const [sortOrder, setSortOrder] = useState('desc');

  const granularityOptions = useMemo(
    () => [
      { label: t('按天'), value: 'day' },
      { label: t('按小时'), value: 'hour' },
    ],
    [t],
  );

  const groupByOptions = useMemo(
    () => [
      { label: t('渠道'), value: 'channel' },
      { label: t('详情'), value: 'detail' },
    ],
    [t],
  );

  const dateRangePresets = useMemo(() => {
    if (filters.granularity === 'hour') {
      const end = dayjs().startOf('hour');
      return [
        {
          text: t('近 24 小时'),
          start: toPickerDate(end.subtract(24, 'hour')),
          end: toPickerDate(end),
        },
        {
          text: t('近 72 小时'),
          start: toPickerDate(end.subtract(72, 'hour')),
          end: toPickerDate(end),
        },
      ];
    }
    const end = dayjs().add(1, 'day').startOf('day');
    return [
      {
        text: t('近 7 天'),
        start: toPickerDate(end.subtract(7, 'day')),
        end: toPickerDate(end),
      },
      {
        text: t('近 30 天'),
        start: toPickerDate(end.subtract(30, 'day')),
        end: toPickerDate(end),
      },
    ];
  }, [filters.granularity, t]);

  const updateFilter = (key, value) => {
    setFilters((prev) => ({ ...prev, [key]: value ?? '' }));
  };

  const handleGranularityChange = (value) => {
    setFilters((prev) => ({
      ...prev,
      granularity: value,
      dateRange:
        value === 'hour' ? getDefaultHourRange() : getDefaultDayRange(),
    }));
  };

  const buildParams = (targetPage, targetPageSize, exportMode = false) => {
    const params = new URLSearchParams();
    const range = appliedFilters.dateRange || [];
    params.set('granularity', appliedFilters.granularity);
    params.set('group_by', appliedFilters.groupBy);
    params.set('timezone_offset', String(timezoneOffsetSeconds));
    params.set('start_timestamp', String(pickerDateToTimestamp(range[0])));
    params.set('end_timestamp', String(pickerDateToTimestamp(range[1])));
    params.set('sort_by', sortBy);
    params.set('sort_order', sortOrder);
    if (!exportMode) {
      params.set('p', String(targetPage));
      params.set('page_size', String(targetPageSize));
    }

    [
      'channel_id',
      'provider_key_id',
      'token_id',
      'requested_model',
      'actual_model',
    ].forEach((key) => {
      const value = String(appliedFilters[key] || '').trim();
      if (value) {
        params.set(key, value);
      }
    });
    return params;
  };

  const loadAggregates = async (
    targetPage = activePage,
    targetSize = pageSize,
  ) => {
    setLoading(true);
    try {
      const params = buildParams(targetPage, targetSize);
      const res = await API.get(`/api/usage_aggregates/?${params.toString()}`);
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const page = data?.page || {};
      const nextItems = page.items || [];
      setItems(
        nextItems.map((item, index) => ({
          ...item,
          key: [
            appliedFilters.groupBy,
            item.bucket_start,
            item.channel_id,
            item.provider_key_id,
            item.token_id,
            item.requested_model,
            item.actual_model,
            index,
          ].join('-'),
        })),
      );
      setSummary(data?.summary || {});
      setTotal(page.total || 0);
      setActivePage(page.page || targetPage);
      setPageSize(page.page_size || targetSize);
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  const handleSearch = () => {
    setActivePage(1);
    setAppliedFilters({
      ...filters,
      channel_id: parseNumericFilter(filters.channel_id),
      provider_key_id: parseNumericFilter(filters.provider_key_id),
      token_id: parseNumericFilter(filters.token_id),
    });
  };

  const handleReset = () => {
    const defaults = createDefaultFilters();
    setFilters(defaults);
    setAppliedFilters(defaults);
    setSortBy('bucket_start');
    setSortOrder('desc');
    setActivePage(1);
  };

  const handlePageChange = (page) => {
    setActivePage(page);
  };

  const handlePageSizeChange = (size) => {
    localStorage.setItem('page-size', String(size));
    setPageSize(size);
    setActivePage(1);
  };

  const handleSort = (nextSortBy) => {
    setActivePage(1);
    if (sortBy !== nextSortBy) {
      setSortBy(nextSortBy);
      setSortOrder('desc');
      return;
    }
    setSortOrder((prev) => (prev === 'desc' ? 'asc' : 'desc'));
  };

  const exportCsv = async () => {
    setExporting(true);
    try {
      const params = buildParams(activePage, pageSize, true);
      const res = await API.get(
        `/api/usage_aggregates/export?${params.toString()}`,
        { responseType: 'blob' },
      );
      const contentType = res.headers?.['content-type'] || '';
      if (contentType.includes('application/json')) {
        const text = await res.data.text();
        try {
          const payload = JSON.parse(text);
          showError(payload.message || t('导出失败'));
        } catch (error) {
          showError(t('导出失败'));
        }
        return;
      }
      const filename = getCsvFilename(
        res.headers,
        `usage-aggregates-${appliedFilters.granularity}.csv`,
      );
      const blob = new Blob([res.data], { type: 'text/csv;charset=utf-8' });
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
      showSuccess(t('导出成功'));
    } catch (error) {
      showError(error.message || t('导出失败'));
    } finally {
      setExporting(false);
    }
  };

  useEffect(() => {
    loadAggregates(activePage, pageSize);
  }, [activePage, pageSize, appliedFilters, sortBy, sortOrder]);

  const metricCells = [
    { label: t('请求数'), value: renderNumber(summary.request_count || 0) },
    { label: t('原价消耗'), value: renderQuota(summary.quota || 0, 4) },
    { label: t('成本消耗'), value: renderQuota(summary.cost_quota || 0, 4) },
    { label: t('输入 Tokens'), value: renderNumber(summary.input_tokens || 0) },
    {
      label: t('输出 Tokens'),
      value: renderNumber(summary.output_tokens || 0),
    },
    {
      label: t('缓存读 Tokens'),
      value: renderNumber(summary.cache_read_tokens || 0),
    },
    {
      label: t('缓存写 Tokens'),
      value: renderNumber(summary.cache_write_tokens || 0),
    },
    { label: t('总 Tokens'), value: renderNumber(summary.total_tokens || 0) },
  ];

  const sortableTitle = (label, key) => {
    const marker = sortBy === key ? (sortOrder === 'desc' ? '↓' : '↑') : '';
    return (
      <button
        type='button'
        className='font-medium'
        style={{
          color: 'inherit',
          background: 'transparent',
          border: 0,
          padding: 0,
          cursor: 'pointer',
        }}
        onClick={() => handleSort(key)}
      >
        {label} {marker}
      </button>
    );
  };

  const primaryColumns = [
    {
      title: sortableTitle(t('时间'), 'bucket_start'),
      dataIndex: 'bucket_start',
      key: 'bucket_start',
      width: 260,
      fixed: 'left',
      render: (value) =>
        formatBucket(value, appliedFilters.granularity, timezoneOffsetSeconds),
    },
    {
      title: t('渠道'),
      dataIndex: 'channel_id',
      key: 'channel_id',
      width: 180,
      render: (_, record) => (
        <Space wrap>
          <Tag color='blue' shape='circle'>
            #{record.channel_id || 0}
          </Tag>
          <Text>{formatVendorChannelName(record)}</Text>
        </Space>
      ),
    },
  ];

  const detailColumns =
    appliedFilters.groupBy === 'detail'
      ? [
          {
            title: t('上游 Key'),
            dataIndex: 'provider_key_id',
            key: 'provider_key_id',
            width: 190,
            render: (_, record) =>
              record.provider_key_id ? (
                <div style={{ lineHeight: 1.5 }}>
                  <Text>#{record.provider_key_id}</Text>
                  <div className='text-xs text-[var(--semi-color-text-2)]'>
                    {record.provider_key_preview || '-'}
                  </div>
                </div>
              ) : (
                '-'
              ),
          },
          {
            title: t('令牌'),
            dataIndex: 'token_id',
            key: 'token_id',
            width: 170,
            render: (_, record) => (
              <div style={{ lineHeight: 1.5 }}>
                <Text>#{record.token_id || 0}</Text>
                <div className='text-xs text-[var(--semi-color-text-2)]'>
                  {record.token_name || '-'}
                </div>
              </div>
            ),
          },
          {
            title: t('模型'),
            dataIndex: 'requested_model',
            key: 'model',
            width: 220,
            render: (_, record) => (
              <div style={{ lineHeight: 1.5, wordBreak: 'break-all' }}>
                <Text>{record.requested_model || '-'}</Text>
                {record.actual_model &&
                record.actual_model !== record.requested_model ? (
                  <div className='text-xs text-[var(--semi-color-text-2)]'>
                    {t('实际')}: {record.actual_model}
                  </div>
                ) : null}
              </div>
            ),
          },
        ]
      : [];

  const metricColumns = [
    {
      title: sortableTitle(t('请求数'), 'request_count'),
      dataIndex: 'request_count',
      key: 'request_count',
      width: 100,
      render: (value) => renderNumber(value || 0),
    },
    {
      title: sortableTitle(t('原价消耗'), 'quota'),
      dataIndex: 'quota',
      key: 'quota',
      width: 120,
      render: (value) => renderQuota(value || 0, 4),
    },
    {
      title: sortableTitle(t('成本消耗'), 'cost_quota'),
      dataIndex: 'cost_quota',
      key: 'cost_quota',
      width: 120,
      render: (value) => renderQuota(value || 0, 4),
    },
    {
      title: sortableTitle(t('输入'), 'input_tokens'),
      dataIndex: 'input_tokens',
      key: 'input_tokens',
      width: 100,
      render: (value) => renderNumber(value || 0),
    },
    {
      title: sortableTitle(t('输出'), 'output_tokens'),
      dataIndex: 'output_tokens',
      key: 'output_tokens',
      width: 100,
      render: (value) => renderNumber(value || 0),
    },
    {
      title: sortableTitle(t('缓存读'), 'cache_read_tokens'),
      dataIndex: 'cache_read_tokens',
      key: 'cache_read_tokens',
      width: 110,
      render: (value) => renderNumber(value || 0),
    },
    {
      title: sortableTitle(t('缓存写'), 'cache_write_tokens'),
      dataIndex: 'cache_write_tokens',
      key: 'cache_write_tokens',
      width: 110,
      render: (value) => renderNumber(value || 0),
    },
    {
      title: sortableTitle(t('总 Tokens'), 'total_tokens'),
      dataIndex: 'total_tokens',
      key: 'total_tokens',
      width: 110,
      render: (value) => renderNumber(value || 0),
    },
  ];

  const columns = [...primaryColumns, ...detailColumns, ...metricColumns];

  return (
    <div className='mt-[60px] px-2'>
      <CardPro
        type='type2'
        statsArea={
          <div className='flex flex-col gap-3'>
            <div className='flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between'>
              <div className='flex items-center gap-2'>
                <TableProperties size={18} />
                <Text strong>{t('用量汇总')}</Text>
                <Tag color='grey' shape='circle'>
                  {formatTimezoneOffsetLabel(timezoneOffsetSeconds)}
                </Tag>
              </div>
              <Space wrap>
                <Button
                  size='small'
                  type='tertiary'
                  icon={<RefreshCw size={14} />}
                  loading={loading}
                  onClick={() => loadAggregates(activePage, pageSize)}
                >
                  {t('刷新')}
                </Button>
                <Button
                  size='small'
                  type='tertiary'
                  icon={<Download size={14} />}
                  loading={exporting}
                  onClick={exportCsv}
                >
                  {t('导出 CSV')}
                </Button>
              </Space>
            </div>
            <div className='grid grid-cols-2 gap-x-4 gap-y-2 md:grid-cols-4 xl:grid-cols-8'>
              {metricCells.map((item) => (
                <div key={item.label} className='min-w-0'>
                  <div className='truncate text-xs text-[var(--semi-color-text-2)]'>
                    {item.label}
                  </div>
                  <div className='truncate text-sm font-semibold'>
                    {item.value}
                  </div>
                </div>
              ))}
            </div>
          </div>
        }
        searchArea={
          <div className='flex flex-col gap-2'>
            <div className='grid grid-cols-1 gap-2 md:grid-cols-2 xl:grid-cols-5'>
              <Select
                value={filters.granularity}
                optionList={granularityOptions}
                placeholder={t('时间粒度')}
                pure
                size='small'
                onChange={handleGranularityChange}
              />
              <Select
                value={filters.groupBy}
                optionList={groupByOptions}
                placeholder={t('模式')}
                pure
                size='small'
                onChange={(value) => updateFilter('groupBy', value)}
              />
              <div className='md:col-span-1 xl:col-span-2'>
                <DatePicker
                  value={filters.dateRange}
                  type='dateTimeRange'
                  placeholder={[t('开始时间'), t('结束时间')]}
                  showClear
                  pure
                  size='small'
                  className='w-full'
                  presets={dateRangePresets}
                  onChange={(value) => updateFilter('dateRange', value)}
                />
              </div>
              <Input
                value={filters.channel_id}
                placeholder={t('渠道 ID')}
                showClear
                pure
                size='small'
                onChange={(value) => updateFilter('channel_id', value)}
              />
              <Input
                value={filters.provider_key_id}
                placeholder={t('上游 Key ID')}
                showClear
                pure
                size='small'
                onChange={(value) => updateFilter('provider_key_id', value)}
              />
              <Input
                value={filters.token_id}
                placeholder={t('令牌 ID')}
                showClear
                pure
                size='small'
                onChange={(value) => updateFilter('token_id', value)}
              />
              <Input
                value={filters.requested_model}
                placeholder={t('请求模型')}
                showClear
                pure
                size='small'
                onChange={(value) => updateFilter('requested_model', value)}
              />
              <Input
                value={filters.actual_model}
                placeholder={t('实际模型')}
                showClear
                pure
                size='small'
                onChange={(value) => updateFilter('actual_model', value)}
              />
            </div>
            <div className='flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
              <Text type='tertiary' size='small'>
                {t('本地')} {formatTimezoneOffsetLabel(timezoneOffsetSeconds)}
              </Text>
              <Space wrap>
                <Button
                  size='small'
                  type='primary'
                  icon={<Search size={14} />}
                  loading={loading}
                  onClick={handleSearch}
                >
                  {t('查询')}
                </Button>
                <Button
                  size='small'
                  type='tertiary'
                  icon={<RotateCcw size={14} />}
                  onClick={handleReset}
                >
                  {t('重置')}
                </Button>
              </Space>
            </div>
          </div>
        }
        paginationArea={createCardProPagination({
          currentPage: activePage,
          pageSize,
          total,
          onPageChange: handlePageChange,
          onPageSizeChange: handlePageSizeChange,
          isMobile,
          t,
        })}
        t={t}
      >
        <CardTable
          rowKey='key'
          hidePagination
          loading={loading}
          columns={columns}
          dataSource={items}
          scroll={{ x: 'max-content' }}
          size='small'
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 150, height: 150 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('搜索无结果')}
              style={{ padding: 30 }}
            />
          }
        />
      </CardPro>
    </div>
  );
};

export default UsageAggregate;
