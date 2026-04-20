# SuccessRateSelector

## 功能概述

SuccessRateSelector 是一套基于真实请求结果的渠道择优机制。

它会在同一 `group + model` 下，对当前可用渠道按照近期成功/失败表现进行动态打分，并优先选择分数更高的渠道。当某个渠道失败后，系统会自动切换到同组内其他未尝试过的渠道，直到当前优先级层的所有候选都试完，再自动进入下一优先级层，最终遍历完整个分组的所有可用渠道。

对于临时熔断，当前支持显式配置熔断范围：

- `circuit_scope = model`：按 `group + model + channel_id` 熔断
- `circuit_scope = channel`：某个 model 命中熔断后，整个渠道在该 `group` 下都会被临时摘除

适合以下场景：

- 同一个模型下有多个渠道，希望系统自动偏向更稳定的渠道
- 不想额外发送探测请求，不希望增加额外 Token 成本
- 希望失败后能自动试完同组所有可用渠道，而不是固定重试几次就放弃

## 它解决了什么问题

默认的随机选路在多个同层渠道都可用时，无法区分”最近明显更稳定”的渠道。

SuccessRateSelector 会基于真实业务请求的结果，持续学习每个渠道在不同 `group + model` 组合下的表现：

- 最近成功更多的渠道，会逐步获得更多流量
- 最近失败更多的渠道，会逐步降权
- 失败后自动切换到同组其他渠道，直到试完所有可用候选
- 已恢复的渠道，仍然有机会通过探索流量重新回到主选链路

这意味着它更像”智能流量调度 + 自动故障切换”，而不是”硬性禁用器”。

## 工作原理

### 1. 选择与自动切换

当 `channel_success_rate_setting.enabled = true` 时，系统会按以下策略选择和切换渠道：

**首次选择：**
- 在当前 `group + model` 的最高优先级层内，对所有可用渠道打分
- 选出当前分数最高的渠道

**失败后自动切换：**
- 记录已失败渠道到本次请求上下文
- 在当前优先级层内，排除已失败渠道，继续选择剩余候选中分数最高的
- 当前优先级层所有渠道都已尝试 → 自动进入下一优先级层
- 继续在下一层内按分数择优，排除已失败渠道
- 直到当前分组所有优先级层的所有可用渠道都试完

**调用链路：**

1. `controller/relay.go` 进入主循环，请求选择渠道
2. `service/channel_select.go` 调用 `selectChannelWithSuccessRate(...)`
3. `service/channel_success_rate.go` 在当前优先级层候选集合内打分，排除本次请求已失败渠道，选出当前最优渠道
4. `controller/relay.go` 拿到该渠道后，真正执行一次 relay 请求
5. 请求完成后，`controller/relay.go` 调用 `ObserveChannelRequestResult(...)`，把成功或失败结果回写给 SuccessRateSelector
6. 如果失败且允许继续，主流程再次进入渠道选择；SuccessRateSelector 会跳过已失败渠道，优先选择当前层剩余候选；若当前层已全部尝试，则自动进入下一优先级层

**职责划分：**

SuccessRateSelector 负责：
- 在当前候选集合内决定”这一轮该选谁”
- 根据真实结果更新各渠道分数
- 跳过本次请求已经失败过的渠道
- 判断当前优先级层是否已全部尝试，自动进入下一层

relay 主流程负责：
- 真正发起请求
- 接收成功或失败结果
- 记录本次请求已尝试过的渠道
- 判断是否继续下一轮尝试（如遇到不可重试错误则终止）

**实际行为示例：**

假设分组1有以下渠道：
```
渠道a，优先级 10
渠道b，优先级 10
渠道c，优先级 5
渠道d，优先级 0
```

某次请求的实际执行流程：
1. 首次选择：在优先级 10 层（a, b）中择优，假设选中 a
2. a 失败：在优先级 10 层剩余候选（b）中继续选择，选中 b
3. b 失败：优先级 10 层已全部尝试，自动进入优先级 5 层，选中 c
4. c 失败：优先级 5 层已全部尝试，自动进入优先级 0 层，选中 d
5. d 失败：当前分组所有渠道都已尝试，返回最终错误

这意味着：
- 不需要手动配置”重试几次”，系统会自动试完所有可用渠道
- 优先级高的渠道会优先被尝试
- 同一优先级内，分数高的渠道会优先被选择
- 失败后不会在同一渠道重复尝试，而是自动切换到其他候选

### 2. 观测阶段

每次请求完成后，系统都会把实际结果回写到选择器：

- 成功请求：增加成功样本
- 失败请求：增加失败样本
- 同时也会写入渠道监控统计，便于在后台观察趋势

它使用的是**真实线上请求结果**，不需要额外健康检查流量。

### 3. 打分方式

选择器的核心分数由以下几部分组成：

#### 时间衰减

旧样本会按照半衰期持续衰减，最近的结果影响更大。

- 半衰期越短：对波动更敏感，切换更快
- 半衰期越长：更稳定，但对故障恢复和恶化反应更慢

#### 基础成功率分数

基础分采用 Laplace 平滑：

```text
score = (success + 1) / (success + failure + 2)
```

这样做有两个好处：

- 新渠道冷启动时不会直接是 0 分或 1 分
- 没有样本时默认是中性分 `0.5`

#### 快速降权

如果 `quick_downgrade = true`，失败一次会按更重的失败权重计入。

这会让明显异常的渠道更快失去流量。

#### 连续失败惩罚

当连续失败次数达到 `consecutive_fail_threshold` 后，系统会对该渠道施加额外惩罚。

这对于“短时间连续炸掉”的渠道尤其有效，通常比单纯看长期平均成功率更快。

### 4. 探索机制

当 `explore_rate > 0` 时，系统会按一定概率忽略当前分数，随机从当前候选集合里选一个渠道。

作用是：

- 避免流量长期只压在单个高分渠道上
- 给已经恢复的低分渠道重新获得样本的机会
- 避免某个渠道因为历史坏样本过多而长期无法翻身

注意：当前实现是**在当前候选集合中随机选一个渠道**，并不是只挑“最低分渠道”。

### 5. 并列分数处理

如果多个渠道分数相同，系统会用轮询方式在这些并列候选之间分配请求，而不是固定命中其中一个。

## 重要行为与边界

### 1. 优先级决定尝试顺序

SuccessRateSelector 会严格遵守优先级顺序：

- 优先级高的渠道会优先被尝试
- 只有当前优先级层的所有渠道都已尝试后，才会进入下一优先级层
- 同一优先级层内，按成功率分数择优

因此，**优先级仍然决定大方向，SuccessRateSelector 负责同层内的精细调度和跨层自动切换**。

### 2. 它本身不直接修改渠道持久状态

SuccessRateSelector 的职责是“调整选择偏好”“自动切换”和“运行时临时熔断”，不会把渠道真实写成禁用状态。

当前实现中：

- 命中 `immediate_disable` 或连续失败阈值时，只会在 **SuccessRateSelector 内部** 把当前 `group + model + channel` 或整个渠道（取决于 `circuit_scope`）标记为临时熔断
- 临时熔断期间，该渠道只会从当前选择器候选列表里被排除
- 到 `recovery_check_interval` 后，不会直接恢复，而是进入半开探测阶段
- 半开阶段只放行少量探测请求；连续成功达到 `half_open_success_threshold` 后才恢复正常
- 半开探测任一请求失败，会重新进入下一轮临时熔断
- 渠道原本的 `enabled / manually disabled / auto disabled` 持久状态不会被 SuccessRateSelector 改写

这意味着：

- 手动禁用、传统持久化禁用机制仍然是独立机制
- SuccessRateSelector 的熔断是运行时行为，不依赖数据库状态闭环恢复
- 恢复后不需要等“真实恢复启用”，只要熔断窗口到期就会重新参与选路

### 3. 不需要配置”重试次数”

启用 SuccessRateSelector 后，系统会自动试完当前分组的所有可用渠道，不需要手动配置”失败后重试几次”。

原有的 `RetryTimes` 配置在 SuccessRateSelector 启用后主要影响：
- 是否允许跨分组重试（配合 `cross_group_retry` 使用）
- 某些特殊错误的重试判断

但对于同一分组内的渠道切换，SuccessRateSelector 会自动处理，无需依赖固定的重试次数配置。

### 4. 状态是进程内存态

当前选择器的学习状态保存在进程内存里，不会持久化到数据库。

这意味着：

- 服务重启后，会重新从中性分开始学习
- 刚启动时，各渠道默认分数接近 `0.5`
- 发布或重启后，短时间内的选路表现更接近”冷启动状态”

### 5. 渠道亲和命中时，亲和优先

如果你同时启用了 `channel_affinity_setting`，并且请求命中了渠道亲和缓存，那么会优先使用亲和命中的渠道。

SuccessRateSelector 在这种情况下主要承担：

- 亲和未命中时的回退选择器
- 同组同模型下的默认择优器

如果你的业务非常依赖会话粘性、用户粘性或上下文绑定，建议把渠道亲和作为第一层，SuccessRateSelector 作为第二层。

**页面设置步骤（是否失败后继续重试）：**

1. 打开后台 **设置** 页面
2. 进入 **模型相关设置**
3. 找到 **渠道亲和性** 卡片
4. 在规则列表中找到要修改的亲和规则，点击 **编辑**
5. 在规则弹窗中找到开关 **失败后不重试**
   - 开启：命中该亲和规则后，若本次请求失败，不再切换其它渠道重试
   - 关闭：命中该亲和规则后，若本次请求失败，允许继续回退到后续渠道选择流程

如果你希望渠道亲和只负责“优先命中”，而失败后仍交给后续渠道选择器兜底，那么应当把这个开关关闭。

## 配置项说明

SuccessRateSelector 使用统一配置前缀：`channel_success_rate_setting.*`

建议按下面三个层次理解配置，而不是把它当成一整张参数表去记。

### 1. 基础择优参数

- `channel_success_rate_setting.enabled`
  - 类型：`bool`
  - 默认值：`false`
  - 说明：是否启用 SuccessRateSelector
  - 调优建议：建议显式开启

- `channel_success_rate_setting.half_life_seconds`
  - 类型：`int`
  - 默认值：`1800`
  - 说明：成功/失败样本半衰期，单位秒
  - 调优建议：大流量可调低，小流量可调高

- `channel_success_rate_setting.explore_rate`
  - 类型：`float`
  - 默认值：`0.02`
  - 说明：探索概率，范围会被限制在 `0 ~ 1`
  - 调优建议：常用范围 `0.01 ~ 0.05`

- `channel_success_rate_setting.quick_downgrade`
  - 类型：`bool`
  - 默认值：`true`
  - 说明：失败时是否更快降权
  - 调优建议：建议开启

- `channel_success_rate_setting.consecutive_fail_threshold`
  - 类型：`int`
  - 默认值：`3`
  - 说明：连续失败达到阈值后施加额外惩罚
  - 调优建议：常用范围 `2 ~ 5`

### 2. 进阶策略参数

- `channel_success_rate_setting.priority_weights`
  - 类型：`JSON object`
  - 默认值：`{"10":0.2,"5":0,"0":-0.1}`
  - 说明：按 priority 调整同一候选集合内的分数权重
  - 调优建议：仅在当前候选集中存在不同 priority 时才有明显效果

- `channel_success_rate_setting.immediate_disable`
  - 类型：`JSON object`
  - 默认值：`{"enabled":true,"status_codes":[401,403,429],"error_codes":[],"error_types":[]}`
  - 说明：命中状态码、错误码或错误类型后立即触发禁用回调
  - 调优建议：建议保留默认状态码，再按上游特征补充

### 3. 临时熔断与恢复参数

- `channel_success_rate_setting.health_manager`
  - 类型：`JSON object`
  - 默认值：`{"circuit_scope":"model","disable_threshold":0.2,"enable_threshold":0.7,"min_sample_size":10,"recovery_check_interval":600,"half_open_success_threshold":2}`
  - 说明：控制 SuccessRateSelector 运行时临时熔断范围，以及基础间隔 + 半开探测恢复策略
  - 调优建议：建议先用默认值，再按流量规模调整

补充说明：

- `half_life_seconds <= 0` 时，会回退到默认值 `1800`
- `explore_rate < 0` 时会按 `0` 处理，`> 1` 时会按 `1` 处理
- `consecutive_fail_threshold <= 0` 时，会回退到默认值 `3`
- `priority_weights` 会乘到当前候选渠道的得分上，但当前系统仍以 retry 分层选路为主，不会跨层抢流量
- `immediate_disable` 用来触发**运行时临时熔断**，不是写数据库禁用状态
- `health_manager.disable_threshold` 与 `min_sample_size` 决定何时触发连续失败后的临时熔断
- `health_manager.circuit_scope` 决定熔断作用在单个 `group + model + channel`，还是扩大到整个渠道
- `health_manager.recovery_check_interval` 是基础恢复间隔；到期后会切到半开探测，而不是直接恢复
- `health_manager.half_open_success_threshold` 决定半开阶段连续成功多少次才恢复正常
- `health_manager.enable_threshold` 目前主要保留配置兼容语义；恢复主流程依赖基础间隔 + 半开探测，而不是数据库自动启用

## 页面配置

当前管理端已提供独立设置卡片，可直接在**渠道监控 → 渠道设置**页面中修改：

- 基础参数使用表单输入
- `priority_weights`、`immediate_disable`、`health_manager` 使用 JSONEditor 编辑
- 页面保存后会写入对应 option 键，并参与热更新

推荐在页面中直接使用以下三组模板。

下方示例使用 `jsonc` 展示注释，便于理解每个字段的作用；如果你要复制到页面里的 JSONEditor，请删除注释后再保存。

## 推荐配置

下面给出三个更实用的推荐档位。

### 1. 通用推荐（大多数场景直接从这里开始）

```jsonc
{
  // 总开关：开启后才会进入成功率择优逻辑
  "enabled": true,
  // 半衰期：越小越关注最近样本，越大越平滑
  "half_life_seconds": 1800,
  // 探索率：保留少量随机探测，避免长期只压单一路线
  "explore_rate": 0.02,
  // 失败时加快降权，适合大多数线上场景
  "quick_downgrade": true,
  // 连续失败达到 3 次后追加额外惩罚
  "consecutive_fail_threshold": 3,
  "priority_weights": {
    // 同一候选集合内，高优先级可额外加分
    "10": 0.2,
    // 中间优先级保持中性
    "5": 0,
    // 低优先级轻微减分
    "0": -0.1
  },
  "immediate_disable": {
    // 命中下面规则时立即触发禁用回调
    "enabled": true,
    // 常见硬失败状态码：鉴权失败、权限不足、限流
    "status_codes": [401, 403, 429],
    // 可按上游错误码补充，例如 insufficient_quota、rate_limit_exceeded
    "error_codes": [],
    // 可按上游错误类型补充，例如 billing_error
    "error_types": []
  },
  "health_manager": {
    // 熔断粒度：model 为单模型熔断；channel 为命中后整条渠道一起熔断
    "circuit_scope": "model",
    // 成功率低于 0.2 且样本足够时，可触发临时熔断
    "disable_threshold": 0.2,
    // 该阈值主要保留配置兼容语义
    "enable_threshold": 0.7,
    // 至少积累 3 个样本再判断是否需要临时熔断
    "min_sample_size": 3,
    // 每 600 秒进入一次恢复检查窗口
    "recovery_check_interval": 600,
    // 半开阶段连续成功 2 次后恢复正常
    "half_open_success_threshold": 2
  }
}
```

适用：

- 同模型下有 2 到 5 个主力渠道
- 流量中等或较高
- 希望既能快速识别故障，又不要切换得过于激进

这是当前代码默认值对应的配置，也是最稳妥的起点。

### 2. 高流量、波动明显的场景

```jsonc
{
  // 开启成功率择优
  "enabled": true,
  // 半衰期缩短，让系统更快感知最近波动
  "half_life_seconds": 900,
  // 提高一点探索率，便于高流量下更快重采样
  "explore_rate": 0.03,
  // 高流量场景建议保留快速降权
  "quick_downgrade": true,
  // 连续 2 次失败就开始加重惩罚
  "consecutive_fail_threshold": 2,
  "priority_weights": {
    "10": 0.2,
    "5": 0,
    "0": -0.1
  },
  "immediate_disable": {
    "enabled": true,
    "status_codes": [401, 403, 429],
    // 对高流量场景，建议把常见限流错误码也纳入立即禁用
    "error_codes": ["rate_limit_exceeded"],
    "error_types": []
  },
  "health_manager": {
    // 高流量场景通常仍建议按单模型熔断，避免把健康模型一起摘掉
    "circuit_scope": "model",
    // 保持较严格的临时熔断阈值
    "disable_threshold": 0.2,
    // 该阈值主要用于兼容配置语义
    "enable_threshold": 0.75,
    // 至少积累 3 个样本再判断是否需要临时熔断
    "min_sample_size": 3,
    // 更快进入半开探测
    "recovery_check_interval": 300,
    // 半开连续成功 2 次后恢复正常
    "half_open_success_threshold": 2
  }
}
```

适用：

- 请求量大，样本积累很快
- 上游质量波动明显，希望更快切走异常渠道
- 同一模型有多条可替代主线路

特点：

- 对最近失败更敏感
- 更快把流量让给表现更好的渠道
- 但切换频率会更高

### 3. 低流量、稳定优先的场景

```jsonc
{
  // 开启成功率择优
  "enabled": true,
  // 半衰期拉长，避免少量样本就导致剧烈切换
  "half_life_seconds": 3600,
  // 低流量时探索率建议更低
  "explore_rate": 0.01,
  // 仍然保留快速降权，但整体会因为更长半衰期而更平滑
  "quick_downgrade": true,
  // 连续失败阈值调高，降低偶发失败的影响
  "consecutive_fail_threshold": 5,
  "priority_weights": {
    "10": 0.2,
    "5": 0,
    "0": -0.1
  },
  "immediate_disable": {
    "enabled": true,
    "status_codes": [401, 403, 429],
    "error_codes": [],
    "error_types": []
  },
  "health_manager": {
    // 低流量也建议先按单模型熔断，降低误伤范围
    "circuit_scope": "model",
    // 低流量下可略放宽临时熔断阈值
    "disable_threshold": 0.15,
    // 该阈值主要用于兼容配置语义
    "enable_threshold": 0.75,
    // 至少积累 3 个样本再判断是否需要临时熔断
    "min_sample_size": 3,
    // 半开探测频率放慢，减少频繁开关
    "recovery_check_interval": 900,
    // 半开连续成功 2 次后恢复正常
    "half_open_success_threshold": 2
  }
}
```

适用：

- 请求量较小，样本增长慢
- 不希望因为少量偶发失败就频繁切换
- 渠道整体质量较稳定，只想做轻度择优

特点：

- 对偶发抖动更宽容
- 选择更平滑
- 恢复和恶化感知速度都会更慢

## 最佳实践建议

### 1. 让“应该彼此竞争”的渠道处于同一优先级

如果两个渠道本质上都是同类主力渠道，建议放在同一优先级，让 SuccessRateSelector 在同层内动态比较。

如果某个渠道是冷备、贵价、低额度或仅用于兜底，建议放到更低优先级，而不是指望 SuccessRateSelector 自动少用它。

### 2. 小流量场景不要把参数调得太激进

在小流量场景下：

- `half_life_seconds` 过小，会让少量样本就引起明显偏转
- `explore_rate` 过高，会让随机探测流量占比过大
- `consecutive_fail_threshold` 过低，会让偶发失败被放大

如果你的请求量不高，建议优先保持稳定，先用保守配置。

### 3. 一般不建议把 `explore_rate` 设为 `0`

如果完全没有探索流量，历史上被打低分的渠道重新积累样本会更慢。

除非你明确希望系统只压在当前高分渠道上，否则建议至少保留少量探索，例如 `0.01` 或 `0.02`。

### 4. 推荐配合其它异常摘除机制使用

更合理的职责分工通常是：

- 其它异常摘除机制：处理明显坏掉的渠道
- SuccessRateSelector：处理“都还能用，但最近谁更稳定”的问题

这样能同时兼顾硬故障摘除和软性流量优化。

### 6. 熔断范围建议

- 如果同一渠道下不同 model 的故障通常彼此独立，建议使用 `circuit_scope = model`
- 如果某个渠道一旦出现鉴权、额度、区域或上游整体故障，往往会影响该渠道下所有 model，建议使用 `circuit_scope = channel`
- `channel` 级熔断更稳，但会把原本健康的 model 一并摘除，适合“渠道级故障比模型级故障更常见”的场景


启用后可以通过后台的渠道监控页面观察整体变化。

可重点关注：

- 整体成功率是否提升
- 同模型多渠道之间的请求分布是否更合理
- 某条渠道异常时，流量是否能自然转移到其他可用渠道
- 渠道恢复后，是否能重新获得请求

相关页面与接口：

- 管理端页面：`/console/channel-monitor`
- 汇总接口：`/api/channel_monitor/summary`
- 趋势接口：`/api/channel_monitor/health`
- 渠道分页接口：`/api/channel_monitor/channels`

## 一个简单的落地建议

如果你是第一次启用 SuccessRateSelector，建议按下面的方式落地：

1. 先只在有多个同层渠道的模型/分组上启用
2. 先使用“通用推荐”配置
3. 通过渠道监控观察成功率、失败数和流量分布变化
4. 再根据你的实际流量特征，选择偏敏捷或偏稳定的参数档位

这样通常更容易得到稳定、可解释的效果。