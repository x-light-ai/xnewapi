---
name: upstream-merge-dev
description: 将官方上游 QuantumNous/new-api 的指定 tag（或无新 tag 时的 upstream/main）安全合并到本地 fork 的 dev 分支，保护 dev 上的 fork 自定义功能（分层计费 billingexpr、渠道监控、SuccessRateSelector、codex/cpaopenai 转 claude 转换器、渠道亲和性等）不被上游覆盖或产生语义冲突。适用于：从官方上游拉取新版本、同步上游 tag、上游无新 tag 时同步 main、处理 fork 合并冲突、合并后验证 fork 功能完整性。触发词：upstream merge、同步上游、拉取上游、合并上游 tag、fork 同步、new-api 升级。注意与 cliproxyapi-translator-sync 区分：那是从参考仓库 CLIProxyAPI 手工移植转换器（非 git merge）。
---

# Upstream Merge Dev

将官方上游 `QuantumNous/new-api` 的 tag/HEAD 安全合并到本地 `dev`，**核心目标是引入上游更新的同时不破坏 dev 上的 fork 自定义功能**。

## 仓库与分支模型

| 项 | 值 |
|----|----|
| upstream | `https://github.com/QuantumNous/new-api`（官方，合并源） |
| origin | `x-light-ai/xnewapi`（fork 远端） |
| `dev` | 所有 fork 自定义修改 + 合并的最终落点 |
| `main` | 干净同步分支，不在此开发 |

合并源是 upstream 的 tag（优先）或 `upstream/main`（无新 tag 时），**直接合并进 `dev`**。

## 核心原则

- 操作前工作区必须干净（无未提交变更），否则先暂停提示。
- 优先展示 tag 由用户选择；无新 tag 则展示 `upstream/main` 最近提交，由用户确认以 HEAD 为目标，**不自动推断版本**。
- **保护 dev fork 功能**：合并前必读 [references/fork-features.md](references/fork-features.md)，了解要保护什么。
- **逐文件审查优先**：对 fork 改动过的文件，优先手动三方对比而非依赖 Git 自动合并。
- **文本无冲突 ≠ 语义无冲突**：Git 不报冲突也要验证 fork 功能完整性。
- 合并用 `--no-ff` 保留独立 merge commit，便于回溯/回滚。
- 有冲突立即停止，列出冲突文件并给每个文件解决建议，等待用户决策。
- 不使用破坏性 git（`reset --hard` 仅用于回退本地未推送的失败合并，执行前确认）；不 `push --force`。
- 不修改 upstream remote URL、不推送，除非用户明确要求。

## 标准流程

1. **检查状态**：确认在 git 仓库、当前分支、工作区是否干净。
2. **确认 upstream remote**：无则提示确认后 `git remote add upstream https://github.com/QuantumNous/new-api`；有但 URL 不符先征求确认。
3. **拉取上游**：`git fetch upstream --tags`（指定版本可 `git fetch upstream tag vX.Y.Z`）。
4. **确定合并目标**（二选一，展示后等用户选）：
   - 有新 tag：`git tag --sort=-version:refname | head -30` 展示最近 20-50 个。
   - 无新 tag：`git log upstream/main --oneline -20` 展示，等用户确认以 HEAD 为目标。
5. **确定对比基准**：`BASE=$(git merge-base dev <目标>)`，后续三方对比都以 `BASE` 为共同祖先。
6. **了解 fork 功能点**（关键前置）：读 [references/fork-features.md](references/fork-features.md)，据清单心中有数；只在清单与代码明显不符时才问用户。
7. **识别 fork 改动文件**：`git diff $BASE..dev --name-only` 列出 dev 相对共同祖先的自定义改动，分类（fork 专有文件 vs fork 修改的上游文件），关联到功能清单。
8. **选择合并策略**（见下）。
9. **执行合并**：切到 `dev`（须先确认）→ `git merge <目标> --no-ff --no-edit`，冲突则停止等用户。
10. **验证构建和测试**（必须，见 workflow.md 第 7 节）：`go build ./...` + 基于 fork-features.md 逐项验证 fork 功能。
11. **报告结果**：成功/冲突/新增提交摘要/fork 功能验证/后续步骤。

## 保护 dev 自定义功能

- 合并前用 `git log $BASE..dev --oneline` 确认 dev 领先共同祖先的自定义提交。
- 对 fork 改动的文件优先手动三方对比（`git diff $BASE..dev -- <file>` vs `git diff $BASE..<目标> -- <file>`）。
- `--no-ff` 保留 merge commit；出问题且未推送时可 `git merge --abort`（合并中）或 `git reset --hard ORIG_HEAD`（合并已完成）安全回退。
- **注意**：#3 cpaopenai / #4 codex 转换器的真正上游是参考仓库 CLIProxyAPI，官方 merge 时只需保证这些文件不被误删/误改，其功能更新走 cliproxyapi-translator-sync 技能。

### 合并策略选择

**策略 A — 逐文件审查（推荐）**：fork 改动多（5+ 文件）、上游有重构/API 变更、涉及关键文件（`relay/` 主流程、`channel_select.go`、计费、配置、DB）。方法：每个 fork 文件三方对比 → 意图分析 → 手动合并 → 验证。详见 [workflow.md 第 9 节](references/workflow.md#9-逐文件审查合并推荐策略)。

**策略 B — Git 自动合并 + 语义审查（快速但有风险）**：fork 改动少（<5 文件）、上游变更小（bug 修复/文档）、非关键文件。方法：`git merge` → 解决文本冲突 → **必做语义冲突审查**。

### 语义冲突审查（策略 B 必做）

Git 只检测文本冲突。自动合并后必须检查：
1. **影响范围**：冲突函数/类型的所有调用点。
2. **调用方适配**：返回类型、参数、行为变更是否需调用方修改。
3. **注册完整性**：路由（`router/`）、中间件、渠道适配器注册是否被覆盖。
4. **计费/额度逻辑**：上游改 usage 解析或结算流程，是否影响 billingexpr 注入点。
5. **DB 兼容**：改动是否保持 SQLite/MySQL/PostgreSQL 三库兼容（项目 Rule 2）。
6. **测试同步**：断言、mock 数据是否需更新。

newapi 高发语义冲突点：`service/channel_select.go`（SuccessRateSelector/亲和性集成）、`relay/` 计费与 usage 主流程、`relay/claude_handler.go`（第三方 codex 分流）、渠道设置 DTO。详见 [workflow.md 第 6.5 节](references/workflow.md#65-语义冲突审查必须执行)。

## 必须先征求用户确认

- 切换到 `dev` 分支。
- 新增或修改 upstream remote URL。
- 工作区有未提交变更时仍要继续合并。
- 合并冲突的处理策略。
- 推送到远端（只推 `dev`）。

## 参考

- **Fork 功能清单**（合并前必读，判断语义冲突、保护功能）：[references/fork-features.md](references/fork-features.md)
- **详细操作**（三方对比、逐文件审查、语义冲突审查、构建测试验证、回退命令）：[references/workflow.md](references/workflow.md)
