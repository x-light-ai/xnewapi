/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Keep provider profitability filters scoped to daily aggregate data.

import dayjs from 'dayjs'
import { CalendarDays } from 'lucide-react'
import type { ReactElement } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import type { ProviderProfitDateRange } from './types'

type ProviderDateRangeProps = {
  value: ProviderProfitDateRange
  onChange: (value: ProviderProfitDateRange) => void
}

export function ProviderDateRangePicker(
  props: ProviderDateRangeProps
): ReactElement {
  const { t } = useTranslation('xnewapi')
  const today = dayjs().format('YYYY-MM-DD')

  const applyRange = (startDate: string, endDate: string) => {
    props.onChange({ startDate, endDate })
  }

  const applyToday = () => applyRange(today, today)
  const applySevenDays = () =>
    applyRange(dayjs().subtract(6, 'day').format('YYYY-MM-DD'), today)
  const applyThisMonth = () =>
    applyRange(dayjs().startOf('month').format('YYYY-MM-DD'), today)

  return (
    <div className='flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-end'>
      <div className='flex min-w-0 flex-1 flex-col gap-1.5 sm:max-w-md'>
        <div className='text-muted-foreground flex items-center gap-1.5 text-xs font-medium'>
          <CalendarDays className='size-3.5' aria-hidden />
          {t('Date range')}
        </div>
        <div className='grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-2'>
          <Input
            type='date'
            value={props.value.startDate}
            max={props.value.endDate}
            onChange={(event) => {
              const startDate = event.target.value
              if (!startDate) return
              applyRange(
                startDate,
                startDate > props.value.endDate
                  ? startDate
                  : props.value.endDate
              )
            }}
            aria-label={t('Start date')}
          />
          <span className='text-muted-foreground text-sm' aria-hidden>
            -
          </span>
          <Input
            type='date'
            value={props.value.endDate}
            min={props.value.startDate}
            max={today}
            onChange={(event) => {
              const endDate = event.target.value
              if (!endDate) return
              applyRange(
                endDate < props.value.startDate
                  ? endDate
                  : props.value.startDate,
                endDate
              )
            }}
            aria-label={t('End date')}
          />
        </div>
      </div>
      <div className='flex flex-wrap gap-1.5'>
        <Button type='button' variant='outline' size='sm' onClick={applyToday}>
          {t('Today')}
        </Button>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={applySevenDays}
        >
          {t('Last 7 days')}
        </Button>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={applyThisMonth}
        >
          {t('This month')}
        </Button>
      </div>
    </div>
  )
}
