/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: 渠道运营中心主入口 - 统一监控、成本、利润视图

import { Activity } from 'lucide-react'
import { useState } from 'react'

import { SectionPageLayout } from '@/components/layout'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { OperationsCost } from './operations-cost'
import { OperationsMonitoring } from './operations-monitoring'
import { OperationsOverview } from './operations-overview'
import { OperationsProfit } from './operations-profit'

type OperationsTab = 'overview' | 'monitoring' | 'suppliers' | 'profit'

export function ChannelOperations() {
  const [activeTab, setActiveTab] = useState<OperationsTab>('overview')
  const [selectedChannelId, setSelectedChannelId] = useState<number | null>(
    null
  )

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <div className='flex min-w-0 items-center gap-2'>
          <Activity className='text-primary size-5 shrink-0' />
          <span className='truncate'>渠道运营中心</span>
        </div>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <Tabs
          value={activeTab}
          onValueChange={(v) => setActiveTab(v as OperationsTab)}
        >
          <TabsList className='mb-4'>
            <TabsTrigger value='overview'>概览</TabsTrigger>
            <TabsTrigger value='monitoring'>监控视图</TabsTrigger>
            <TabsTrigger value='profit'>利润分析</TabsTrigger>
            <TabsTrigger value='suppliers'>供应商管理</TabsTrigger>
          </TabsList>

          <TabsContent value='overview' className='mt-0'>
            <OperationsOverview
              onViewChannel={(channelId) => {
                setSelectedChannelId(channelId)
                setActiveTab('monitoring')
              }}
            />
          </TabsContent>

          <TabsContent value='monitoring' className='mt-0'>
            <OperationsMonitoring
              highlightChannelId={selectedChannelId}
              onHighlighted={() => setSelectedChannelId(null)}
            />
          </TabsContent>

          <TabsContent value='profit' className='mt-0'>
            <OperationsProfit />
          </TabsContent>

          <TabsContent value='suppliers' className='mt-0'>
            <OperationsCost />
          </TabsContent>
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
