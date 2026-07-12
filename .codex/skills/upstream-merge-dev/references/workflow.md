# Upstream Merge 操作参考（newapi）

将官方上游 `QuantumNous/new-api` 的 tag/HEAD 安全合并进 `dev`，保护 fork 自定义功能。功能清单见 [fork-features.md](fork-features.md)。

## 目录

1. [检查状态](#1-检查状态)
2. [确认 upstream remote](#2-确认-upstream-remote)
3. [拉取上游](#3-拉取上游)
4. [确定合并目标](#4-确定合并目标)
5. [执行合并](#5-执行合并)
6. [冲突处理与建议](#6-冲突处理与建议)
6.5 [语义冲突审查（必须执行）](#65-语义冲突审查必须执行)
7. [合并后验证](#7-合并后验证保护-dev-功能的关键)
8. [成功报告格式](#8-成功报告格式)
9. [逐文件审查合并（推荐策略）](#9-逐文件审查合并推荐策略)

## 1. 检查状态

```bash
git status --short
git branch --show-current
git remote -v
```

工作区不干净时先暂停，提示用户处理未提交变更。

## 2. 确认 upstream remote

```bash
git remote get-url upstream    # 应为 https://github.com/QuantumNous/new-api
# 无则新增（提示用户确认）
git remote add upstream https://github.com/QuantumNous/new-api
```

## 3. 拉取上游

```bash
git fetch upstream --tags
# 或只拉指定版本
git fetch upstream tag vX.Y.Z
```

## 4. 确定合并目标

### 情况 A：有新 tag

```bash
git tag --sort=-version:refname | head -30
```

展示最近 20-50 个 tag，等用户选择。

### 情况 B：无新 tag（仅 upstream/main 前进）

```bash
git rev-list --count dev..upstream/main   # dev 落后多少
git log upstream/main --oneline -20
```

不自动假定，等用户确认以 `upstream/main` HEAD 为目标。

## 5. 执行合并

```bash
TARGET=vX.Y.Z                        # 或 upstream/main
BASE=$(git merge-base dev $TARGET)   # 共同祖先，三方对比基准
git checkout dev                     # 须先确认切换
git log $BASE..dev --oneline         # 确认 dev 自定义提交，心中有数
git merge $TARGET --no-ff --no-edit  # 保留 merge commit
```

## 6. 冲突处理与建议

```bash
git diff --name-only --diff-filter=U   # 列冲突文件
```

立即停止，对每个冲突文件给建议，按归属区分：
- **纯上游文件**（fork 未改）→ 通常取上游：`git checkout --theirs <file>`。
- **fork 专有文件**（转换器、渠道监控、billingexpr 等）→ 优先保留 dev，再手工并入上游必要变更。
- **双方都改的文件** → 逐段人工合并，保 fork 功能前提下吸收上游修复。
- **锁文件/生成物**（`go.sum`、前端 `bun.lockb`）→ 不手工解，重新生成后 `git add`：`go mod tidy`；`cd web && bun install`。
- 上游重命名/删除符号而 fork 仍引用旧符号 → 标记为语义冲突（文本可合但需同步改 fork 引用）。

解决后 `git add <files>` → `git commit --no-edit`。放弃：`git merge --abort`（合并中）/ `git reset --hard ORIG_HEAD`（已完成未推送）。

## 6.5 语义冲突审查（必须执行）

⚠️ Git 只检测文本冲突。文本冲突解决后**不要直接提交**，先审查语义冲突，否则可能静默破坏 fork 功能。

### 语义冲突类型

| 类型 | newapi 示例 |
|------|-------------|
| 接口签名变更 | 上游改渠道适配器/relay helper 返回类型，fork 调用方用旧方式 |
| 数据格式变更 | 上游改渠道设置 DTO / usage 结构，fork 计费或分流代码按旧格式解析 |
| 依赖/注册变更 | 上游重构渠道选择、路由或适配器注册，覆盖 fork 的 SuccessRateSelector/亲和性/codex 分流 |
| 行为假设变更 | 上游改 usage 计费口径，fork 分层计费结算基于旧假设 |

### 审查流程

```bash
# 上游在冲突区域改了什么
git diff $BASE..$TARGET -- <file>
# fork 在冲突区域改了什么
git diff $BASE..dev -- <file>
# 冲突函数/类型的所有调用点
grep -rn "<FuncOrType>" --include="*.go"
```

关键问题：上游改动目的（bug/重构/新功能）？fork 改动目的（自定义逻辑/集成）？两者能否共存？

### newapi 重点检查项

- [ ] **渠道选择**：`service/channel_select.go` 是否保留 SuccessRateSelector（#6）、渠道亲和性（#8）集成，未被上游选路逻辑覆盖。
- [ ] **计费主流程**：`relay/helper/price.go`、`service/log_info_generate.go`、usage 解析是否仍正确调用 billingexpr（#1/#2）注入点。
- [ ] **relay 入口**：`relay/claude_handler.go` 的第三方 codex 分流（#7）、转换器调用（#3/#4）是否完整。
- [ ] **渠道适配器注册**：`relay/channel/*/adaptor.go`、`relay/relay_adaptor.go` 的 codex 适配器是否仍注册。
- [ ] **DB 兼容**：改动的 GORM/原生 SQL 是否保持三库兼容（Rule 2）；迁移 SQLite 用 ADD COLUMN。
- [ ] **JSON**：新代码 marshal/unmarshal 走 `common.*`（Rule 1）。
- [ ] **调用点/测试**：签名或行为变更后，所有调用点与相关测试已同步。

### 决策原则

1. 安全/性能修复 > 功能适配 > 代码风格。
2. 优先保留上游变更，除非与 fork 功能确实不兼容。
3. 不兼容时优先调整 fork 代码而非丢弃上游改动；必须丢弃时在代码注释原因。

## 7. 合并后验证（保护 dev 功能的关键）

文本无冲突不代表 fork 功能正常。合并后必须两步验证。

### 7.1 构建验证

```bash
go build ./...
go vet ./...                     # 可选，更全
cd web && bun install && bun run build && cd ..   # 前端如有上游变更
```

构建失败优先怀疑 fork 改动与上游新代码的语义冲突，定位修复，不直接回退上游更新。

### 7.2 Fork 功能测试验证

基于 [fork-features.md](fork-features.md) 按风险等级验证。优先跑相关包单测 + 构建冒烟：

```bash
# 高风险
go test ./pkg/billingexpr/ ./service/ -run 'Tiered|Settle' -count=1     # 分层计费 #1/#2
go test ./relay/channel/openai/ ./relay/channel/codex/ -count=1         # 转换器 #3/#4
# 中风险
go test ./service/ -run 'SuccessRate|Affinity' -count=1                 # 择优/亲和 #6/#8
go test ./model/ -run 'ChannelMonitor' -count=1                         # 渠道监控 #5
# 全量（时间允许）
go test ./... -count=1
```

无自动化测试覆盖的功能（前端页面、运行时行为）→ 列出待人工验证清单交用户，不假装已验证。

### 失败排查方向（按优先级）

1. **注册/配置丢失**（最常见）：渠道适配器注册、路由注册（`router/`）、定时任务、配置项被上游覆盖。
   ```bash
   grep -rn "codex" relay/relay_adaptor.go relay/channel/codex/adaptor.go
   grep -rn "SuccessRate\|Affinity" service/channel_select.go
   ```
2. **函数签名/行为变更未适配**：调用方用旧签名、返回值解构不匹配、usage 口径变化。
3. **数据格式/DB**：DTO/序列化格式与代码期望不一致、三库兼容被破坏、迁移未适配。
4. **依赖/模块变更**：上游删除/重命名 fork 依赖的包或符号。

## 8. 成功报告格式

```
合并完成：<目标 tag 或 upstream/main short-sha> → dev（--no-ff）
新增提交：X 个
dev 自定义提交：保留 Y 个
冲突：无 / 已解决（列文件）
构建验证：go build 通过 / 前端 bun run build 通过

Fork 功能验证（对照 fork-features.md）：
- ✅ 分层计费 #1/#2 - 测试通过
- ✅ 转换器 #3/#4 - 测试通过
- ⚠️ 渠道监控页面 #5 - 需人工验证（前端）
测试结论：全部通过 / 部分待人工验证 / 有失败需修复

后续步骤：
- 全部通过 → 可执行 git push origin dev
- 待人工验证 → 用户确认后再推送
- 有失败 → 修复后重新测试
```

## 9. 逐文件审查合并（推荐策略）

⚠️ Git 自动合并盲点：只看文本相似度，不懂逻辑；"无冲突"≠语义兼容；fork 改动可能被上游变更**静默覆盖**（如渠道选择集成、适配器注册丢失）。

**何时用**：fork 改动过的文件（尤其双方都改）、关键基础设施（`relay/` 主流程、`channel_select.go`、路由、DB 模型、计费）、上游变更大时。
**何时可用 Git 自动合并**：fork 未改的文件、上游新增文件、纯文档/注释/测试数据。

### 9.1 识别 fork 改动文件

```bash
BASE=$(git merge-base dev $TARGET)
git diff $BASE..dev --name-only | sort -u > /tmp/fork_files.txt
git diff $BASE..$TARGET --name-only | sort -u > /tmp/upstream_files.txt
echo "=== 双方都改（高风险）==="
comm -12 /tmp/fork_files.txt /tmp/upstream_files.txt
```

### 9.2 逐文件三方对比与决策

```bash
FILE=service/channel_select.go
git diff $BASE..dev    -- $FILE   # fork 改了什么
git diff $BASE..$TARGET -- $FILE   # 上游改了什么
```

| 场景 | Fork | 上游 | 策略 |
|------|------|------|------|
| A | 新增函数/注册 | 同文件他处改动 | 手动合并：保留 fork 新增 + 应用上游改动 |
| B | 改函数逻辑 | 同函数签名变更 | 手动重写：用上游新签名 + 迁移 fork 逻辑 |
| C | 配置/注册项 | 他配置项 | 手动合并：保留 fork 配置 + 加上游新项 |
| D | fork 专有文件 | 无此文件 | 直接保留 fork 版本 |
| E | 未改动 | 有改动 | 直接采用上游版本 |

### 9.3 手动合并（推荐：基于上游版本改）

```bash
git checkout $TARGET -- $FILE        # 取上游版本
# 依据 git diff $BASE..dev -- $FILE 手动重新应用 fork 改动
git diff dev -- $FILE                # 确认 fork 功能没丢
go build ./...                       # 编译验证
```

### 9.4 检查清单（每个文件）

- [ ] 三方对比完成（fork diff + upstream diff）
- [ ] 改动意图分析（目的、区域、兼容性）
- [ ] 合并策略确定（A/B/C/D/E）
- [ ] 手动合并执行
- [ ] fork 功能验证（搜关键符号/注册点、跑测试）
- [ ] 上游关键修复已引入（`git log $BASE..$TARGET -- $FILE | grep -iE "fix|security"`）
- [ ] 编译通过

## 注意事项

- 不 `git push --force`；`git reset --hard` 仅回退本地未推送的失败合并，执行前确认。
- 不在用户未确认时切换分支。
- 用户只要求 fetch/查看 tag 时，不触发合并。
- 转换器（#3/#4）功能更新不走本流程，用 cliproxyapi-translator-sync 技能。
