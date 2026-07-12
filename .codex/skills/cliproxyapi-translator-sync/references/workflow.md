# CLIProxyAPI 转换器移植操作参考

## 目录

1. [方向核对](#1-方向核对)
2. [定位基线与列出新 commit](#2-定位基线与列出新-commit)
3. [小改动手工移植](#3-小改动手工移植)
4. [大重写移植（cp + sed + gofmt）](#4-大重写移植cp--sed--gofmt)
5. [测试移植](#5-测试移植)
6. [验证](#6-验证)
7. [本次同步案例（参考）](#7-本次同步案例参考)

约定：以下 `<CPA>` = `F:\MyWork\develop\XCPA\refer\CLIProxyAPI`，命令在 newapi 仓库根目录执行。

## 1. 方向核对

先确认 newapi 各 channel 对应的 CPA 源目录方向正确（详见 SKILL.md 映射表）。用函数名判断：

```bash
# newapi 侧导出转换函数
grep -rnE "^func (Convert|Translate)" relay/channel/openai/claude_*.go relay/channel/codex/claude_*.go

# CPA 侧：正确方向目录应含 ConvertClaudeRequestTo*（Claude 请求→上游）
grep -rnE "^func Convert" "<CPA>/internal/translator/openai/claude/" "<CPA>/internal/translator/codex/claude/"
```

若在 CPA 看到 `Convert<上游>RequestToClaude`，说明取错了反方向目录（`translator/claude/<上游>/`），换回 `translator/<上游>/claude/`。

## 2. 定位基线与列出新 commit

```bash
# 在 newapi 找上次同步记录的 CPA commit hash
git log --oneline -i --grep=CLIProxyAPI --grep=cpaopenai --grep=codex | head

# 在 CPA 列出两个正确方向目录自基线以来的新提交
git -C "<CPA>" log --oneline <上次CPA基线>..HEAD -- \
  internal/translator/openai/claude/ internal/translator/codex/claude/

# 逐个查看提交日期与涉及文件，与 newapi 上次同步日期对齐
git -C "<CPA>" show -s --format="%ci %h %s" <commit>
git -C "<CPA>" show --stat --format="" <commit> -- <目标目录>
```

排除只改**反方向**目录的提交（如某些 cache_control 只落在 `translator/claude/openai/`）。

## 3. 小改动手工移植

适用：新增独立函数、单点逻辑增改、少量行。

```bash
# 看某个 commit 对某文件的净 diff，理解功能
git -C "<CPA>" diff <基线>..HEAD -- internal/translator/openai/claude/openai_claude_request.go
```

在 newapi 对应文件用 Edit 手工落地，注意：
- 替换 helper 名（见 SKILL.md 适配表）。
- 只搬功能性变更，不跟随纯风格重构（`if/else`→`switch` 等无需照搬）。
- 若依赖 `claudeMessageSystemReminderText` 等 newapi 尚无的本地 helper，一并加到该 channel 的 `claude_translator_utils.go`（连同 `claudeSystemTextParts` 与常量），import 补 `gjson`。

## 4. 大重写移植（cp + sed + gofmt）

适用：整个状态机重构、参数结构改名+新增字段、多个 commit 高度交织、新增文件。逐个 patch 易错时，直接以 CPA HEAD 整文件为蓝本重写。

**前置检查**（确认可安全整体替换）：

```bash
# 确认被重写文件的导出类型/字段无 newapi 外部引用（改名会破坏调用方）
# 例：参数结构、被改名的字段
grep -rn "ConvertCodexResponseToClaudeParams|HasToolCall" --include="*.go" relay/ service/ controller/
# 确认调用点签名不变（通常在 claude_helper.go）
grep -rn "ConvertCodexResponseToClaude" --include="*.go" relay/channel/codex/
```

**执行**（以 codex response 为例）：

```bash
# 1. 覆盖 + 新增文件
cp "<CPA>/internal/translator/codex/claude/codex_claude_response.go"            relay/channel/codex/claude_response_translator.go
cp "<CPA>/internal/translator/codex/claude/codex_claude_response_web_search.go" relay/channel/codex/claude_response_web_search.go

# 2. sed 适配（package / 删 CPA 专有 import / 换 helper 名）
f=relay/channel/codex/claude_response_translator.go
sed -i '1,6d' "$f"                                   # 删 CPA 文件顶部 package 注释块（按实际行数调整）
sed -i 's/^package claude$/package codex/' "$f"
sed -i '\#internal/translator/common#d; \#internal/util#d' "$f"
sed -i 's/translatorcommon\.AppendSSEEventBytes/appendSSEEventBytes/g' "$f"
sed -i 's/translatorcommon\.ClaudeInputTokensJSON/claudeInputTokensJSON/g' "$f"
sed -i 's/util\.SanitizeClaudeToolID/sanitizeClaudeToolID/g' "$f"

g=relay/channel/codex/claude_response_web_search.go
sed -i 's/^package claude$/package codex/' "$g"
sed -i '\#internal/translator/common#d' "$g"
sed -i 's#"encoding/json"#"github.com/QuantumNous/new-api/common"#' "$g"   # 换成 common.Marshal
sed -i 's/translatorcommon\.AppendSSEEventBytes/appendSSEEventBytes/g' "$g"
sed -i 's/json\.Marshal/common.Marshal/g' "$g"

# 3. 验证无残留 + 格式化
grep -nE "translatorcommon|internal/util|internal/translator/common|encoding/json|package claude" "$f" "$g" || echo "OK"
gofmt -w "$f" "$g"
```

> sed 用 `\#...#d` 以 `#` 作分隔符，避免与路径中的 `/` 冲突。删注释块的行号（`1,6d`）按 CPA 文件实际注释行数调整。

整体重写后，newapi 原文件里的 helper（`codexStopReason`、`extractResponsesUsage` 等）会被 CPA 版本一并带入，无需担心丢失——前提是已确认 newapi 无 CPA HEAD 之外的独有逻辑（大多数情况 newapi = 某个 CPA 旧版）。

## 5. 测试移植

CPA 每个 commit 通常带测试，移植过来能强验证正确性：

```bash
cp "<CPA>/internal/translator/codex/claude/codex_claude_response_test.go" \
   relay/channel/codex/claude_response_translator_test.go
sed -i 's/^package claude$/package codex/' relay/channel/codex/claude_response_translator_test.go
gofmt -w relay/channel/codex/claude_response_translator_test.go
```

- 测试的被测函数与 newapi 同名同签名，通常只需改 package。
- 测试内 helper（如 `firstClaudeStreamPayloadForEvent`）随文件带入，注意与 newapi 现有测试文件**无重名冲突**（先 `grep -nE "^func " 两边测试文件` 核对）。
- 若只需补充少数几个测试（如 request 的单点改动），从 `git show <commit> -- <test文件>` 取出新增测试函数，追加到 newapi 现有测试文件，并补齐 import（`fmt`/`gjson` 等）。

## 6. 验证

```bash
go build ./...
go vet ./relay/channel/openai/ ./relay/channel/codex/
go test ./relay/channel/openai/ ./relay/channel/codex/ -count=1
# 定向确认新移植的测试执行
go test ./relay/channel/codex/ -run '<新测试名正则>' -v -count=1
```

测试失败即移植有偏差，回到对应 commit 核对逻辑，不要跳过。

## 7. 本次同步案例（参考）

2026-07 从 CPA `3a54fb7`（newapi 提交 `e5465108f` 记录）同步到 CPA `bc279c61`，共 10 个正确方向 commit：

- **openai channel**（3 个，均 request）：`c13dbcc2` object schema 补 properties、`f1ed8912` system role → user reminder、`d6c4fc2d`（被 f1ed8912 取代）。手工移植 `normalizeObjectSchemaProperties` + reminder wrap + 本地 `claudeMessageSystemReminderText`。
- **codex channel**（7 个）：request `893412e9` service_tier、`f1ed8912` reminder；response `702295d7` error 转换、`30dc2e7f` web_search（新增 `claude_response_web_search.go`）、`a5cb8832` content block、`34639c3c` defer function call、`cdccc72d` terminal resolve pending。response 5 个高度耦合 → 整体重写 + 适配。
- 移植 CPA 28 个 response 测试 + 4 个 request 测试，全部通过。

要点回顾：方向映射（`translator/<上游>/claude/`）、helper 适配、response 整体重写、测试移植验证。
