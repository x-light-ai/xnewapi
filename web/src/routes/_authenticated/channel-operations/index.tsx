/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: 渠道运营中心 - 整合监控、成本、利润管理

import { createFileRoute } from '@tanstack/react-router'

import { ChannelOperations } from '@/features/xnewapi/channel-operations'

export const Route = createFileRoute('/_authenticated/channel-operations/')({
  component: ChannelOperations,
})
