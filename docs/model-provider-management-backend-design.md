# 模型供应商管理设计

## 核心关系

模型供应商管理以供应商账户下的“成本归集单元”为成本和利润单元。不同上游协议由同步适配器归一化：NewAPI 的归集单元是上游分组，Sub2API 的归集单元是单个 API Key。

- 上游成本通过 `/api/log/self/stat` 的 `group` 参数查询。
- 一个上游分组可以映射多个本地渠道。
- 一个本地渠道只能归属一个上游分组，避免收入重复计算。
- 本地收入为该上游分组全部映射渠道的消费日志 `quota` 总和。
- Key 只保存稳定上游 ID、当前名称、掩码、状态以及所属分组。
- Key 改名或换组时按稳定上游 ID 更新当前记录，不保存分组历史。

## 最新数据结构

当前功能未部署，不提供旧字段或旧表迁移逻辑，直接使用以下最新结构。

### 供应商与账户

`xnewapi_upstream_providers` 保存供应商地址、同步周期、适配器所需的换算参数和同步状态。NewAPI 使用 Quota 换算基数和充值比例还原金额；Sub2API 返回的余额、累计充值和 `actual_cost` 已是金额，不使用 Quota 换算基数。

`xnewapi_upstream_provider_accounts` 保存上游账户标识、适配器所需的登录用户名/同步密钥、余额、累计充值及账户同步状态。登录密码和同步密钥不会通过 API 返回。

### 上游分组

`xnewapi_upstream_provider_groups` 保存：

- `account_id`：所属供应商账户。
- `name`：上游分组名称。
- 唯一约束：`account_id + name`。

`xnewapi_upstream_provider_group_channels` 保存分组与本地渠道的一对多关系：

- `provider_group_id`：供应商分组 ID。
- `channel_id`：本地渠道 ID。
- 同一分组和渠道不能重复关联。
- `channel_id` 全局唯一，防止同一渠道收入归属多个供应商分组。

### Key

`xnewapi_upstream_provider_keys` 保存：

- `account_id`：所属供应商账户。
- `provider_group_id`：当前所属上游分组。
- `external_id`：上游稳定 Key ID，是同步更新依据。
- `name`：上游当前 Key 名称。
- `key_masked` / `key_fingerprint`：展示和去重信息。
- `status` / `last_usage_at`：当前状态和最近使用时间。

Key 不保存渠道映射。Key 名称也不是成本查询条件。
`account_id + external_id` 唯一。Key 由上游同步独占写入，workspace 保存和页面编辑不会手工创建或修改 Key。

### 分组日利润

`xnewapi_upstream_provider_group_profit_daily` 以 `date + provider_group_id` 唯一，保存：

- `provider_usage_quota`：NewAPI 上游分组当天累计 quota；Sub2API 无 quota 语义，固定保存为 0。
- `provider_cost`：按供应商换算参数得到的分组成本。
- `revenue_quota`：该分组全部映射渠道的消费 quota。
- `revenue_amount`：按系统 quota 基数换算的收入。
- `cost_status` / `cost_observed_at`：成本可用状态和观测时间。

利润和毛利率查询时计算。成本未知时返回 `null`，不能按零成本计算。

## 同步与归集

1. 拉取账户余额和累计充值。
2. 拉取 `/api/token/?p=1&page_size=100`，并按稳定 ID 补查已保存但未出现在列表中的 Key。
3. 按上游分组名创建或更新供应商分组。
4. 按稳定 Key ID 更新名称、掩码、状态和当前所属分组。
   当旧分组已经没有其他 Key 且迁移目标唯一时，将旧分组的渠道映射转移到新分组并删除旧空分组。
5. 对去重后的每个非空上游分组请求一次 `/api/log/self/stat`，`token_name` 为空，`group` 为上游分组名。
6. 按 `日期 + 分组 ID` 覆盖当天累计成本，不累加重复同步结果。
7. 查询该分组映射的全部渠道，汇总消费日志并更新分组日收入。

### Sub2API 适配

Sub2API 账户使用独立的 `login_username` 和 `login_password` 配置同步；适配器登录时按参考实现将用户名放入请求的 `email` 字段，不复用上游账户 ID。每次同步只登录一次：

1. `POST /api/v1/auth/login` 获取短期访问令牌，令牌不落库。
2. `GET /api/v1/auth/me?timezone=Asia%2FShanghai` 获取上游账户 ID、余额和累计充值；返回的账户 ID 写入 `external_id`。
3. 分页请求 `GET /api/v1/keys` 获取账户下的 API Key。
4. 对每个 Key 请求 `GET /api/v1/usage/dashboard/snapshot-v2`，使用 `api_key_id` 和当天日期查询。
5. 只汇总 `trend.actual_cost` 作为实付成本；`cost` 是标准计费金额，不参与利润计算。`balance`、`total_recharged` 和 `actual_cost` 均直接按金额保存，不经过 Quota 换算基数或充值比例换算。

Sub2API 的每个 Key 会生成独立归集单元，名称包含稳定 Key ID（例如 `production (#4050)`），因此同名 Key 不会合并。一个归集单元仍可映射多个本地渠道，一个本地渠道仍只能属于一个归集单元，收入与成本的统一规则不变。

同组多个 Key 只查询和保存一份成本。渠道映射变更只重算当天收入并影响未来日期；昨天及更早的分组日利润一旦生成便冻结，重建查询不会按当前映射改写历史归属。

## 页面结构

页面只保留两层：

1. 供应商行：账户数、分组数、余额、累计充值和汇总利润。
2. 分组行：分组名、Key 摘要、多个渠道映射、收入、成本、毛利率和同步时间。

分组下不再展开 Key 子行：

- 分组名可点击复制。
- 单 Key 直接在分组行展示名称、掩码和状态，名称可复制。
- 多 Key 显示数量按钮，通过 Popover 查看并复制 Key。
- Key 不显示余额、累计充值或费用。

编辑页在分组层添加或移除多个渠道。供应商和账户同步属于状态变更操作，调用同步 API 前必须显示确认对话框；同步失败徽标通过 tooltip 展示后端错误。

## 数据库兼容性

所有最新表通过 GORM 建表，必须同时支持 SQLite、MySQL 5.7.8+ 和 PostgreSQL 9.6+。日志库与主库可独立部署，因此日志收入查询和主库分组日表更新分开执行，不进行跨库 join。
