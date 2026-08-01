/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Present providers and channel-mapped upstream groups without nested key rows.

import type { TFunction } from 'i18next'
import {
  ChevronDown,
  ChevronRight,
  KeyRound,
  Layers,
  MoreHorizontal,
  Pencil,
} from 'lucide-react'
import { Fragment } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Popover,
  PopoverContent,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { cn } from '@/lib/utils'

import { ProviderSyncStatusBadge } from './provider-sync-status-badge'
import { getProviderTotals } from './provider-totals'
import type {
  ProviderAdapterType,
  ProviderKey,
  ProviderKeyStatus,
  UpstreamProvider,
  UpstreamProviderStatus,
} from './types'

const providerAdapterTypeLabel: Record<ProviderAdapterType, string> = {
  newapi: 'NewAPI',
  sub2api: 'Sub2API',
}

type ProviderTableProps = {
  providers: UpstreamProvider[]
  expandedIds: Set<string>
  formatAmount: (value: number) => string
  onToggle: (providerId: string) => void
  onEdit: (provider: UpstreamProvider) => void
}

const providerStatusVariant: Record<
  UpstreamProviderStatus,
  'default' | 'secondary' | 'outline'
> = {
  healthy: 'default',
  disabled: 'secondary',
}

const keyStatusVariant: Record<
  ProviderKeyStatus,
  'default' | 'secondary' | 'outline'
> = {
  active: 'default',
  disabled: 'secondary',
}

function getStatusLabel(
  status: UpstreamProviderStatus | ProviderKeyStatus,
  t: TFunction
): string {
  if (status === 'healthy') return t('Healthy')
  if (status === 'active') return t('Active')
  return t('Disabled')
}

function ProviderKeySummary(props: { keys: ProviderKey[] }) {
  const { t } = useTranslation('xnewapi')
  if (props.keys.length === 0) {
    return <span className='text-muted-foreground text-xs'>-</span>
  }
  if (props.keys.length === 1) {
    const key = props.keys[0]
    return (
      <div className='min-w-0'>
        <div className='flex min-w-0 items-center gap-1.5'>
          <KeyRound className='text-muted-foreground size-3.5 shrink-0' />
          <CopyButton
            value={key.name}
            size='sm'
            tooltip={t('Copy to clipboard')}
            className='h-auto min-w-0 justify-start gap-1 px-0 py-0 hover:bg-transparent hover:underline'
          >
            <span className='truncate'>{key.name}</span>
          </CopyButton>
          <Badge variant={keyStatusVariant[key.status]}>
            {getStatusLabel(key.status, t)}
          </Badge>
        </div>
        <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
          {key.maskedKey || '-'}
        </div>
      </div>
    )
  }
  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button variant='outline' size='sm' className='max-w-full gap-1.5' />
        }
      >
        <KeyRound />
        {t('{{count}} provider keys', { count: props.keys.length })}
      </PopoverTrigger>
      <PopoverContent align='start' className='w-80'>
        <PopoverHeader>
          <PopoverTitle>
            {t('{{count}} provider keys', { count: props.keys.length })}
          </PopoverTitle>
        </PopoverHeader>
        <div className='divide-y'>
          {props.keys.map((key) => (
            <div
              key={key.id}
              className='flex items-center gap-2 py-2 first:pt-0 last:pb-0'
            >
              <div className='min-w-0 flex-1'>
                <CopyButton
                  value={key.name}
                  size='sm'
                  tooltip={t('Copy to clipboard')}
                  className='h-auto max-w-full min-w-0 justify-start gap-1 px-0 py-0 font-medium hover:bg-transparent hover:underline'
                >
                  <span className='truncate'>{key.name}</span>
                </CopyButton>
                <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
                  {key.maskedKey || '-'}
                </div>
              </div>
              <Badge variant={keyStatusVariant[key.status]}>
                {getStatusLabel(key.status, t)}
              </Badge>
            </div>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  )
}

export function ProviderTable(props: ProviderTableProps) {
  const { t } = useTranslation('xnewapi')

  if (props.providers.length === 0) {
    return (
      <div className='text-muted-foreground rounded-lg border border-dashed px-4 py-16 text-center text-sm'>
        {t('No model providers found')}
      </div>
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <Table className='min-w-[1120px]'>
        <TableHeader className='bg-muted/40'>
          <TableRow>
            <TableHead className='w-[24%] pl-3'>
              {t('Model provider')}
            </TableHead>
            <TableHead className='w-[16%]'>{t('Accounts / groups')}</TableHead>
            <TableHead className='w-[17%]'>{t('Channel mappings')}</TableHead>
            <TableHead className='w-[12%] text-right'>
              {t('Balance / recharged')}
            </TableHead>
            <TableHead className='w-[15%] text-right'>
              {t('Revenue / cost')}
            </TableHead>
            <TableHead className='w-[11%]'>{t('Sync status')}</TableHead>
            <TableHead className='w-[5%] text-right'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.providers.map((provider) => {
            const totals = getProviderTotals(provider)
            const expanded = props.expandedIds.has(provider.id)
            return (
              <Fragment key={provider.id}>
                <TableRow aria-expanded={expanded}>
                  <TableCell className='pl-2'>
                    <div className='flex min-w-0 items-center gap-2'>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        onClick={() => props.onToggle(provider.id)}
                        aria-label={
                          expanded
                            ? t('Collapse provider')
                            : t('Expand provider')
                        }
                        title={
                          expanded
                            ? t('Collapse provider')
                            : t('Expand provider')
                        }
                      >
                        {expanded ? <ChevronDown /> : <ChevronRight />}
                      </Button>
                      <div className='min-w-0'>
                        <div className='flex min-w-0 items-center gap-2'>
                          <button
                            type='button'
                            className='truncate text-left font-medium hover:underline'
                            onClick={() => props.onEdit(provider)}
                          >
                            {provider.name}
                          </button>
                          <Badge
                            variant={providerStatusVariant[provider.status]}
                          >
                            {getStatusLabel(provider.status, t)}
                          </Badge>
                          <Badge variant='outline'>
                            {providerAdapterTypeLabel[provider.adapterType]}
                          </Badge>
                        </div>
                        {provider.endpoint && (
                          <a
                            href={provider.endpoint}
                            target='_blank'
                            rel='noreferrer'
                            className='text-muted-foreground mt-1 block truncate text-xs hover:underline'
                            title={provider.endpoint}
                          >
                            {provider.endpoint}
                          </a>
                        )}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className='font-medium tabular-nums'>
                      {t('{{accounts}} accounts', {
                        accounts: totals.accountCount,
                      })}
                    </div>
                    <div className='text-muted-foreground mt-1 text-xs tabular-nums'>
                      {t('{{count}} provider groups', {
                        count: totals.groupCount,
                      })}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className='font-medium tabular-nums'>
                      {t('{{mapped}} of {{total}} groups mapped', {
                        mapped: totals.mappedGroupCount,
                        total: totals.groupCount,
                      })}
                    </div>
                    <div className='text-muted-foreground mt-1 text-xs'>
                      {totals.groupCount === totals.mappedGroupCount
                        ? t('All groups mapped')
                        : t('Mapping required')}
                    </div>
                  </TableCell>
                  <TableCell className='text-right font-medium tabular-nums'>
                    {props.formatAmount(totals.balance)} /{' '}
                    {props.formatAmount(totals.totalRecharge)}
                  </TableCell>
                  <TableCell className='text-right'>
                    <div className='font-medium tabular-nums'>
                      {props.formatAmount(totals.revenue)} /{' '}
                      {totals.cost === null
                        ? '-'
                        : props.formatAmount(totals.cost)}
                    </div>
                    <div
                      className={cn(
                        'mt-1 text-xs tabular-nums',
                        totals.profit !== null && totals.profit < 0
                          ? 'text-destructive'
                          : 'text-muted-foreground'
                      )}
                    >
                      {t('Gross margin')}:{' '}
                      {totals.margin === null
                        ? '-'
                        : `${totals.margin.toFixed(1)}%`}
                    </div>
                  </TableCell>
                  <TableCell>
                    <ProviderSyncStatusBadge
                      status={provider.syncStatus}
                      error={provider.lastSyncError}
                    />
                    <div className='text-muted-foreground mt-1 text-xs tabular-nums'>
                      {provider.lastSyncedAt ?? '-'}
                    </div>
                  </TableCell>
                  <TableCell className='text-right'>
                    <DropdownMenu>
                      <DropdownMenuTrigger
                        render={
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            aria-label={t('Model provider actions')}
                            title={t('Model provider actions')}
                          />
                        }
                      >
                        <MoreHorizontal />
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align='end'>
                        <DropdownMenuItem
                          onClick={() => props.onEdit(provider)}
                        >
                          <Pencil />
                          {t('Edit')}
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>

                {expanded &&
                  provider.accounts.flatMap((account) =>
                    account.groups.map((group) => {
                      const profit = group.profit
                      return (
                        <TableRow
                          key={group.groupKey}
                          className='bg-muted/10 hover:bg-muted/20'
                        >
                          <TableCell className='pl-8'>
                            <div className='flex min-w-0 items-center gap-2'>
                              <span className='bg-background flex size-7 shrink-0 items-center justify-center rounded-md border'>
                                <Layers className='text-muted-foreground size-3.5' />
                              </span>
                              <div className='min-w-0'>
                                <CopyButton
                                  value={group.name}
                                  size='sm'
                                  tooltip={t('Copy to clipboard')}
                                  className='h-auto max-w-full min-w-0 justify-start gap-1 px-0 py-0 font-medium hover:bg-transparent hover:underline'
                                >
                                  <span className='truncate'>
                                    {group.name || '-'}
                                  </span>
                                </CopyButton>
                                <div className='text-muted-foreground mt-1 truncate text-xs'>
                                  {account.name} / {account.externalId || '-'}
                                </div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell>
                            <ProviderKeySummary keys={group.keys} />
                          </TableCell>
                          <TableCell>
                            <div className='flex flex-wrap gap-1.5'>
                              {group.channels.length > 0 ? (
                                group.channels.map((channel) => (
                                  <Badge
                                    key={channel.id}
                                    variant={
                                      channel.status === 'available'
                                        ? 'outline'
                                        : 'secondary'
                                    }
                                  >
                                    #{channel.id} {channel.name}
                                  </Badge>
                                ))
                              ) : (
                                <Badge variant='secondary'>
                                  {t('Unmapped')}
                                </Badge>
                              )}
                            </div>
                          </TableCell>
                          <TableCell />
                          <TableCell className='text-right'>
                            <div className='font-medium tabular-nums'>
                              {props.formatAmount(profit?.revenue ?? 0)} /{' '}
                              {profit?.cost == null
                                ? '-'
                                : props.formatAmount(profit.cost)}
                            </div>
                            <div className='text-muted-foreground mt-1 text-xs tabular-nums'>
                              {t('Gross margin')}:{' '}
                              {profit?.grossMargin == null
                                ? '-'
                                : `${profit.grossMargin.toFixed(1)}%`}
                            </div>
                          </TableCell>
                          <TableCell className='text-muted-foreground text-xs tabular-nums'>
                            {account.lastSyncedAt || '-'}
                          </TableCell>
                          <TableCell />
                        </TableRow>
                      )
                    })
                  )}
              </Fragment>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
