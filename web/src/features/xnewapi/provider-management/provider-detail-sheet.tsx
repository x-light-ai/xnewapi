/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: Keep upstream provider editing and relationship inspection in one owned sheet.

import { Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'

import { ProviderAccountKeyEditor } from './provider-account-key-editor'
import { ProviderSyncStatusBadge } from './provider-sync-status-badge'
import type {
  ProviderAuthenticationMethod,
  ProviderAdapterType,
  ProviderAccount,
  ProviderChannel,
  UpstreamProvider,
  UpstreamProviderStatus,
} from './types'

type ProviderDetailSheetProps = {
  open: boolean
  provider: UpstreamProvider | null
  channels: ProviderChannel[]
  saving?: boolean
  formatAmount: (value: number) => string
  onOpenChange: (open: boolean) => void
  onSave: (provider: UpstreamProvider) => void
  onSync?: (provider: UpstreamProvider) => void
  onSyncAccount?: (account: ProviderAccount) => void
  onAdjustRecharge?: (accountId: string, delta: string) => Promise<unknown>
  onDelete?: (provider: UpstreamProvider) => Promise<unknown>
  onDeleteAccount?: (accountId: string) => Promise<unknown>
  onDeleteGroup?: (groupId: string) => Promise<unknown>
  syncing?: boolean
  adjustingRecharge?: boolean
  deleting?: boolean
}

export function ProviderDetailSheet(props: ProviderDetailSheetProps) {
  const { t } = useTranslation('xnewapi')
  const [name, setName] = useState(() => props.provider?.name ?? '')
  const [endpoint, setEndpoint] = useState(() => props.provider?.endpoint ?? '')
  const [status, setStatus] = useState<UpstreamProviderStatus>(
    () => props.provider?.status ?? 'healthy'
  )
  const [authenticationMethod, setAuthenticationMethod] =
    useState<ProviderAuthenticationMethod>(
      () => props.provider?.authenticationMethod ?? 'api-key'
    )
  const [adapterType, setAdapterType] = useState<ProviderAdapterType>(
    () => props.provider?.adapterType ?? 'newapi'
  )
  const [syncIntervalMinutes, setSyncIntervalMinutes] = useState(() =>
    String(props.provider?.syncIntervalMinutes ?? 60)
  )
  const [rechargeRatio, setRechargeRatio] = useState(() =>
    String(props.provider?.rechargeRatio ?? 1)
  )
  const [quotaConversionBase, setQuotaConversionBase] = useState(() =>
    String(props.provider?.quotaConversionBase ?? 500000)
  )
  const [accounts, setAccounts] = useState(() => props.provider?.accounts ?? [])
  const [syncConfirmationOpen, setSyncConfirmationOpen] = useState(false)
  const [accountSyncTarget, setAccountSyncTarget] =
    useState<ProviderAccount | null>(null)
  const [deleteConfirmationOpen, setDeleteConfirmationOpen] = useState(false)
  const formValid =
    name.trim().length > 0 &&
    accounts.every(
      (account) =>
        account.name.trim().length > 0 &&
        (adapterType !== 'sub2api' ||
          (account.loginUsername.trim().length > 0 &&
            (account.hasSyncApiKey || account.syncApiKey.length > 0))) &&
        account.groups.every((group) => group.name.trim().length > 0)
    )
  const handleSync = () => {
    if (props.provider && props.onSync) setSyncConfirmationOpen(true)
  }
  const confirmSync = () => {
    if (!props.provider || !props.onSync) return
    props.onSync(props.provider)
    setSyncConfirmationOpen(false)
  }
  const requestAccountSync = (account: ProviderAccount) => {
    setAccountSyncTarget(account)
  }
  const confirmAccountSync = () => {
    if (!accountSyncTarget || !props.onSyncAccount) return
    props.onSyncAccount(accountSyncTarget)
    setAccountSyncTarget(null)
  }
  const confirmDelete = async () => {
    if (!props.provider || !props.onDelete) return
    try {
      await props.onDelete(props.provider)
      setDeleteConfirmationOpen(false)
    } catch {
      // The mutation handler renders the server error and keeps the dialog open.
    }
  }

  const saveProvider = () => {
    const trimmedName = name.trim()
    if (!formValid) return
    const parsedInterval = Number.parseInt(syncIntervalMinutes, 10)
    const normalizedInterval = Number.isFinite(parsedInterval)
      ? Math.min(1440, Math.max(1, parsedInterval))
      : 60
    const parsedRechargeRatio = Number.parseFloat(rechargeRatio)
    const normalizedRechargeRatio =
      Number.isFinite(parsedRechargeRatio) && parsedRechargeRatio > 0
        ? parsedRechargeRatio
        : 1
    const parsedQuotaConversionBase = Number.parseInt(quotaConversionBase, 10)
    let normalizedQuotaConversionBase = 1
    if (adapterType === 'newapi') {
      normalizedQuotaConversionBase = Number.isFinite(parsedQuotaConversionBase)
        ? Math.max(1, parsedQuotaConversionBase)
        : 500000
    }

    props.onSave({
      id: props.provider?.id ?? `provider-${Date.now()}`,
      name: trimmedName,
      endpoint: endpoint.trim(),
      status,
      syncStatus: props.provider?.syncStatus ?? 'never',
      lastSyncedAt: props.provider?.lastSyncedAt ?? null,
      lastSyncError: props.provider?.lastSyncError,
      authenticationMethod,
      adapterType,
      syncIntervalMinutes: normalizedInterval,
      rechargeRatio: normalizedRechargeRatio,
      quotaConversionBase: normalizedQuotaConversionBase,
      currency: props.provider?.currency ?? 'USD',
      accounts,
    })
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className='w-full sm:max-w-2xl'>
        <SheetHeader className='border-b pr-12'>
          <SheetTitle>
            {props.provider ? t('Edit') : t('Add model provider')}
          </SheetTitle>
          <SheetDescription>
            {t(
              'Provider profile, accounts, groups, keys, and channel mappings'
            )}
          </SheetDescription>
        </SheetHeader>

        <ScrollArea className='min-h-0 flex-1'>
          <div className='flex flex-col gap-6 p-4'>
            <section className='flex flex-col gap-4'>
              <h3 className='text-sm font-semibold'>{t('Provider profile')}</h3>
              <div className='grid gap-4 sm:grid-cols-2'>
                <div className='flex flex-col gap-2'>
                  <Label htmlFor='provider-adapter-type'>
                    {t('Provider type')}
                  </Label>
                  <NativeSelect
                    id='provider-adapter-type'
                    className='w-full'
                    value={adapterType}
                    onChange={(event) => {
                      const nextAdapter = event.target
                        .value as ProviderAdapterType
                      if (nextAdapter !== adapterType) {
                        setAccounts((current) =>
                          current.map((account) => ({
                            ...account,
                            syncApiKey: '',
                            hasSyncApiKey: false,
                            clearSyncApiKey: true,
                          }))
                        )
                      }
                      setAdapterType(nextAdapter)
                      setAuthenticationMethod(
                        nextAdapter === 'sub2api' ? 'password' : 'api-key'
                      )
                    }}
                  >
                    <NativeSelectOption value='newapi'>
                      NewAPI
                    </NativeSelectOption>
                    <NativeSelectOption value='sub2api'>
                      Sub2API
                    </NativeSelectOption>
                  </NativeSelect>
                </div>
                <div className='flex flex-col gap-2'>
                  <Label htmlFor='provider-name'>
                    {t('Model provider name')}
                  </Label>
                  <Input
                    id='provider-name'
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    placeholder={t('Model provider name')}
                  />
                </div>
                <div className='flex flex-col gap-2 sm:col-span-2'>
                  <Label htmlFor='provider-endpoint'>{t('API endpoint')}</Label>
                  <Input
                    id='provider-endpoint'
                    value={endpoint}
                    onChange={(event) => setEndpoint(event.target.value)}
                    placeholder='https://api.example.com'
                  />
                </div>
                <div className='flex flex-col gap-2'>
                  <Label htmlFor='provider-status'>{t('Status')}</Label>
                  <NativeSelect
                    id='provider-status'
                    className='w-full'
                    value={status}
                    onChange={(event) =>
                      setStatus(event.target.value as UpstreamProviderStatus)
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
                  <Label>{t('Sync status')}</Label>
                  <div className='flex min-h-8 items-center gap-2 rounded-lg border px-2.5 py-1'>
                    <ProviderSyncStatusBadge
                      status={props.provider?.syncStatus ?? 'never'}
                      error={props.provider?.lastSyncError}
                    />
                    <span className='text-muted-foreground truncate text-xs'>
                      {props.provider?.lastSyncedAt
                        ? `${t('Last synced')}: ${props.provider.lastSyncedAt}`
                        : t('Not synced')}
                    </span>
                  </div>
                </div>
              </div>
            </section>

            <section className='flex flex-col gap-4'>
              <h3 className='text-sm font-semibold'>
                {t('Upstream synchronization')}
              </h3>
              <div className='grid gap-4 sm:grid-cols-2'>
                <div className='flex flex-col gap-2'>
                  <Label htmlFor='provider-authentication-method'>
                    {t('Authentication method')}
                  </Label>
                  <NativeSelect
                    id='provider-authentication-method'
                    className='w-full'
                    value={authenticationMethod}
                    disabled
                  >
                    {adapterType === 'sub2api' ? (
                      <NativeSelectOption value='password'>
                        {t('Email and password')}
                      </NativeSelectOption>
                    ) : (
                      <NativeSelectOption value='api-key'>
                        {t('API Key')}
                      </NativeSelectOption>
                    )}
                  </NativeSelect>
                </div>
                <div className='flex flex-col gap-2'>
                  <Label htmlFor='provider-recharge-ratio'>
                    {t('Recharge ratio')}
                  </Label>
                  <Input
                    id='provider-recharge-ratio'
                    type='number'
                    min={0}
                    step='any'
                    value={rechargeRatio}
                    onChange={(event) => setRechargeRatio(event.target.value)}
                  />
                </div>
                {adapterType === 'newapi' && (
                  <div className='flex flex-col gap-2'>
                    <Label htmlFor='provider-quota-conversion-base'>
                      {t('Quota conversion base')}
                    </Label>
                    <Input
                      id='provider-quota-conversion-base'
                      type='number'
                      min={1}
                      step={1}
                      value={quotaConversionBase}
                      onChange={(event) =>
                        setQuotaConversionBase(event.target.value)
                      }
                    />
                  </div>
                )}
                <div className='flex flex-col gap-2'>
                  <Label htmlFor='provider-sync-interval'>
                    {t('Sync interval (minutes)')}
                  </Label>
                  <Input
                    id='provider-sync-interval'
                    type='number'
                    min={1}
                    max={1440}
                    step={1}
                    value={syncIntervalMinutes}
                    onChange={(event) =>
                      setSyncIntervalMinutes(event.target.value)
                    }
                  />
                </div>
              </div>
            </section>

            <ProviderAccountKeyEditor
              accounts={accounts}
              adapterType={adapterType}
              channels={props.channels}
              formatAmount={props.formatAmount}
              onAccountsChange={setAccounts}
              onSyncAccount={
                props.provider && props.onSyncAccount
                  ? requestAccountSync
                  : undefined
              }
              onAdjustRecharge={
                props.provider ? props.onAdjustRecharge : undefined
              }
              onDeleteAccount={props.onDeleteAccount}
              onDeleteGroup={props.onDeleteGroup}
              syncing={props.syncing}
              adjustingRecharge={props.adjustingRecharge}
              deleting={props.deleting}
            />
          </div>
        </ScrollArea>

        <SheetFooter className='flex-row justify-end border-t'>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          {props.provider && props.onSync && (
            <Button
              variant='outline'
              onClick={handleSync}
              disabled={props.syncing || props.deleting}
            >
              {t('Sync now')}
            </Button>
          )}
          {props.provider && props.onDelete && (
            <Button
              variant='destructive'
              onClick={() => setDeleteConfirmationOpen(true)}
              disabled={props.deleting}
            >
              <Trash2 data-icon='inline-start' />
              {t('Delete Provider')}
            </Button>
          )}
          <Button
            onClick={saveProvider}
            disabled={!formValid || props.saving || props.deleting}
          >
            {t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
      <ConfirmDialog
        open={syncConfirmationOpen}
        onOpenChange={setSyncConfirmationOpen}
        title={t('Synchronize all accounts?')}
        desc={t(
          'This will synchronize all {{count}} accounts for {{provider}}.',
          {
            count: props.provider?.accounts.length ?? 0,
            provider: props.provider?.name ?? '',
          }
        )}
        confirmText={t('Sync all accounts')}
        handleConfirm={confirmSync}
      />
      <ConfirmDialog
        open={accountSyncTarget !== null}
        onOpenChange={(open) => {
          if (!open) setAccountSyncTarget(null)
        }}
        title={t('Synchronize this account?')}
        desc={t('This will synchronize account {{account}}.', {
          account: accountSyncTarget?.name ?? '',
        })}
        confirmText={t('Sync account')}
        handleConfirm={confirmAccountSync}
      />
      <ConfirmDialog
        open={deleteConfirmationOpen}
        onOpenChange={setDeleteConfirmationOpen}
        title={t('Delete Provider')}
        desc={t('This action cannot be undone.')}
        confirmText={t('Delete')}
        destructive
        isLoading={props.deleting}
        handleConfirm={confirmDelete}
      />
    </Sheet>
  )
}
