/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Port SuccessRateSelector configuration to the default frontend.

import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { JsonEditor } from '@/components/json-editor'
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
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '@/features/system-settings/components/settings-form-layout'
import { SettingsPageFormActions } from '@/features/system-settings/components/settings-page-context'
import { SettingsSection } from '@/features/system-settings/components/settings-section'
import { useResetForm } from '@/features/system-settings/hooks/use-reset-form'
import { useUpdateOption } from '@/features/system-settings/hooks/use-update-option'
import { safeNumberFieldProps } from '@/features/system-settings/utils/numeric-field'

const jsonObject = z.string().refine((value) => {
  try {
    const parsed: unknown = JSON.parse(value)
    return (
      typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)
    )
  } catch {
    return false
  }
}, 'Enter a valid JSON object')

const successRateSchema = z.object({
  enabled: z.boolean(),
  half_life_seconds: z.coerce.number().int().min(1),
  explore_rate: z.coerce.number().min(0).max(1),
  quick_downgrade: z.boolean(),
  consecutive_fail_threshold: z.coerce.number().int().min(1),
  priority_weights: jsonObject,
  immediate_disable: jsonObject,
  health_manager: jsonObject,
})

type SuccessRateFormInput = z.input<typeof successRateSchema>
type SuccessRateFormValues = z.output<typeof successRateSchema>

export type SuccessRateSettingsDefaults = {
  'channel_success_rate_setting.enabled': boolean
  'channel_success_rate_setting.half_life_seconds': number
  'channel_success_rate_setting.explore_rate': number
  'channel_success_rate_setting.quick_downgrade': boolean
  'channel_success_rate_setting.consecutive_fail_threshold': number
  'channel_success_rate_setting.priority_weights': string
  'channel_success_rate_setting.immediate_disable': string
  'channel_success_rate_setting.health_manager': string
}

type SuccessRateSettingsSectionProps = {
  defaultValues: SuccessRateSettingsDefaults
}

const PRIORITY_TEMPLATE = { 10: 0.2, 5: 0, 0: -0.1 }
const IMMEDIATE_DISABLE_TEMPLATE = {
  enabled: true,
  status_codes: [400, 401, 403, 404, 500, 502],
  error_codes: [],
  error_types: [],
}
const HEALTH_MANAGER_TEMPLATE = {
  circuit_scope: 'model',
  disable_threshold: 0.2,
  enable_threshold: 0.7,
  min_sample_size: 10,
  recovery_check_interval: 600,
  half_open_success_threshold: 2,
}

function formatJson(value: string, fallback: Record<string, unknown>): string {
  try {
    const parsed: unknown = JSON.parse(value)
    if (
      typeof parsed === 'object' &&
      parsed !== null &&
      !Array.isArray(parsed)
    ) {
      return JSON.stringify(parsed, null, 2)
    }
  } catch {
    return JSON.stringify(fallback, null, 2)
  }
  return JSON.stringify(fallback, null, 2)
}

function buildDefaults(
  defaults: SuccessRateSettingsDefaults
): SuccessRateFormInput {
  return {
    enabled: defaults['channel_success_rate_setting.enabled'] ?? false,
    half_life_seconds:
      defaults['channel_success_rate_setting.half_life_seconds'] ?? 1800,
    explore_rate: defaults['channel_success_rate_setting.explore_rate'] ?? 0.02,
    quick_downgrade:
      defaults['channel_success_rate_setting.quick_downgrade'] ?? true,
    consecutive_fail_threshold:
      defaults['channel_success_rate_setting.consecutive_fail_threshold'] ?? 3,
    priority_weights: formatJson(
      defaults['channel_success_rate_setting.priority_weights'],
      PRIORITY_TEMPLATE
    ),
    immediate_disable: formatJson(
      defaults['channel_success_rate_setting.immediate_disable'],
      IMMEDIATE_DISABLE_TEMPLATE
    ),
    health_manager: formatJson(
      defaults['channel_success_rate_setting.health_manager'],
      HEALTH_MANAGER_TEMPLATE
    ),
  }
}

function normalize(values: SuccessRateFormValues): SuccessRateSettingsDefaults {
  return {
    'channel_success_rate_setting.enabled': values.enabled,
    'channel_success_rate_setting.half_life_seconds': values.half_life_seconds,
    'channel_success_rate_setting.explore_rate': values.explore_rate,
    'channel_success_rate_setting.quick_downgrade': values.quick_downgrade,
    'channel_success_rate_setting.consecutive_fail_threshold':
      values.consecutive_fail_threshold,
    'channel_success_rate_setting.priority_weights': JSON.stringify(
      JSON.parse(values.priority_weights)
    ),
    'channel_success_rate_setting.immediate_disable': JSON.stringify(
      JSON.parse(values.immediate_disable)
    ),
    'channel_success_rate_setting.health_manager': JSON.stringify(
      JSON.parse(values.health_manager)
    ),
  }
}

export function SuccessRateSettingsSection(
  props: SuccessRateSettingsSectionProps
) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const formDefaults = useMemo(
    () => buildDefaults(props.defaultValues),
    [props.defaultValues]
  )
  const baselineRef = useRef<SuccessRateSettingsDefaults>(
    normalize(successRateSchema.parse(formDefaults))
  )
  const form = useForm<SuccessRateFormInput, unknown, SuccessRateFormValues>({
    resolver: zodResolver(successRateSchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  useEffect(() => {
    baselineRef.current = normalize(successRateSchema.parse(formDefaults))
  }, [formDefaults])

  const onSubmit = async (values: SuccessRateFormValues) => {
    const normalized = normalize(values)
    const keys = Object.keys(normalized) as Array<
      keyof SuccessRateSettingsDefaults
    >
    const changed = keys.filter(
      (key) => normalized[key] !== baselineRef.current[key]
    )
    if (changed.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    for (const key of changed) {
      await updateOption.mutateAsync({ key, value: normalized[key] })
    }
    baselineRef.current = normalized
  }

  return (
    <SettingsSection title={t('Success rate selection')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <div className='grid gap-4 lg:grid-cols-2'>
            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable SuccessRateSelector')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Prefer healthy channels using observed relay results'
                      )}
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
              name='quick_downgrade'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Quick downgrade')}</FormLabel>
                    <FormDescription>
                      {t('Penalize failures more aggressively')}
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
              name='half_life_seconds'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Sample half-life (seconds)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={60}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Controls how quickly historical outcomes decay')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='explore_rate'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Exploration rate')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={1}
                      step={0.01}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Chance of exploring another eligible channel')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='consecutive_fail_threshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Consecutive failure threshold')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Failures required before temporary circuit handling')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <Separator />

          <div className='grid gap-6 xl:grid-cols-2'>
            <FormField
              control={form.control}
              name='priority_weights'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Priority score offsets')}</FormLabel>
                  <FormControl>
                    <JsonEditor
                      value={field.value}
                      onChange={field.onChange}
                      template={PRIORITY_TEMPLATE}
                      valueType='number'
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Adjust scores for channels at each priority level')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='immediate_disable'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Immediate circuit rules')}</FormLabel>
                  <FormControl>
                    <JsonEditor
                      value={field.value}
                      onChange={field.onChange}
                      template={IMMEDIATE_DISABLE_TEMPLATE}
                      valueType='any'
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Open a circuit immediately for matching upstream errors'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='health_manager'
              render={({ field }) => (
                <FormItem className='xl:col-span-2'>
                  <FormLabel>{t('Circuit recovery policy')}</FormLabel>
                  <FormControl>
                    <JsonEditor
                      value={field.value}
                      onChange={field.onChange}
                      template={HEALTH_MANAGER_TEMPLATE}
                      valueType='any'
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Configure circuit scope, thresholds, and half-open recovery'
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
