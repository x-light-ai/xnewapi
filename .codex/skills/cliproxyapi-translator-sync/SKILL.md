---
name: cliproxyapi-translator-sync
description: 从参考仓库 CLIProxyAPI(CPA)选择性手工移植 Claude↔OpenAI/Codex 协议转换器的更新到 newapi 的 relay/channel/openai 与 relay/channel/codex。适用于：同步 cpaopenai 转 claude、codex 转 claude 的最新转换逻辑，移植 CPA 上游对 request/response translator 的更新，对齐 CPA 转换器 bug 修复与新特性。触发词：同步 CLIProxyAPI、CPA 转换器同步、移植 cpaopenai、codex 转 claude 同步、更新转换器上游、cpa translator sync。注意与 upstream-merge-dev 区分：那是官方 QuantumNous/new-api 的 git tag 合并，本技能是异构参考仓库的定向代码移植（非 git merge）。
---

# CLIProxyAPI 转换器同步

把参考仓库 CLIProxyAPI(CPA)的 Claude↔OpenAI/Codex 协议转换器更新，定向手工移植到 newapi 对应 channel。**这不是 git merge，而是异构仓库的选择性代码移植**：CPA 与 newapi 包结构、helper 命名、import 路径都不同，newapi 的转换器是 CPA 的适配镜像。

## 两个仓库

| 角色 | 位置 | 说明 |
|------|------|------|
| newapi(本仓库) | `relay/channel/openai/claude_*.go`、`relay/channel/codex/claude_*.go` | 移植落点 |
| CPA 参考仓库 | `F:\MyWork\develop\XCPA\refer\CLIProxyAPI` | 是 git repo，可 `git log/diff/show` |

## 方向映射（最易踩坑，务必先核对）

newapi 的场景是「**Claude 客户端 ↔ OpenAI/Codex 上游**」，对应 CPA 里 `internal/translator/<上游>/claude/` 目录。**不要**用反方向的 `internal/translator/claude/<上游>/`（那是 OpenAI/Codex 客户端 ↔ Claude 上游，newapi 不用）。

| newapi channel | ← CPA 源目录 | 关键函数 |
|----------------|-------------|---------|
| `relay/channel/openai/`(Claude↔OpenAI Chat) | `internal/translator/openai/claude/` | `ConvertClaudeRequestToOpenAI` / `ConvertOpenAIResponseToClaude` |
| `relay/channel/codex/`(Claude↔OpenAI Responses) | `internal/translator/codex/claude/` | `ConvertClaudeRequestToCodex` / `ConvertCodexResponseToClaude` |

**判断方向最可靠的方法**：`grep -rE "^func Convert" <目录>`，看函数是 `ConvertClaudeRequestTo*`（Claude 请求→上游，正确方向）还是 `Convert*RequestToClaude`（反方向，不用）。

## helper 适配（CPA → newapi 本地）

CPA 已把公共逻辑重构进 `translatorcommon`/`util` 包；newapi 各 channel 用包内本地 helper。移植时按此表替换：

| CPA | newapi 本地 |
|-----|------------|
| `translatorcommon.AppendSSEEventBytes` | `appendSSEEventBytes` |
| `translatorcommon.ClaudeInputTokensJSON` | `claudeInputTokensJSON` |
| `util.SanitizeClaudeToolID` | `sanitizeClaudeToolID` |
| `translatorcommon.ClaudeMessageSystemReminderText` | 本地 `claudeMessageSystemReminderText`（依赖本地 `isClaudeCodeAttributionSystemText`，若缺则一并移植到该 channel 的 utils） |
| `encoding/json`（如 `json.Marshal`） | `common.Marshal`（项目 JSON Rule 1，禁止业务代码直接用 encoding/json 做 marshal） |

其余 helper（`shortenCodexCallIDIfNeeded`、`buildReverseMapFromClaudeOriginalShortToOriginal`、`startCodexThinkingBlock` 等）newapi 已有同名实现，直接复用。

## 标准流程

1. **定位上次同步基线**：`git -C <newapi> log --oneline --grep=CLIProxyAPI --grep=cpaopenai --grep=codex -i | head`，从最近的同步提交信息里拿到上次移植的 CPA commit hash（如提交 `e5465108f` 记录了 CPA `3a54fb7`）。
2. **列出 CPA 相关目录的新 commit**：对正确方向的两个目录，跑 `git -C <CPA> log --oneline <上次CPA基线>..HEAD -- internal/translator/openai/claude/ internal/translator/codex/claude/`。
3. **过滤与排期**：核对每个 commit 的日期是否晚于 newapi 上次同步；确认它改的是正确方向目录（反方向目录如 `translator/claude/openai/` 的改动跳过，例如某些 cache_control 只在反方向）。
4. **逐 commit 移植**（按时间序，理解累积效果）：
   - **小改动**（新增独立函数、单点逻辑）→ 在 newapi 对应位置手工 Edit 移植。
   - **高度耦合的大重写**（整个状态机重构、参数结构改名+新增字段、跨多 commit 交织）→ 直接 `cp` CPA 文件覆盖 newapi 文件 + `sed` 批量改 package/import/helper 名 + `gofmt`，再验证。详见 references/workflow.md。
5. **移植测试**：CPA 每个 commit 通常带测试。`cp` CPA 的测试文件（`sed` 改 package 即可，被测函数同名同签名），能强验证移植正确性。
6. **验证**：`go build ./...`、`go vet ./relay/channel/{openai,codex}/`、`go test ./relay/channel/openai/ ./relay/channel/codex/ -count=1`。
7. **记录新基线**：提交信息里写明本次同步到的 CPA commit hash，方便下次定位基线。

## 核心原则

- **先核对方向再动手**：方向搞反会移植错代码，是最常见的坑。
- **只移植功能，不跟随纯风格重构**：CPA 可能把 `if/else` 改 `switch`、或 translatorcommon 化；newapi 保留自身结构与本地 helper，只搬功能性变更（除非该文件本就要整体重写）。
- **保持镜像一致**：这些文件是 CPA 的适配镜像，非 newapi 原生代码；移植时贴合 CPA 最新形态 + 本地适配，利于未来持续同步。
- **测试是正确性保障**：优先把 CPA 对应测试一并移植跑通，尤其是复杂的响应状态机（defer function call、pending resolve、web_search、error 转换等）。
- 遵守项目规则：JSON 走 `common.*`（Rule 1）；请求 DTO 保留显式零值（Rule 6）。

## 何时先征求用户确认

- 移植范围很大（涉及新增文件、整文件重写）时，先汇报识别到的 commit 清单与移植计划。
- 某个 CPA 变更依赖 newapi 没有的包/机制（如 registry、thinking provider），需判断是否适用或如何降级时。
- 测试移植后出现失败，说明移植有偏差或行为不一致，停下来报告而非强行跳过。

## 参考

- 逐 commit 的具体操作命令（cp/sed/gofmt 模板）、方向核对命令、测试移植与验证步骤：读取 [references/workflow.md](references/workflow.md)
