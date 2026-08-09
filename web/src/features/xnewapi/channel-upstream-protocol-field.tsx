/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Add third-party Codex dispatch policy to the default channel editor.

import type { Control } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { ChannelFormValues } from '@/features/channels/lib'

type ChannelUpstreamProtocolFieldProps = {
  channelType: number
  control: Control<ChannelFormValues>
}

export function ChannelUpstreamProtocolField(
  props: ChannelUpstreamProtocolFieldProps
) {
  const { t } = useTranslation('xnewapi')
  if (props.channelType !== 14) return null

  return (
    <FormField
      control={props.control}
      name='upstream_protocol'
      render={({ field }) => (
        <FormItem>
          <FormLabel>{t('Upstream protocol')}</FormLabel>
          <Select
            items={[
              { value: 'anthropic', label: t('Anthropic Messages') },
              { value: 'codex', label: t('Codex Responses') },
            ]}
            value={field.value}
            onValueChange={field.onChange}
          >
            <FormControl>
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
            </FormControl>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='anthropic'>
                  {t('Anthropic Messages')}
                </SelectItem>
                <SelectItem value='codex'>{t('Codex Responses')}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
          <FormDescription>
            {t(
              'Use Codex Responses when this Claude-compatible channel points to a third-party Codex upstream.'
            )}
          </FormDescription>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}
