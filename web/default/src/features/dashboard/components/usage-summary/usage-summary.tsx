/*
Copyright (C) 2023-2026 QuantumNous

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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  type ColumnDef,
  type PaginationState,
  type SortingState,
  type VisibilityState,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { Download, RefreshCcw, RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import { formatNumber, formatQuota } from '@/lib/format'
import { dateToUnixTimestamp, formatChartTime } from '@/lib/time'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { DataTableColumnHeader, DataTablePage } from '@/components/data-table'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import { exportUsageAggregates, getUsageAggregates } from '../../api'
import type {
  UsageAggregateGranularity,
  UsageAggregateQueryParams,
  UsageAggregateRow,
  UsageAggregateSummary,
} from '../../types'

type UsageSource = 'live' | 'aggregate'

type UsageSummaryFilters = {
  granularity: UsageAggregateGranularity
  source: UsageSource
  range: { start?: Date; end?: Date }
  channelId: string
  providerKeyId: string
  tokenId: string
  requestedModel: string
  actualModel: string
}

const DEFAULT_SUMMARY: UsageAggregateSummary = {
  request_count: 0,
  quota: 0,
  cost_quota: 0,
  input_tokens: 0,
  output_tokens: 0,
  cache_read_tokens: 0,
  cache_write_tokens: 0,
  total_tokens: 0,
}

function getDefaultFilters(): UsageSummaryFilters {
  const now = dayjs()
  return {
    granularity: 'day',
    source: 'live',
    range: {
      start: now.subtract(6, 'day').startOf('day').toDate(),
      end: now.add(1, 'hour').toDate(),
    },
    channelId: '',
    providerKeyId: '',
    tokenId: '',
    requestedModel: '',
    actualModel: '',
  }
}

function numericFilter(value: string): number | undefined {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
}

function textFilter(value: string): string | undefined {
  const trimmed = value.trim()
  return trimmed ? trimmed : undefined
}

function getSortParams(sorting: SortingState): {
  sort_by: string
  sort_order: 'asc' | 'desc'
} {
  const first = sorting[0]
  return {
    sort_by: first?.id || 'bucket_start',
    sort_order: first?.desc === false ? 'asc' : 'desc',
  }
}

function buildUsageSummaryParams(
  filters: UsageSummaryFilters,
  pagination?: PaginationState,
  sorting: SortingState = []
): UsageAggregateQueryParams {
  const sort = getSortParams(sorting)
  return {
    p: pagination ? pagination.pageIndex + 1 : undefined,
    page_size: pagination?.pageSize,
    granularity: filters.granularity,
    live: filters.source === 'live',
    start_timestamp: filters.range.start
      ? dateToUnixTimestamp(filters.range.start)
      : undefined,
    end_timestamp: filters.range.end
      ? dateToUnixTimestamp(filters.range.end) + 1
      : undefined,
    channel_id: numericFilter(filters.channelId),
    provider_key_id: numericFilter(filters.providerKeyId),
    token_id: numericFilter(filters.tokenId),
    requested_model: textFilter(filters.requestedModel),
    actual_model: textFilter(filters.actualModel),
    ...sort,
  }
}

function MetricTile(props: {
  label: string
  value: string
  accent: string
  isLoading?: boolean
}) {
  return (
    <div className='rounded-lg border px-4 py-3'>
      <div className='flex items-center gap-2'>
        <span className={cn('h-3.5 w-0.5 rounded-full', props.accent)} />
        <span className='text-muted-foreground text-xs'>{props.label}</span>
      </div>
      {props.isLoading ? (
        <Skeleton className='mt-2 h-7 w-24' />
      ) : (
        <div className='mt-2 truncate font-mono text-xl font-semibold tabular-nums'>
          {props.value}
        </div>
      )}
    </div>
  )
}

function ModelText({ value }: { value?: string }) {
  if (!value) {
    return <span className='text-muted-foreground'>-</span>
  }
  return <span className='font-mono text-xs'>{value}</span>
}

function EntityLabel(props: {
  id: number
  name?: string
  preview?: string
  emptyLabel?: string
}) {
  if (!props.id) {
    return (
      <span className='text-muted-foreground'>{props.emptyLabel || '-'}</span>
    )
  }
  return (
    <div className='min-w-0'>
      <div className='font-mono text-xs'>#{props.id}</div>
      {(props.name || props.preview) && (
        <div className='text-muted-foreground max-w-40 truncate text-xs'>
          {props.name || props.preview}
        </div>
      )}
    </div>
  )
}

function useUsageSummaryColumns(
  granularity: UsageAggregateGranularity
): ColumnDef<UsageAggregateRow>[] {
  const { t } = useTranslation()

  return useMemo(
    () => [
      {
        accessorKey: 'bucket_start',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Bucket')} />
        ),
        cell: ({ row }) => (
          <span className='font-mono text-xs'>
            {formatChartTime(row.original.bucket_start, granularity)}
          </span>
        ),
        enableSorting: true,
        meta: { label: t('Bucket'), mobileTitle: true },
      },
      {
        accessorKey: 'requested_model',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Requested model')} />
        ),
        cell: ({ row }) => <ModelText value={row.original.requested_model} />,
        meta: { label: t('Requested model') },
      },
      {
        accessorKey: 'actual_model',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Actual model')} />
        ),
        cell: ({ row }) => <ModelText value={row.original.actual_model} />,
        meta: { label: t('Actual model') },
      },
      {
        accessorKey: 'channel_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Channel')} />
        ),
        cell: ({ row }) => (
          <EntityLabel
            id={row.original.channel_id}
            name={row.original.channel_name}
          />
        ),
        meta: { label: t('Channel') },
      },
      {
        accessorKey: 'provider_key_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Provider key')} />
        ),
        cell: ({ row }) => (
          <EntityLabel
            id={row.original.provider_key_id}
            preview={row.original.provider_key_preview}
          />
        ),
        meta: { label: t('Provider key') },
      },
      {
        accessorKey: 'token_id',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Token')} />
        ),
        cell: ({ row }) => (
          <EntityLabel
            id={row.original.token_id}
            name={row.original.token_name}
          />
        ),
        meta: { label: t('Token') },
      },
      {
        accessorKey: 'request_count',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Requests')} />
        ),
        cell: ({ row }) => (
          <span className='font-mono tabular-nums'>
            {formatNumber(row.original.request_count)}
          </span>
        ),
        enableSorting: true,
        meta: { label: t('Requests'), mobileBadge: true },
      },
      {
        accessorKey: 'quota',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Usage')} />
        ),
        cell: ({ row }) => (
          <span className='font-mono tabular-nums'>
            {formatQuota(row.original.quota)}
          </span>
        ),
        enableSorting: true,
        meta: { label: t('Usage') },
      },
      {
        accessorKey: 'cost_quota',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Cost')} />
        ),
        cell: ({ row }) => (
          <span className='font-mono tabular-nums'>
            {formatQuota(row.original.cost_quota)}
          </span>
        ),
        enableSorting: true,
        meta: { label: t('Cost') },
      },
      {
        accessorKey: 'input_tokens',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Input tokens')} />
        ),
        cell: ({ row }) => (
          <span className='font-mono tabular-nums'>
            {formatNumber(row.original.input_tokens)}
          </span>
        ),
        enableSorting: true,
        meta: { label: t('Input tokens') },
      },
      {
        accessorKey: 'output_tokens',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Output tokens')} />
        ),
        cell: ({ row }) => (
          <span className='font-mono tabular-nums'>
            {formatNumber(row.original.output_tokens)}
          </span>
        ),
        enableSorting: true,
        meta: { label: t('Output tokens') },
      },
      {
        accessorKey: 'cache_read_tokens',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Cache read')} />
        ),
        cell: ({ row }) => (
          <span className='font-mono tabular-nums'>
            {formatNumber(row.original.cache_read_tokens)}
          </span>
        ),
        enableSorting: true,
        meta: { label: t('Cache read') },
      },
      {
        accessorKey: 'cache_write_tokens',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Cache write')} />
        ),
        cell: ({ row }) => (
          <span className='font-mono tabular-nums'>
            {formatNumber(row.original.cache_write_tokens)}
          </span>
        ),
        enableSorting: true,
        meta: { label: t('Cache write') },
      },
      {
        accessorKey: 'total_tokens',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Total tokens')} />
        ),
        cell: ({ row }) => (
          <span className='font-mono tabular-nums'>
            {formatNumber(row.original.total_tokens)}
          </span>
        ),
        enableSorting: true,
        meta: { label: t('Total tokens') },
      },
    ],
    [granularity, t]
  )
}

function buildExportFilename() {
  return `usage-summary-${dayjs().format('YYYYMMDDHHmmss')}.csv`
}

export function UsageSummary() {
  const { t } = useTranslation()
  const [filters, setFilters] = useState<UsageSummaryFilters>(() =>
    getDefaultFilters()
  )
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: 20,
  })
  const [sorting, setSorting] = useState<SortingState>([
    { id: 'bucket_start', desc: true },
  ])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    provider_key_id: false,
    token_id: false,
    cache_read_tokens: false,
    cache_write_tokens: false,
  })
  const [isExporting, setIsExporting] = useState(false)
  const columns = useUsageSummaryColumns(filters.granularity)

  const params = useMemo(
    () => buildUsageSummaryParams(filters, pagination, sorting),
    [filters, pagination, sorting]
  )

  const query = useQuery({
    queryKey: ['usage-aggregates', params],
    queryFn: async () => {
      const result = await getUsageAggregates(params)
      if (!result.success) {
        throw new Error(result.message || 'Failed to fetch usage')
      }
      return result.data
    },
    placeholderData: (previousData) => previousData,
  })

  const rows = query.data?.page.items ?? []
  const summary = query.data?.summary ?? DEFAULT_SUMMARY
  const total = query.data?.page.total ?? 0

  const table = useReactTable({
    data: rows,
    columns,
    state: {
      pagination,
      sorting,
      columnVisibility,
    },
    manualPagination: true,
    manualSorting: true,
    pageCount: Math.ceil(total / pagination.pageSize),
    onPaginationChange: setPagination,
    onSortingChange: (updater) => {
      setSorting((current) =>
        typeof updater === 'function' ? updater(current) : updater
      )
      setPagination((current) => ({ ...current, pageIndex: 0 }))
    },
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: getCoreRowModel(),
  })

  const updateFilter = <K extends keyof UsageSummaryFilters>(
    key: K,
    value: UsageSummaryFilters[K]
  ) => {
    setFilters((current) => ({ ...current, [key]: value }))
    setPagination((current) => ({ ...current, pageIndex: 0 }))
  }

  const resetFilters = () => {
    setFilters(getDefaultFilters())
    setPagination((current) => ({ ...current, pageIndex: 0 }))
    setSorting([{ id: 'bucket_start', desc: true }])
  }

  const exportCsv = async () => {
    setIsExporting(true)
    try {
      const blob = await exportUsageAggregates(
        buildUsageSummaryParams(filters, undefined, sorting)
      )
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = buildExportFilename()
      anchor.click()
      URL.revokeObjectURL(url)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to export data')
      )
    } finally {
      setIsExporting(false)
    }
  }

  return (
    <div className='space-y-3 sm:space-y-4'>
      <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-4'>
        <MetricTile
          label={t('Requests')}
          value={formatNumber(summary.request_count)}
          accent='bg-rose-500/70'
          isLoading={query.isLoading}
        />
        <MetricTile
          label={t('Billable usage')}
          value={formatQuota(summary.quota)}
          accent='bg-sky-500/70'
          isLoading={query.isLoading}
        />
        <MetricTile
          label={t('Provider cost')}
          value={formatQuota(summary.cost_quota)}
          accent='bg-amber-500/75'
          isLoading={query.isLoading}
        />
        <MetricTile
          label={t('Total tokens')}
          value={formatNumber(summary.total_tokens)}
          accent='bg-emerald-500/70'
          isLoading={query.isLoading}
        />
      </div>

      <div className='rounded-lg border p-3'>
        <div className='grid gap-2 lg:grid-cols-[minmax(260px,1.5fr)_repeat(3,minmax(130px,0.75fr))]'>
          <CompactDateTimeRangePicker
            start={filters.range.start}
            end={filters.range.end}
            onChange={(range) => updateFilter('range', range)}
          />
          <Select
            items={[
              { value: 'day', label: t('Day') },
              { value: 'hour', label: t('Hour') },
            ]}
            value={filters.granularity}
            onValueChange={(value) =>
              updateFilter('granularity', value as UsageAggregateGranularity)
            }
          >
            <SelectTrigger className='w-full'>
              <SelectValue placeholder={t('Granularity')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='day'>{t('Day')}</SelectItem>
                <SelectItem value='hour'>{t('Hour')}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
          <Select
            items={[
              { value: 'live', label: t('Live ledger') },
              { value: 'aggregate', label: t('Aggregated table') },
            ]}
            value={filters.source}
            onValueChange={(value) =>
              updateFilter('source', value as UsageSource)
            }
          >
            <SelectTrigger className='w-full'>
              <SelectValue placeholder={t('Data source')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='live'>{t('Live ledger')}</SelectItem>
                <SelectItem value='aggregate'>
                  {t('Aggregated table')}
                </SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
          <div className='flex gap-2'>
            <Button
              type='button'
              variant='outline'
              className='min-w-0 flex-1 gap-2'
              onClick={() => query.refetch()}
            >
              <RefreshCcw className='size-4' />
              <span className='truncate'>{t('Refresh')}</span>
            </Button>
            <Button
              type='button'
              variant='outline'
              className='min-w-0 flex-1 gap-2'
              onClick={resetFilters}
            >
              <RotateCcw className='size-4' />
              <span className='truncate'>{t('Reset filters')}</span>
            </Button>
          </div>
        </div>

        <div className='mt-2 grid gap-2 sm:grid-cols-2 lg:grid-cols-5'>
          <Input
            value={filters.channelId}
            onChange={(event) => updateFilter('channelId', event.target.value)}
            placeholder={t('Channel ID')}
            inputMode='numeric'
          />
          <Input
            value={filters.providerKeyId}
            onChange={(event) =>
              updateFilter('providerKeyId', event.target.value)
            }
            placeholder={t('Provider key ID')}
            inputMode='numeric'
          />
          <Input
            value={filters.tokenId}
            onChange={(event) => updateFilter('tokenId', event.target.value)}
            placeholder={t('Token ID')}
            inputMode='numeric'
          />
          <Input
            value={filters.requestedModel}
            onChange={(event) =>
              updateFilter('requestedModel', event.target.value)
            }
            placeholder={t('Requested model')}
          />
          <Input
            value={filters.actualModel}
            onChange={(event) =>
              updateFilter('actualModel', event.target.value)
            }
            placeholder={t('Actual model')}
          />
        </div>

        <div className='mt-2 flex flex-wrap items-center justify-between gap-2'>
          <div className='flex flex-wrap gap-1.5'>
            <Badge variant='secondary'>
              {filters.source === 'live'
                ? t('Live ledger')
                : t('Aggregated table')}
            </Badge>
            <Badge variant='outline'>
              {filters.granularity === 'day' ? t('Day') : t('Hour')}
            </Badge>
          </div>
          <Button
            type='button'
            variant='outline'
            className='gap-2'
            disabled={isExporting}
            onClick={exportCsv}
          >
            <Download className='size-4' />
            {isExporting ? t('Exporting...') : t('Export CSV')}
          </Button>
        </div>
      </div>

      <DataTablePage
        table={table}
        columns={columns}
        isLoading={query.isLoading}
        isFetching={query.isFetching}
        emptyTitle={t('No usage summary found')}
        emptyDescription={t(
          'No aggregated usage rows match the current filters.'
        )}
        toolbarProps={null}
        skeletonKeyPrefix='usage-summary-skeleton'
        tableClassName='overflow-x-auto'
        paginationInFooter={false}
        applyHeaderSize
      />
    </div>
  )
}
