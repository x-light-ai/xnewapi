/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

// FORK-CUSTOM: 运营中心 - 成本视图(复用供应商管理)

import { ProviderManagement } from '../provider-management'

// 目前直接复用原始供应商管理组件
// 后续可以在此基础上增强:内联显示渠道性能等
export function OperationsCost() {
  return <ProviderManagement embedded />
}
