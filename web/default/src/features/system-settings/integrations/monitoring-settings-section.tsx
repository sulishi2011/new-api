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
import { useEffect, useMemo, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  createAutoDisablePolicyGroup,
  normalizeAutoDisablePolicyGroupsString,
  parseAutoDisablePolicyGroups,
  serializeAutoDisablePolicyGroups,
  type AutoDisablePolicyGroup,
} from '@/lib/auto-disable-policy-groups'
import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

const numericString = z.string().refine((value) => {
  const trimmed = value.trim()
  if (!trimmed) return true
  return !Number.isNaN(Number(trimmed)) && Number(trimmed) >= 0
}, 'Enter a non-negative number or leave empty')

const monitoringSchema = z
  .object({
    ChannelDisableThreshold: numericString,
    QuotaRemindThreshold: numericString,
    AutomaticDisableChannelEnabled: z.boolean(),
    AutomaticEnableChannelEnabled: z.boolean(),
    AutomaticDisableKeywords: z.string(),
    AutomaticDisablePolicyGroups: z.string(),
    AutomaticDisableStatusCodes: z.string(),
    AutomaticRetryStatusCodes: z.string(),
    monitor_setting: z.object({
      auto_test_channel_enabled: z.boolean(),
      auto_test_channel_minutes: z.coerce
        .number()
        .int()
        .min(1, 'Interval must be at least 1 minute'),
      channel_failure_rate_disable_enabled: z.boolean(),
      channel_failure_rate_window_minutes: z.coerce
        .number()
        .int()
        .min(1, 'Window must be at least 1 minute'),
      channel_failure_rate_threshold: z.coerce
        .number()
        .min(1, 'Threshold must be at least 1%')
        .max(100, 'Threshold cannot exceed 100%'),
      channel_failure_rate_min_requests: z.coerce
        .number()
        .int()
        .min(1, 'Minimum requests must be at least 1'),
      request_failure_webhook_enabled: z.boolean(),
      request_failure_webhook_url: z.string(),
      request_failure_webhook_secret: z.string(),
      channel_disabled_webhook_enabled: z.boolean(),
      channel_disabled_webhook_url: z.string(),
      channel_disabled_webhook_secret: z.string(),
    }),
  })
  .superRefine((values, ctx) => {
    const validateWebhookUrl = (
      enabled: boolean,
      value: string,
      path: Array<string>
    ) => {
      if (!enabled) return
      const trimmed = value.trim()
      if (!trimmed) {
        ctx.addIssue({
          code: 'custom',
          path,
          message: 'Webhook URL is required when enabled',
        })
        return
      }
      if (!trimmed.startsWith('https://')) {
        ctx.addIssue({
          code: 'custom',
          path,
          message: 'Webhook URL must start with https://',
        })
      }
    }

    validateWebhookUrl(
      values.monitor_setting.request_failure_webhook_enabled,
      values.monitor_setting.request_failure_webhook_url,
      ['monitor_setting', 'request_failure_webhook_url']
    )
    validateWebhookUrl(
      values.monitor_setting.channel_disabled_webhook_enabled,
      values.monitor_setting.channel_disabled_webhook_url,
      ['monitor_setting', 'channel_disabled_webhook_url']
    )

    const disableParsed = parseHttpStatusCodeRules(
      values.AutomaticDisableStatusCodes
    )
    if (!disableParsed.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['AutomaticDisableStatusCodes'],
        message: `Invalid status code rules: ${disableParsed.invalidTokens.join(
          ', '
        )}`,
      })
    }

    const policyGroupsResult = normalizeAutoDisablePolicyGroupsString(
      values.AutomaticDisablePolicyGroups
    )
    if (!policyGroupsResult.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['AutomaticDisablePolicyGroups'],
        message: `Invalid auto-disable policy groups: ${policyGroupsResult.error}`,
      })
    }

    const retryParsed = parseHttpStatusCodeRules(
      values.AutomaticRetryStatusCodes
    )
    if (!retryParsed.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['AutomaticRetryStatusCodes'],
        message: `Invalid status code rules: ${retryParsed.invalidTokens.join(
          ', '
        )}`,
      })
    }
  })

type MonitoringFormValues = z.output<typeof monitoringSchema>
type MonitoringFormInput = z.input<typeof monitoringSchema>

type MonitoringSettingsSectionProps = {
  defaultValues: {
    ChannelDisableThreshold: string
    QuotaRemindThreshold: string
    AutomaticDisableChannelEnabled: boolean
    AutomaticEnableChannelEnabled: boolean
    AutomaticDisableKeywords: string
    AutomaticDisablePolicyGroups: string
    AutomaticDisableStatusCodes: string
    AutomaticRetryStatusCodes: string
    'monitor_setting.auto_test_channel_enabled': boolean
    'monitor_setting.auto_test_channel_minutes': number
    'monitor_setting.channel_failure_rate_disable_enabled': boolean
    'monitor_setting.channel_failure_rate_window_minutes': number
    'monitor_setting.channel_failure_rate_threshold': number
    'monitor_setting.channel_failure_rate_min_requests': number
    'monitor_setting.request_failure_webhook_enabled': boolean
    'monitor_setting.request_failure_webhook_url': string
    'monitor_setting.request_failure_webhook_secret': string
    'monitor_setting.channel_disabled_webhook_enabled': boolean
    'monitor_setting.channel_disabled_webhook_url': string
    'monitor_setting.channel_disabled_webhook_secret': string
  }
}

function normalizeLineEndings(value: string) {
  return value.replace(/\r\n/g, '\n')
}

type NormalizedMonitoringValues = {
  ChannelDisableThreshold: string
  QuotaRemindThreshold: string
  AutomaticDisableChannelEnabled: boolean
  AutomaticEnableChannelEnabled: boolean
  AutomaticDisableKeywords: string
  AutomaticDisablePolicyGroups: string
  AutomaticDisableStatusCodes: string
  AutomaticRetryStatusCodes: string
  'monitor_setting.auto_test_channel_enabled': boolean
  'monitor_setting.auto_test_channel_minutes': number
  'monitor_setting.channel_failure_rate_disable_enabled': boolean
  'monitor_setting.channel_failure_rate_window_minutes': number
  'monitor_setting.channel_failure_rate_threshold': number
  'monitor_setting.channel_failure_rate_min_requests': number
  'monitor_setting.request_failure_webhook_enabled': boolean
  'monitor_setting.request_failure_webhook_url': string
  'monitor_setting.request_failure_webhook_secret': string
  'monitor_setting.channel_disabled_webhook_enabled': boolean
  'monitor_setting.channel_disabled_webhook_url': string
  'monitor_setting.channel_disabled_webhook_secret': string
}

const secretOptionKeys = new Set<keyof NormalizedMonitoringValues>([
  'monitor_setting.request_failure_webhook_secret',
  'monitor_setting.channel_disabled_webhook_secret',
])

const buildFormDefaults = (
  defaults: MonitoringSettingsSectionProps['defaultValues']
): MonitoringFormInput => ({
  ChannelDisableThreshold: defaults.ChannelDisableThreshold ?? '',
  QuotaRemindThreshold: defaults.QuotaRemindThreshold ?? '',
  AutomaticDisableChannelEnabled: defaults.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: defaults.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    defaults.AutomaticDisableKeywords ?? ''
  ),
  AutomaticDisablePolicyGroups: normalizeAutoDisablePolicyGroupsString(
    defaults.AutomaticDisablePolicyGroups ?? ''
  ).value,
  AutomaticDisableStatusCodes: defaults.AutomaticDisableStatusCodes ?? '',
  AutomaticRetryStatusCodes: defaults.AutomaticRetryStatusCodes ?? '',
  monitor_setting: {
    auto_test_channel_enabled:
      defaults['monitor_setting.auto_test_channel_enabled'],
    auto_test_channel_minutes:
      defaults['monitor_setting.auto_test_channel_minutes'],
    channel_failure_rate_disable_enabled:
      defaults['monitor_setting.channel_failure_rate_disable_enabled'],
    channel_failure_rate_window_minutes:
      defaults['monitor_setting.channel_failure_rate_window_minutes'],
    channel_failure_rate_threshold:
      defaults['monitor_setting.channel_failure_rate_threshold'],
    channel_failure_rate_min_requests:
      defaults['monitor_setting.channel_failure_rate_min_requests'],
    request_failure_webhook_enabled:
      defaults['monitor_setting.request_failure_webhook_enabled'],
    request_failure_webhook_url:
      defaults['monitor_setting.request_failure_webhook_url'] ?? '',
    request_failure_webhook_secret:
      defaults['monitor_setting.request_failure_webhook_secret'] ?? '',
    channel_disabled_webhook_enabled:
      defaults['monitor_setting.channel_disabled_webhook_enabled'],
    channel_disabled_webhook_url:
      defaults['monitor_setting.channel_disabled_webhook_url'] ?? '',
    channel_disabled_webhook_secret:
      defaults['monitor_setting.channel_disabled_webhook_secret'] ?? '',
  },
})

const normalizeDefaults = (
  defaults: MonitoringSettingsSectionProps['defaultValues']
): NormalizedMonitoringValues => ({
  ChannelDisableThreshold: (defaults.ChannelDisableThreshold ?? '').trim(),
  QuotaRemindThreshold: (defaults.QuotaRemindThreshold ?? '').trim(),
  AutomaticDisableChannelEnabled: defaults.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: defaults.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    defaults.AutomaticDisableKeywords ?? ''
  ),
  AutomaticDisablePolicyGroups: normalizeAutoDisablePolicyGroupsString(
    defaults.AutomaticDisablePolicyGroups ?? ''
  ).value,
  AutomaticDisableStatusCodes: parseHttpStatusCodeRules(
    defaults.AutomaticDisableStatusCodes ?? ''
  ).normalized,
  AutomaticRetryStatusCodes: parseHttpStatusCodeRules(
    defaults.AutomaticRetryStatusCodes ?? ''
  ).normalized,
  'monitor_setting.auto_test_channel_enabled':
    defaults['monitor_setting.auto_test_channel_enabled'],
  'monitor_setting.auto_test_channel_minutes':
    defaults['monitor_setting.auto_test_channel_minutes'],
  'monitor_setting.channel_failure_rate_disable_enabled':
    defaults['monitor_setting.channel_failure_rate_disable_enabled'],
  'monitor_setting.channel_failure_rate_window_minutes':
    defaults['monitor_setting.channel_failure_rate_window_minutes'],
  'monitor_setting.channel_failure_rate_threshold':
    defaults['monitor_setting.channel_failure_rate_threshold'],
  'monitor_setting.channel_failure_rate_min_requests':
    defaults['monitor_setting.channel_failure_rate_min_requests'],
  'monitor_setting.request_failure_webhook_enabled':
    defaults['monitor_setting.request_failure_webhook_enabled'],
  'monitor_setting.request_failure_webhook_url': (
    defaults['monitor_setting.request_failure_webhook_url'] ?? ''
  ).trim(),
  'monitor_setting.request_failure_webhook_secret': (
    defaults['monitor_setting.request_failure_webhook_secret'] ?? ''
  ).trim(),
  'monitor_setting.channel_disabled_webhook_enabled':
    defaults['monitor_setting.channel_disabled_webhook_enabled'],
  'monitor_setting.channel_disabled_webhook_url': (
    defaults['monitor_setting.channel_disabled_webhook_url'] ?? ''
  ).trim(),
  'monitor_setting.channel_disabled_webhook_secret': (
    defaults['monitor_setting.channel_disabled_webhook_secret'] ?? ''
  ).trim(),
})

const normalizeFormValues = (
  values: MonitoringFormValues
): NormalizedMonitoringValues => ({
  ChannelDisableThreshold: values.ChannelDisableThreshold.trim(),
  QuotaRemindThreshold: values.QuotaRemindThreshold.trim(),
  AutomaticDisableChannelEnabled: values.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: values.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    values.AutomaticDisableKeywords
  ),
  AutomaticDisablePolicyGroups: normalizeAutoDisablePolicyGroupsString(
    values.AutomaticDisablePolicyGroups
  ).value,
  AutomaticDisableStatusCodes: parseHttpStatusCodeRules(
    values.AutomaticDisableStatusCodes
  ).normalized,
  AutomaticRetryStatusCodes: parseHttpStatusCodeRules(
    values.AutomaticRetryStatusCodes
  ).normalized,
  'monitor_setting.auto_test_channel_enabled':
    values.monitor_setting.auto_test_channel_enabled,
  'monitor_setting.auto_test_channel_minutes':
    values.monitor_setting.auto_test_channel_minutes,
  'monitor_setting.channel_failure_rate_disable_enabled':
    values.monitor_setting.channel_failure_rate_disable_enabled,
  'monitor_setting.channel_failure_rate_window_minutes':
    values.monitor_setting.channel_failure_rate_window_minutes,
  'monitor_setting.channel_failure_rate_threshold':
    values.monitor_setting.channel_failure_rate_threshold,
  'monitor_setting.channel_failure_rate_min_requests':
    values.monitor_setting.channel_failure_rate_min_requests,
  'monitor_setting.request_failure_webhook_enabled':
    values.monitor_setting.request_failure_webhook_enabled,
  'monitor_setting.request_failure_webhook_url':
    values.monitor_setting.request_failure_webhook_url.trim(),
  'monitor_setting.request_failure_webhook_secret':
    values.monitor_setting.request_failure_webhook_secret.trim(),
  'monitor_setting.channel_disabled_webhook_enabled':
    values.monitor_setting.channel_disabled_webhook_enabled,
  'monitor_setting.channel_disabled_webhook_url':
    values.monitor_setting.channel_disabled_webhook_url.trim(),
  'monitor_setting.channel_disabled_webhook_secret':
    values.monitor_setting.channel_disabled_webhook_secret.trim(),
})

function AutoDisablePolicyGroupsEditor({
  value,
  onChange,
}: {
  value: string
  onChange: (value: string) => void
}) {
  const { t } = useTranslation()
  const groups = useMemo(() => parseAutoDisablePolicyGroups(value), [value])

  const updateGroups = (nextGroups: AutoDisablePolicyGroup[]) => {
    onChange(serializeAutoDisablePolicyGroups(nextGroups))
  }

  const updateGroup = (
    index: number,
    patch: Partial<AutoDisablePolicyGroup>
  ) => {
    updateGroups(
      groups.map((group, groupIndex) =>
        groupIndex === index ? { ...group, ...patch } : group
      )
    )
  }
  const createUniqueGroupName = () => {
    const baseName = t('New policy group')
    const usedNames = new Set(groups.map((group) => group.name.toLowerCase()))
    let name = baseName
    let suffix = 2
    while (usedNames.has(name.toLowerCase())) {
      name = `${baseName} ${suffix}`
      suffix += 1
    }
    return name
  }

  return (
    <div className='space-y-4 rounded-md border p-4'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='space-y-1'>
          <div className='text-sm font-medium'>
            {t('Auto-disable policy groups')}
          </div>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Policy groups let selected channels use dedicated failure keywords instead of the default keyword list.'
            )}
          </p>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Disabled groups are not applied; channels assigned to them fall back to the default keywords.'
            )}
          </p>
        </div>
        <Button
          type='button'
          size='sm'
          onClick={() =>
            updateGroups([
              ...groups,
              createAutoDisablePolicyGroup(createUniqueGroupName()),
            ])
          }
        >
          <Plus className='mr-2 h-4 w-4' />
          {t('Add policy group')}
        </Button>
      </div>

      {groups.length === 0 ? (
        <div className='text-muted-foreground rounded-md border border-dashed p-4 text-sm'>
          {t('No policy groups configured')}
        </div>
      ) : (
        <div className='space-y-3'>
          {groups.map((group, index) => (
            <div key={group.id} className='space-y-3 rounded-md border p-3'>
              <div className='grid gap-3 md:grid-cols-[1fr_auto]'>
                <div className='space-y-2'>
                  <FormLabel>{t('Policy group name')}</FormLabel>
                  <Input
                    value={group.name}
                    onChange={(event) =>
                      updateGroup(index, { name: event.target.value })
                    }
                    placeholder={t('Policy group name')}
                  />
                </div>
                <div className='flex items-end gap-3 pb-2'>
                  <div className='space-y-1'>
                    <FormLabel>{t('Enabled')}</FormLabel>
                    <Switch
                      checked={group.enabled}
                      onCheckedChange={(enabled) =>
                        updateGroup(index, { enabled })
                      }
                    />
                  </div>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='text-destructive'
                    title={t('Delete policy group')}
                    onClick={() =>
                      updateGroups(
                        groups.filter((_, groupIndex) => groupIndex !== index)
                      )
                    }
                  >
                    <Trash2 className='h-4 w-4' />
                  </Button>
                </div>
              </div>
              <div className='space-y-2'>
                <FormLabel>{t('Keywords')}</FormLabel>
                <Textarea
                  rows={4}
                  value={group.keywords.join('\n')}
                  placeholder={t('one keyword per line')}
                  onChange={(event) =>
                    updateGroup(index, {
                      keywords: event.target.value.split('\n'),
                    })
                  }
                />
                <p className='text-muted-foreground text-sm'>
                  {t('One keyword per line for this policy group.')}
                </p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export function MonitoringSettingsSection({
  defaultValues,
}: MonitoringSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const normalizedDefaults = useMemo(
    () => normalizeDefaults(defaultValues),
    [defaultValues]
  )
  const baselineRef = useRef<NormalizedMonitoringValues>(normalizedDefaults)

  useEffect(() => {
    baselineRef.current = normalizedDefaults
  }, [normalizedDefaults])

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<MonitoringFormInput, unknown, MonitoringFormValues>({
    resolver: zodResolver(monitoringSchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  const autoDisableStatusCodes = form.watch('AutomaticDisableStatusCodes')
  const autoRetryStatusCodes = form.watch('AutomaticRetryStatusCodes')
  const failureRateAutoDisableEnabled = form.watch(
    'monitor_setting.channel_failure_rate_disable_enabled'
  )
  const autoDisableParsed = useMemo(
    () => parseHttpStatusCodeRules(autoDisableStatusCodes),
    [autoDisableStatusCodes]
  )
  const autoRetryParsed = useMemo(
    () => parseHttpStatusCodeRules(autoRetryStatusCodes),
    [autoRetryStatusCodes]
  )

  const onSubmit = async (values: MonitoringFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (
      Object.keys(normalized) as Array<keyof NormalizedMonitoringValues>
    ).filter((key) => {
      const value = normalized[key]
      if (secretOptionKeys.has(key) && String(value).trim() === '') {
        return false
      }
      return value !== baselineRef.current[key]
    })

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of updates) {
      const value = normalized[key]
      await updateOption.mutateAsync({
        key,
        value,
      })
    }

    baselineRef.current = {
      ...normalized,
      'monitor_setting.request_failure_webhook_secret': '',
      'monitor_setting.channel_disabled_webhook_secret': '',
    }
    form.setValue('monitor_setting.request_failure_webhook_secret', '')
    form.setValue('monitor_setting.channel_disabled_webhook_secret', '')
  }

  return (
    <SettingsSection title={t('Monitoring & Alerts')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save monitoring rules'
          />
          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='monitor_setting.auto_test_channel_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Scheduled channel tests')}</FormLabel>
                    <FormDescription>
                      {t('Automatically probe all channels in the background')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='monitor_setting.auto_test_channel_minutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Test interval (minutes)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('How frequently the system tests all channels')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='ChannelDisableThreshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Disable threshold (seconds)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Automatically disable channels exceeding this response time'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaRemindThreshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Quota reminder (tokens)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Send email alerts when a user falls below this quota')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='AutomaticDisableChannelEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Disable on failure')}</FormLabel>
                    <FormDescription>
                      {t('Automatically disable channels when tests fail')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='AutomaticEnableChannelEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Re-enable on success')}</FormLabel>
                    <FormDescription>
                      {t('Bring channels back online after successful checks')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='monitor_setting.channel_failure_rate_disable_enabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                <div className='space-y-0.5'>
                  <FormLabel className='text-base'>
                    {t('Failure rate auto-disable')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Disable channels when recent request failures exceed the configured rate'
                    )}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          {failureRateAutoDisableEnabled && (
            <div className='grid gap-6 md:grid-cols-3'>
              <FormField
                control={form.control}
                name='monitor_setting.channel_failure_rate_window_minutes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Failure rate window (minutes)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={
                          typeof field.value === 'number' &&
                          Number.isFinite(field.value)
                            ? field.value
                            : ''
                        }
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='monitor_setting.channel_failure_rate_threshold'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Request failure threshold (%)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={100}
                        step={1}
                        value={
                          typeof field.value === 'number' &&
                          Number.isFinite(field.value)
                            ? field.value
                            : ''
                        }
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='monitor_setting.channel_failure_rate_min_requests'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Minimum requests')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={
                          typeof field.value === 'number' &&
                          Number.isFinite(field.value)
                            ? field.value
                            : ''
                        }
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Minimum request count before failure-rate rules can disable a channel'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          )}

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='monitor_setting.request_failure_webhook_enabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Request failure Webhook')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Send a webhook notification whenever an upstream request fails'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='monitor_setting.channel_disabled_webhook_enabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('Channel disabled Webhook')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Send a webhook notification when a channel is automatically disabled'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='monitor_setting.request_failure_webhook_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Request failure Webhook URL')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('https://example.com/webhook')}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Webhook URL must start with https://')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='monitor_setting.request_failure_webhook_secret'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Request failure Webhook secret')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder={t(
                        'Webhook signing secret (leave blank unless updating)'
                      )}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='monitor_setting.channel_disabled_webhook_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Channel disabled Webhook URL')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('https://example.com/webhook')}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Webhook URL must start with https://')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='monitor_setting.channel_disabled_webhook_secret'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Channel disabled Webhook secret')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder={t(
                        'Webhook signing secret (leave blank unless updating)'
                      )}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='AutomaticDisableKeywords'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Failure keywords')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={6}
                    placeholder={t('one keyword per line')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'If an upstream error contains any of these keywords (case insensitive), the channel will be disabled automatically.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='AutomaticDisablePolicyGroups'
            render={({ field }) => (
              <FormItem>
                <FormControl>
                  <AutoDisablePolicyGroupsEditor
                    value={field.value}
                    onChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='AutomaticDisableStatusCodes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Auto-disable status codes')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('e.g. 401, 403, 429, 500-599')}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Accepts comma-separated status codes and inclusive ranges.'
                    )}{' '}
                    {autoDisableParsed.ok &&
                      autoDisableParsed.normalized &&
                      autoDisableParsed.normalized !== field.value.trim() && (
                        <span className='text-muted-foreground'>
                          {t('Normalized:')} {autoDisableParsed.normalized}
                        </span>
                      )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='AutomaticRetryStatusCodes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Auto-retry status codes')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('e.g. 401, 403, 429, 500-599')}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Accepts comma-separated status codes and inclusive ranges.'
                    )}{' '}
                    {autoRetryParsed.ok &&
                      autoRetryParsed.normalized &&
                      autoRetryParsed.normalized !== field.value.trim() && (
                        <span className='text-muted-foreground'>
                          {t('Normalized:')} {autoRetryParsed.normalized}
                        </span>
                      )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
