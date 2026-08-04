/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Edit account groups, group-level channel mappings, and compact key details.

import {
  ChevronDown,
  KeyRound,
  Layers,
  Plus,
  Trash2,
  WalletCards,
  X,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Popover,
  PopoverContent,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'

import { ProviderSyncStatusBadge } from './provider-sync-status-badge'
import type {
  ProviderAccount,
  ProviderAccountStatus,
  ProviderAdapterType,
  ProviderChannel,
  ProviderGroup,
} from './types'

type ProviderAccountKeyEditorProps = {
  accounts: ProviderAccount[]
  adapterType: ProviderAdapterType
  channels: ProviderChannel[]
  formatAmount: (value: number) => string
  onAccountsChange: (accounts: ProviderAccount[]) => void
  onSyncAccount?: (account: ProviderAccount) => void
  onAdjustRecharge?: (accountId: string, delta: string) => Promise<unknown>
  onDeleteAccount?: (accountId: string) => Promise<unknown>
  onDeleteGroup?: (groupId: string) => Promise<unknown>
  syncing?: boolean
  adjustingRecharge?: boolean
  deleting?: boolean
}

const accountStatusVariant: Record<
  ProviderAccountStatus,
  'default' | 'secondary'
> = {
  healthy: 'default',
  disabled: 'secondary',
}

type DeleteTarget =
  | { type: 'account'; id: string }
  | { type: 'group'; accountId: string; id: string }

export function ProviderAccountKeyEditor(props: ProviderAccountKeyEditorProps) {
  const { t } = useTranslation('xnewapi')
  const [selectedChannelIds, setSelectedChannelIds] = useState<
    Record<string, string>
  >({})
  const [openAccountIds, setOpenAccountIds] = useState(
    () => new Set(props.accounts.slice(0, 1).map((account) => account.id))
  )
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null)
  const [rechargeAccount, setRechargeAccount] =
    useState<ProviderAccount | null>(null)
  const [rechargeDelta, setRechargeDelta] = useState('')
  const [rechargeError, setRechargeError] = useState('')
  const assignedChannelIds = useMemo(
    () =>
      new Set(
        props.accounts.flatMap((account) =>
          account.groups.flatMap((group) =>
            group.channels.map((channel) => channel.id)
          )
        )
      ),
    [props.accounts]
  )

  const updateAccount = (
    accountId: string,
    updater: (account: ProviderAccount) => ProviderAccount
  ) => {
    props.onAccountsChange(
      props.accounts.map((account) =>
        account.id === accountId ? updater(account) : account
      )
    )
  }

  const updateGroup = (
    accountId: string,
    groupId: string,
    updater: (group: ProviderGroup) => ProviderGroup
  ) => {
    updateAccount(accountId, (account) => ({
      ...account,
      groups: account.groups.map((group) =>
        group.id === groupId ? updater(group) : group
      ),
    }))
  }

  const addAccount = () => {
    const id = `new-account-${Date.now()}`
    props.onAccountsChange([
      ...props.accounts,
      {
        id,
        name: '',
        externalId: '',
        loginUsername: '',
        status: 'healthy',
        syncApiKey: '',
        autoSync: true,
        balance: 0,
        totalRecharge: 0,
        syncStatus: 'never',
        lastSyncedAt: '',
        groups: [],
      },
    ])
    setOpenAccountIds((current) => new Set(current).add(id))
  }

  const addProviderGroup = (accountId: string) => {
    const id = `new-group-${Date.now()}`
    updateAccount(accountId, (account) => ({
      ...account,
      groups: [
        ...account.groups,
        { id, name: '', groupKey: id, channels: [], keys: [], profit: null },
      ],
    }))
  }

  const addChannelMapping = (accountId: string, groupId: string) => {
    const selectedChannelId = selectedChannelIds[groupId]
    const channel = props.channels.find(
      (item) => item.id === Number(selectedChannelId)
    )
    if (!channel || assignedChannelIds.has(channel.id)) return
    updateGroup(accountId, groupId, (group) => ({
      ...group,
      channels: [...group.channels, channel],
    }))
    setSelectedChannelIds((current) => ({ ...current, [groupId]: '' }))
  }

  const removeChannelMapping = (
    accountId: string,
    groupId: string,
    channelId: number
  ) => {
    updateGroup(accountId, groupId, (group) => ({
      ...group,
      channels: group.channels.filter((channel) => channel.id !== channelId),
    }))
  }

  const setAccountOpen = (accountId: string, open: boolean) => {
    setOpenAccountIds((current) => {
      const next = new Set(current)
      if (open) next.add(accountId)
      else next.delete(accountId)
      return next
    })
  }

  const confirmDelete = async () => {
    if (!deleteTarget) return
    try {
      if (deleteTarget.type === 'account') {
        if (deleteTarget.id.startsWith('new-account-')) {
          props.onAccountsChange(
            props.accounts.filter((account) => account.id !== deleteTarget.id)
          )
        } else if (props.onDeleteAccount) {
          await props.onDeleteAccount(deleteTarget.id)
        }
      } else if (deleteTarget.id.startsWith('new-group-')) {
        updateAccount(deleteTarget.accountId, (account) => ({
          ...account,
          groups: account.groups.filter(
            (group) => group.id !== deleteTarget.id
          ),
        }))
      } else if (props.onDeleteGroup) {
        await props.onDeleteGroup(deleteTarget.id)
      }
      setDeleteTarget(null)
    } catch {
      // The mutation handler renders the server error and keeps the dialog open.
    }
  }

  const openRechargeAdjustment = (account: ProviderAccount) => {
    setRechargeAccount(account)
    setRechargeDelta('')
    setRechargeError('')
  }

  const submitRechargeAdjustment = async () => {
    if (!rechargeAccount || !props.onAdjustRecharge) return
    const delta = rechargeDelta.trim()
    const numericDelta = Number(delta)
    if (!delta || !Number.isFinite(numericDelta) || numericDelta === 0) {
      setRechargeError(t('Enter a valid recharge adjustment'))
      return
    }
    try {
      await props.onAdjustRecharge(rechargeAccount.id, delta)
      setRechargeAccount(null)
    } catch {
      // The mutation handler renders the server error and leaves the dialog open.
    }
  }

  return (
    <section className='flex flex-col gap-3'>
      <div className='flex items-center justify-between gap-3'>
        <h3 className='text-sm font-semibold'>
          {t('Accounts and provider groups')}
        </h3>
        <div className='flex items-center gap-2'>
          <Badge variant='outline'>
            {t('{{accounts}} accounts', { accounts: props.accounts.length })}
          </Badge>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={addAccount}
          >
            <Plus />
            {t('Add account')}
          </Button>
        </div>
      </div>

      <div className='flex flex-col gap-2'>
        {props.accounts.map((account) => {
          const accountOpen = openAccountIds.has(account.id)
          const keyCount = account.groups.reduce(
            (count, group) => count + group.keys.length,
            0
          )
          const externalIDLabel =
            props.adapterType === 'sub2api'
              ? t('Login username')
              : t('External account ID')
          const credentialLabel =
            props.adapterType === 'sub2api'
              ? t('Account password')
              : t('Account sync API key')
          let credentialPlaceholder = t('Enter API Key')
          if (props.adapterType === 'sub2api') {
            credentialPlaceholder = t('Enter password')
          }
          if (account.hasSyncApiKey) {
            credentialPlaceholder = t('Configured; leave blank to keep it')
          }
          return (
            <Collapsible
              key={account.id}
              open={accountOpen}
              onOpenChange={(open) => setAccountOpen(account.id, open)}
              className='rounded-lg border'
            >
              <CollapsibleTrigger
                render={
                  <button
                    type='button'
                    className='hover:bg-muted/40 flex w-full items-center justify-between gap-3 rounded-lg px-3 py-3 text-left transition-colors'
                    aria-expanded={accountOpen}
                  />
                }
              >
                <div className='flex min-w-0 items-center gap-3'>
                  <ChevronDown
                    className={cn(
                      'text-muted-foreground size-4 shrink-0 transition-transform',
                      accountOpen && 'rotate-180'
                    )}
                    aria-hidden='true'
                  />
                  <div className='min-w-0'>
                    <div className='flex items-center gap-2'>
                      <span className='truncate font-medium'>
                        {account.name || '-'}
                      </span>
                      <Badge variant={accountStatusVariant[account.status]}>
                        {account.status === 'healthy'
                          ? t('Healthy')
                          : t('Disabled')}
                      </Badge>
                      <ProviderSyncStatusBadge
                        status={account.syncStatus ?? 'never'}
                        error={account.lastSyncError}
                      />
                    </div>
                    <div className='text-muted-foreground mt-1 truncate text-xs'>
                      {account.externalId || '-'} /{' '}
                      {t('{{count}} provider groups', {
                        count: account.groups.length,
                      })}{' '}
                      / {t('{{count}} provider keys', { count: keyCount })} /{' '}
                      {account.lastSyncedAt || '-'}
                    </div>
                  </div>
                </div>
                <div className='shrink-0 text-right text-sm font-medium tabular-nums'>
                  {props.formatAmount(account.balance)} /{' '}
                  {props.formatAmount(account.totalRecharge)}
                </div>
              </CollapsibleTrigger>

              <CollapsibleContent className='border-t'>
                <div className='grid gap-3 p-3 sm:grid-cols-2'>
                  <div className='flex flex-col gap-2'>
                    <Label htmlFor={`account-name-${account.id}`}>
                      {t('Account name')}
                    </Label>
                    <Input
                      id={`account-name-${account.id}`}
                      value={account.name}
                      onChange={(event) =>
                        updateAccount(account.id, (current) => ({
                          ...current,
                          name: event.target.value,
                        }))
                      }
                    />
                  </div>
                  <div className='flex flex-col gap-2'>
                    <Label htmlFor={`account-external-id-${account.id}`}>
                      {externalIDLabel}
                    </Label>
                    <Input
                      id={`account-external-id-${account.id}`}
                      type={props.adapterType === 'sub2api' ? 'email' : 'text'}
                      value={
                        props.adapterType === 'sub2api'
                          ? account.loginUsername
                          : account.externalId
                      }
                      onChange={(event) => {
                        const value = event.target.value
                        updateAccount(account.id, (current) => {
                          if (props.adapterType === 'sub2api') {
                            return { ...current, loginUsername: value }
                          }
                          return { ...current, externalId: value }
                        })
                      }}
                      autoComplete={
                        props.adapterType === 'sub2api' ? 'username' : 'off'
                      }
                    />
                  </div>
                  <div className='flex flex-col gap-2'>
                    <Label htmlFor={`account-status-${account.id}`}>
                      {t('Status')}
                    </Label>
                    <NativeSelect
                      id={`account-status-${account.id}`}
                      className='w-full'
                      value={account.status}
                      onChange={(event) =>
                        updateAccount(account.id, (current) => ({
                          ...current,
                          status: event.target.value as ProviderAccountStatus,
                        }))
                      }
                    >
                      <NativeSelectOption value='healthy'>
                        {t('Healthy')}
                      </NativeSelectOption>
                      <NativeSelectOption value='disabled'>
                        {t('Disabled')}
                      </NativeSelectOption>
                    </NativeSelect>
                  </div>
                  <div className='flex flex-col gap-2'>
                    <Label htmlFor={`account-sync-api-key-${account.id}`}>
                      {credentialLabel}
                    </Label>
                    <Input
                      id={`account-sync-api-key-${account.id}`}
                      type='password'
                      value={account.syncApiKey}
                      onChange={(event) =>
                        updateAccount(account.id, (current) => ({
                          ...current,
                          syncApiKey: event.target.value,
                        }))
                      }
                      placeholder={credentialPlaceholder}
                      autoComplete={
                        props.adapterType === 'sub2api'
                          ? 'current-password'
                          : 'off'
                      }
                    />
                    {account.hasSyncApiKey && !account.clearSyncApiKey && (
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        onClick={() =>
                          updateAccount(account.id, (current) => ({
                            ...current,
                            syncApiKey: '',
                            clearSyncApiKey: true,
                            hasSyncApiKey: false,
                          }))
                        }
                      >
                        {t('Clear')}
                      </Button>
                    )}
                  </div>
                  <div className='flex min-h-12 items-center justify-between gap-3 rounded-lg border px-3 py-2 sm:col-span-2'>
                    <Label htmlFor={`account-auto-sync-${account.id}`}>
                      {t('Automatic synchronization')}
                    </Label>
                    <Switch
                      id={`account-auto-sync-${account.id}`}
                      checked={account.autoSync}
                      onCheckedChange={(autoSync) =>
                        updateAccount(account.id, (current) => ({
                          ...current,
                          autoSync,
                        }))
                      }
                      aria-label={t('Automatic synchronization')}
                    />
                  </div>
                  {props.onAdjustRecharge && (
                    <div className='flex items-center justify-between gap-3 rounded-lg border px-3 py-2 sm:col-span-2'>
                      <div className='min-w-0'>
                        <div className='text-sm font-medium'>
                          {t('Current total recharged')}
                        </div>
                        <div className='text-muted-foreground mt-0.5 text-xs tabular-nums'>
                          {props.formatAmount(account.totalRecharge)}
                        </div>
                      </div>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => openRechargeAdjustment(account)}
                        disabled={
                          props.adjustingRecharge ||
                          props.deleting ||
                          account.id.startsWith('new-account-')
                        }
                      >
                        <WalletCards />
                        {t('Adjust recharged amount')}
                      </Button>
                    </div>
                  )}
                  <div className='flex justify-end gap-2 sm:col-span-2'>
                    {props.onSyncAccount && (
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => props.onSyncAccount?.(account)}
                        disabled={
                          props.syncing ||
                          props.deleting ||
                          account.id.startsWith('new-account-')
                        }
                      >
                        {t('Sync this account')}
                      </Button>
                    )}
                    <Button
                      type='button'
                      variant='destructive'
                      size='sm'
                      onClick={() =>
                        setDeleteTarget({ type: 'account', id: account.id })
                      }
                      disabled={props.deleting}
                    >
                      <Trash2 />
                      {t('Delete Account')}
                    </Button>
                  </div>
                </div>

                <div className='divide-y border-t'>
                  <div className='flex justify-end p-3'>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => addProviderGroup(account.id)}
                    >
                      <Plus />
                      {t('Add provider group')}
                    </Button>
                  </div>
                  {account.groups.map((group) => {
                    const selectedChannelId = selectedChannelIds[group.id] ?? ''
                    const availableChannels = props.channels.filter(
                      (channel) =>
                        channel.status === 'available' &&
                        !assignedChannelIds.has(channel.id)
                    )
                    return (
                      <div
                        key={group.id}
                        className='grid gap-3 p-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]'
                      >
                        <div className='min-w-0'>
                          <div className='flex items-center gap-2'>
                            <Layers className='text-muted-foreground size-4 shrink-0' />
                            {group.id.startsWith('new-group-') ? (
                              <Input
                                value={group.name}
                                onChange={(event) =>
                                  updateGroup(
                                    account.id,
                                    group.id,
                                    (current) => ({
                                      ...current,
                                      name: event.target.value,
                                    })
                                  )
                                }
                                placeholder={t('Provider group name')}
                                aria-label={t('Provider group name')}
                              />
                            ) : (
                              <CopyButton
                                value={group.name}
                                size='sm'
                                tooltip={t('Copy to clipboard')}
                                className='h-auto min-w-0 justify-start gap-1 px-0 py-0 font-medium hover:bg-transparent hover:underline'
                              >
                                <span className='truncate'>{group.name}</span>
                              </CopyButton>
                            )}
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon-sm'
                              className='ml-auto shrink-0'
                              onClick={() =>
                                setDeleteTarget({
                                  type: 'group',
                                  accountId: account.id,
                                  id: group.id,
                                })
                              }
                              disabled={props.deleting}
                              aria-label={t('Delete group')}
                              title={t('Delete group')}
                            >
                              <Trash2 />
                            </Button>
                          </div>
                          <div className='mt-2 pl-6'>
                            <Popover>
                              <PopoverTrigger
                                render={
                                  <Button
                                    type='button'
                                    variant='outline'
                                    size='sm'
                                  />
                                }
                              >
                                <KeyRound />
                                {t('{{count}} provider keys', {
                                  count: group.keys.length,
                                })}
                              </PopoverTrigger>
                              <PopoverContent align='start' className='w-80'>
                                <PopoverHeader>
                                  <PopoverTitle>
                                    {t('{{count}} provider keys', {
                                      count: group.keys.length,
                                    })}
                                  </PopoverTitle>
                                </PopoverHeader>
                                <div className='divide-y'>
                                  {group.keys.length === 0 && (
                                    <div className='text-muted-foreground py-4 text-center text-xs'>
                                      {t('No provider keys')}
                                    </div>
                                  )}
                                  {group.keys.map((key) => (
                                    <div
                                      key={key.id}
                                      className='py-2 first:pt-0 last:pb-0'
                                    >
                                      <CopyButton
                                        value={key.name}
                                        size='sm'
                                        tooltip={t('Copy to clipboard')}
                                        className='h-auto max-w-full min-w-0 justify-start gap-1 px-0 py-0 font-medium hover:bg-transparent hover:underline'
                                      >
                                        <span className='truncate'>
                                          {key.name}
                                        </span>
                                      </CopyButton>
                                      <div className='text-muted-foreground mt-1 truncate font-mono text-xs'>
                                        {key.maskedKey || '-'}
                                      </div>
                                    </div>
                                  ))}
                                </div>
                              </PopoverContent>
                            </Popover>
                          </div>
                        </div>

                        <div className='flex min-w-0 flex-col gap-2'>
                          <div className='flex flex-wrap gap-1.5'>
                            {group.channels.map((channel) => (
                              <span
                                key={channel.id}
                                className={cn(
                                  'flex items-center gap-1 rounded-md py-0.5 pr-0.5 pl-2 text-xs',
                                  channel.status === 'available'
                                    ? 'bg-muted'
                                    : 'bg-muted text-muted-foreground'
                                )}
                              >
                                #{channel.id} {channel.name}
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='icon-xs'
                                  className='size-4'
                                  onClick={() =>
                                    removeChannelMapping(
                                      account.id,
                                      group.id,
                                      channel.id
                                    )
                                  }
                                  aria-label={t('Remove channel mapping')}
                                  title={t('Remove channel mapping')}
                                >
                                  <X className='size-3' />
                                </Button>
                              </span>
                            ))}
                            {group.channels.length === 0 && (
                              <Badge variant='secondary'>{t('Unmapped')}</Badge>
                            )}
                          </div>
                          <div className='flex gap-2'>
                            <NativeSelect
                              size='sm'
                              className='min-w-0 flex-1'
                              value={selectedChannelId}
                              onChange={(event) =>
                                setSelectedChannelIds((current) => ({
                                  ...current,
                                  [group.id]: event.target.value,
                                }))
                              }
                              disabled={availableChannels.length === 0}
                              aria-label={t('Select channel')}
                            >
                              <NativeSelectOption value=''>
                                {t('Select channel')}
                              </NativeSelectOption>
                              {availableChannels.map((channel) => (
                                <NativeSelectOption
                                  key={channel.id}
                                  value={String(channel.id)}
                                >
                                  #{channel.id} {channel.name}
                                </NativeSelectOption>
                              ))}
                            </NativeSelect>
                            <Button
                              type='button'
                              variant='outline'
                              size='icon-sm'
                              onClick={() =>
                                addChannelMapping(account.id, group.id)
                              }
                              disabled={!selectedChannelId}
                              aria-label={t('Add channel mapping')}
                              title={t('Add channel mapping')}
                            >
                              <Plus />
                            </Button>
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              </CollapsibleContent>
            </Collapsible>
          )
        })}
      </div>
      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
        title={
          deleteTarget?.type === 'account'
            ? t('Delete Account')
            : t('Delete group')
        }
        desc={t('This action cannot be undone.')}
        confirmText={t('Delete')}
        destructive
        isLoading={props.deleting}
        handleConfirm={confirmDelete}
      />
      <Dialog
        open={rechargeAccount !== null}
        onOpenChange={(open) => {
          if (!open && !props.adjustingRecharge) setRechargeAccount(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Recharge adjustment')}</DialogTitle>
            <DialogDescription>
              {rechargeAccount?.name ?? '-'}
            </DialogDescription>
          </DialogHeader>
          <div className='flex flex-col gap-2'>
            <Label htmlFor='provider-recharge-adjustment'>
              {t('Recharge adjustment')}
            </Label>
            <Input
              id='provider-recharge-adjustment'
              type='number'
              step='any'
              value={rechargeDelta}
              onChange={(event) => {
                setRechargeDelta(event.target.value)
                setRechargeError('')
              }}
              placeholder={t('Enter an amount such as 100 or -100')}
              aria-invalid={Boolean(rechargeError)}
              disabled={props.adjustingRecharge}
            />
            {rechargeError && (
              <p className='text-destructive text-xs'>{rechargeError}</p>
            )}
            <p className='text-muted-foreground text-xs'>
              {t(
                'Positive amounts increase the total; negative amounts reduce it, but never below zero.'
              )}
            </p>
          </div>
          <DialogFooter>
            <DialogClose
              render={<Button variant='outline' />}
              disabled={props.adjustingRecharge}
            >
              {t('Cancel')}
            </DialogClose>
            <Button
              type='button'
              onClick={submitRechargeAdjustment}
              disabled={props.adjustingRecharge}
            >
              {t('Submit adjustment')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}
