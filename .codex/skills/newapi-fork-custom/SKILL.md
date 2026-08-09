---
name: newapi-fork-custom
description: 维护 x-light-ai/xnewapi 相对官方 QuantumNous/new-api 的 fork 定制并持续缩小上游冲突面。适用于新增或重构 fork 功能、隔离渠道监控与 SuccessRateSelector、收敛 Claude/OpenAI/Codex 转换器接入、补充 FORK-CUSTOM 标记、审计 A/M 差异与上游重叠文件、删除已被上游覆盖的重复实现。触发词：newapi fork custom、最小化自定义修改、缩小 fork 冲突面、FORK-CUSTOM 标记、审计 fork 差异、fork 功能重构。官方上游合并由 upstream-merge-dev 处理，CPA 转换器更新由 cliproxyapi-translator-sync 处理。
---

# Newapi Fork Custom

以上游实现为默认实现。把 fork 行为放入所属层的自有文件，在上游文件中只保留狭窄、带标记的组装入口。

## 边界

- 不执行上游 fetch、tag 选择或 merge；这些操作交给 `upstream-merge-dev`。
- 不从 CLIProxyAPI 搬运转换器更新；这些操作交给 `cliproxyapi-translator-sync`。
- 不修改、删除或替换项目规则保护的 new-api、QuantumNous 相关名称、路径、许可证、品牌或元数据。
- 不改变 `go.mod` 的官方模块路径。
- 不把 fork 缩面提交与上游 merge 混在同一提交。

## 必读上下文

1. 读取仓库根目录 `AGENTS.md`。
2. 读取 [fork 功能清单](../upstream-merge-dev/references/fork-features.md)，以真实 diff 校验清单，不把清单当作唯一证据。
3. 涉及具体缩面方式时读取 [refactor-patterns.md](references/refactor-patterns.md)。
4. 涉及计费表达式时额外读取 `pkg/billingexpr/expr.md`。

## 工作流

1. **建立基线**：确认当前分支和工作区状态，确定目标上游 ref，运行 `scripts/audit-fork-surface.ps1`。未提交改动存在时添加 `-IncludeWorkingTree`，不误用只比较 commit 的范围。
2. **判断归属**：用 merge-base diff、文件引入提交和 upstream 分支包含关系区分 fork、新近上游提交和通用修复。上游已有等价能力时采用上游实现并删除 fork 重复代码。
3. **划分差异**：
   - A 类：fork 新增文件。保持独立，确认来源后补文件级标记。
   - B 类：上游文件中的窄注册、策略选择、字段或调用入口。保留并就近标记。
   - C 类：重写上游控制流、复制策略、删除上游实现或在多个 adaptor/component 中散布替换。先缩面再标记。
4. **选择实现方式**：严格按以下优先级执行：
   1. 在现有分层内新增 fork 自有文件。
   2. 在稳定组装边界显式注册 policy、translator、route、migration 或 lifecycle hook。
   3. 在上游文件末尾追加一个 import、注册或调用入口。
   4. 用集中式可见性策略包裹完整上游 UI 块。
   5. 只有无稳定边界时才修改上游函数，并先把 fork 实现抽离。
5. **保持接口语义**：不得为了减少 diff 吞掉 translator 错误、改变 retry 边界、泄露密钥，或破坏显式 `0`/`false` 的透传。JSON 操作使用 `common.*`，数据库保持 SQLite、MySQL、PostgreSQL 兼容。
   - 完全位于 `web/src/features/xnewapi/` 的 fork 自有 UI 可直接使用中文可见文案，无需 `useTranslation()`、locale key 或翻译文件；不得把 fork 专属文案写入上游 `web/src/i18n/locales/*.json`。修改共享或上游前端文件、组件中的文案时仍遵循项目的完整 i18n 要求。
   - fork 新增表不得写入上游 `AutoMigrate` 模型列表；`model/main.go` 只保留 `migrateForkTables()` 一行调用，且相对上游必须是纯新增、无修改行。手写 SQLite DDL 必须与 GORM 生成形态一致（索引头单空格、列名反引号），详见 [refactor-patterns.md](references/refactor-patterns.md) 第 4 节。
6. **补充标记**：在最终落点补 `FORK-CUSTOM`，不要先给即将移动或删除的代码批量加标记。
7. **更新清单**：功能、接入点或验证命令发生变化时更新现有 fork 功能清单，不创建第二份事实源。
8. **验证**：运行格式检查、受影响包测试、`go build ./...`，前端变更使用 Bun 运行相关 lint/build。改动迁移时对真实库副本连续跑两次迁移，确认第二次为 no-op 且行数不变。最后再次运行审计脚本并执行 `git diff --check`。

## 标记规则

使用英文说明保留原因和边界：

```text
// FORK-CUSTOM: <reason and ownership boundary>
```

- 新增源码文件添加文件级标记；shebang、Go build tags、现有许可证和受保护归属信息必须保留在其合法位置，标记放在这些内容之后。
- 修改上游源码时，在每个语义接入点前就近标记，包括 import、注册、字段、条件、测试适配和发布触发器。
- JSON、lockfile、图片、二进制和生成文件属于标记例外；在最近的源码或配置入口记录用途。
- 标记不能为大段上游重写提供理由。

## 审计

从仓库根目录运行：

```powershell
pwsh .codex/skills/newapi-fork-custom/scripts/audit-fork-surface.ps1 -Target upstream/main -Branch dev
```

在 CI 或希望发现问题时返回非零状态：

```powershell
pwsh .codex/skills/newapi-fork-custom/scripts/audit-fork-surface.ps1 -Target upstream/main -Branch dev -IncludeWorkingTree -FailOnFindings
```

重点检查：修改上游文件数量、与目标上游重叠文件、缺少标记的候选文件、单个上游文件中的 fork 实现规模、可删除的重复实现，以及 lockfile/生成文件是否只由工具产生。

## 提交纪律

- 按 `transport -> selection/lifecycle -> model migration -> shell/i18n -> release overlay` 拆分缩面提交。
- 只提交当前任务相关变更，不手工修改 lockfile 或生成文件来添加标记。
- 未经明确要求不得 merge、push 或修改远端。
