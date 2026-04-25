# OpenAI2Claude

## 文档目的

本文档用于记录 `new-api` 中“Anthropic ingress + OpenAI family upstream”能力的迁移来源、来源文件版本、在 `new-api` 中的落点与改动方式，方便后续继续从 `CLIProxyAPI` 做增量升级时快速对比与迁移。

本文档不是实现说明书，而是**迁移台账**：

- 记录本次移植主要参考了 `CLIProxyAPI` 的哪些文件
- 记录这些文件在开始移植时对应的版本
- 记录 `new-api` 中对应改动落在哪些文件
- 记录后续升级时应如何做 diff

---

## 上游来源仓库

- 来源仓库：`CLIProxyAPI`
- 本次迁移基线提交：`8ced7a548f7571876e74748072e638956f9805aa`
- 基线提交日期：`2026-04-21`
- 基线提交说明：`Merge pull request #2834 from muzhi1991/fix/openai-compat-host-header`

说明：

后续如果需要继续同步 `CLIProxyAPI` 的 OpenAI <-> Claude 转换能力，优先从本基线开始做 `git diff`，避免遗漏中间协议修复。

### 基线后的已核查增量

以下提交已在本地做过针对性核查：

- `755ca758` — `fix: address review feedback - init ToolNameMap eagerly, log collisions, add collision test`
- `e8bb3504` — `fix: extend tool name sanitization to all remaining Gemini-bound translators`

本次在 `new-api` 中吸收的增量范围：

- 补齐 sanitized tool name -> original tool name 的恢复辅助逻辑
- 为 OpenAI -> Claude 非流式响应补充 sanitized tool name 恢复测试
- 为本地工具函数补充 sanitize/collision 测试

本次**未**引入的部分：

- `CLIProxyAPI` 中与 Gemini/Vertex 专用 translator 直接耦合的其他链路改造
- 上游日志告警（collision warn log）等非关键运行时附加行为

---

## CLIProxyAPI 来源文件清单

### 1. 请求转换主文件

- 来源文件：`internal/translator/openai/claude/openai_claude_request.go`
- 用途：Claude request -> OpenAI chat request
- 最近一次来源提交：`2bd646ad7092264767431142c245ee72fa0463b1`
- 最近一次来源提交日期：`2026-03-19`
- 最近一次来源提交说明：`refactor: replace sjson.Set usage with sjson.SetBytes to optimize mutable JSON transformations`

重点关注能力：

- `system` 到 OpenAI system message 的转换
- `thinking` / `reasoning_effort` 相关映射
- `tool_use` / `tool_result` 组织顺序
- `redacted_thinking` 忽略逻辑
- request JSON 变换方式

### 2. 响应转换主文件

- 来源文件：`internal/translator/openai/claude/openai_claude_response.go`
- 用途：OpenAI response / stream -> Claude response / SSE
- 最近一次来源提交：`2bd646ad7092264767431142c245ee72fa0463b1`
- 最近一次来源提交日期：`2026-03-19`
- 最近一次来源提交说明：`refactor: replace sjson.Set usage with sjson.SetBytes to optimize mutable JSON transformations`

重点关注能力：

- non-stream response -> Claude message
- stream chunk -> Claude SSE event 序列
- `message_start` / `content_block_*` / `message_delta` / `message_stop` 状态机
- tool call 累积与输出
- usage / cached token usage 回填
- reasoning / thinking block 映射

### 3. 通用转换工具文件

- 来源文件：`internal/util/translator.go`
- 用途：工具名、JSON 修复、schema 兼容相关辅助逻辑
- 最近一次来源提交：`e8bb350467e3ff25b6e52815d00b774cde39e185`
- 最近一次来源提交日期：`2026-03-22`
- 最近一次来源提交说明：`fix: extend tool name sanitization to all remaining Gemini-bound translators`

重点关注能力：

- `ToolNameMapFromClaudeRequest(...)`
- `CanonicalToolName(...)`
- `MapToolName(...)`
- `SanitizedToolNameMap(...)`
- `RestoreSanitizedToolName(...)`
- `FixJSON(...)`
- 其他与 schema / tool name sanitize 相关的辅助逻辑

### 4. Claude tool id 辅助文件

- 来源文件：`internal/util/claude_tool_id.go`
- 用途：规范化 `tool_use.id`
- 最近一次来源提交：`553d6f50ea10545c0462b40d9083dbb2f4a396bf`
- 最近一次来源提交日期：`2026-03-06`
- 最近一次来源提交说明：`fix: sanitize tool_use.id to comply with Claude API regex ^[a-zA-Z0-9_-]+$`

重点关注能力：

- `SanitizeClaudeToolID(...)`

### 5. Claude SSE bytes 辅助文件

- 来源文件：`internal/translator/common/bytes.go`
- 用途：构造 Claude SSE event bytes 片段
- 最近一次来源提交：`2bd646ad7092264767431142c245ee72fa0463b1`
- 最近一次来源提交日期：`2026-03-19`
- 最近一次来源提交说明：`refactor: replace sjson.Set usage with sjson.SetBytes to optimize mutable JSON transformations`

重点关注能力：

- `AppendSSEEventBytes(...)`
- `ClaudeInputTokensJSON(...)`
- 其他与 Claude SSE 输出有关的 bytes helper

---

## new-api 中的目标落点

本次迁移遵循“**新文件承接转换逻辑，旧文件只做最小 glue 改动**”原则。

### 建议新增文件

建议把从 `CLIProxyAPI` 承接的转换逻辑放在 `relay/channel/openai/` 下，靠近现有 adaptor：

- `relay/channel/openai/claude_request_translator.go`
- `relay/channel/openai/claude_response_translator.go`
- `relay/channel/openai/claude_translator_utils.go`
- `relay/channel/openai/claude_sse_bytes.go`

这样做的目的：

- 更贴近 `openai.Adaptor` 的实际调用位置
- 减少在 `service/` 包里引入 relay 层依赖
- 后续从 `CLIProxyAPI` 做文件级 diff 更直接

### 需要修改的现有文件

#### 1. `relay/channel/openai/adaptor.go`

修改目标：

- `ConvertClaudeRequest(...)` 改为优先调用新引入的 Claude request translator
- 继续保留 URL、header、request 发送职责
- 不在这里重新实现转换细节

#### 2. `relay/channel/openai/helper.go`

修改目标：

- 使用新引入的 response translator 处理 OpenAI -> Claude 的 stream/non-stream 转换
- helper 继续承担向客户端写回 Claude SSE / JSON 的职责
- 尽量避免在 helper 中继续堆积协议转换细节

#### 3. `service/convert.go`

修改目标：

- 采用保留策略，不直接删除旧实现
- `ClaudeToOpenAIRequest(...)`
- `ResponseOpenAI2Claude(...)`
- `StreamResponseOpenAI2Claude(...)`

说明：

这些旧函数先保留，避免影响潜在调用方。新链路优先接到新复制文件；后续确认没有调用依赖后，再考虑逐步收敛。

#### 4. `dto/openai_request.go`

修改目标：

- `StreamOptions.IncludeUsage bool` -> `*bool`
- `StreamOptions.IncludeObfuscation bool` -> `*bool`

原因：

根据项目 `CLAUDE.md` 的规则，optional scalar 需要使用指针类型以保留显式零值。否则客户端显式传 `false` 时，会因为 `omitempty` 在 marshal 阶段被吞掉。

---

## new-api 修改记录

本节用于记录 `new-api` 最终实际落地时的修改内容。每次有实现变动，都应同步更新本节，方便后续升级时快速判断本地是否偏离来源文件。

### 实际修改记录

#### 新增文件

- `relay/channel/openai/claude_request_translator.go`
  - 来源：`internal/translator/openai/claude/openai_claude_request.go`
  - 承接方式：整体复制 Claude request -> OpenAI request 的核心转换逻辑，在 `relay/channel/openai/` 内独立承接
  - 本地适配点：改用 `dto.GeneralOpenAIRequest` / `dto.ClaudeRequest`；保留 `sjson/gjson` 变换；补接 `thinking`、`tool_use`、`tool_result`、`tool_choice` 与 `stream` 映射

- `relay/channel/openai/claude_response_translator.go`
  - 来源：`internal/translator/openai/claude/openai_claude_response.go`
  - 承接方式：整体复制 OpenAI non-stream / stream -> Claude response / SSE 的状态机与转换逻辑
  - 本地适配点：接入 `relaycommon.RelayInfo` 承载状态；通过 `OriginalRequestRawJSON` 恢复原始 tool name；复用 new-api 的 `dto.ClaudeResponse`、`helper.ClaudeData` 与 usage 结构

- `relay/channel/openai/claude_translator_utils.go`
  - 来源：`internal/util/translator.go` + `internal/util/claude_tool_id.go`
  - 承接方式：合并复制工具名规范化、tool id 清洗、partial JSON 修复与 reasoning effort 映射辅助逻辑
  - 本地适配点：提供 `BoolPtr`、`appendSSEEventBytes`、`sanitizeClaudeToolID`、`toolNameMapFromClaudeRequest` 等供 request/response translator 复用
  - 后续补齐：已追加 `sanitizeFunctionName(...)`、`sanitizedToolNameMap(...)`、`restoreSanitizedToolName(...)`，用于承接基线之后的 sanitized tool name 恢复能力

- `relay/channel/openai/claude_sse_bytes.go`
  - 来源：`internal/translator/common/bytes.go`
  - 承接方式：复制 Claude SSE bytes helper，独立放在 `relay/channel/openai/`
  - 本地适配点：补齐 Claude token count / input token JSON 片段构造，供流式 translator 输出 SSE event bytes

#### 修改现有文件

- `relay/channel/openai/adaptor.go`
  - 修改点：`ConvertClaudeRequest(...)` 改为先保存原始 Claude 请求 JSON 到 `info.ClaudeConvertInfo.OriginalRequestRawJSON`，再调用 `ConvertClaudeRequestToOpenAIRequest(...)`；流式时把 `StreamOptions.IncludeUsage` 改为指针写法
  - 目的：接线到新 request translator，并为后续响应侧恢复 tool name 提供原始请求上下文

- `relay/channel/openai/helper.go`
  - 修改点：Claude 流式输出路径统一改为 `StreamResponseOpenAI2ClaudeWithTranslator(...)`；最终包尾响应也改走新 translator 并补充错误处理
  - 目的：接线到新 response translator

- `relay/channel/openai/relay-openai.go`
  - 修改点：OpenAI 非流式响应转 Claude 改为 `ResponseOpenAI2ClaudeWithTranslator(...)`
  - 目的：让普通 chat completions 的 Claude 出口与新 translator 对齐

- `relay/channel/openai/chat_via_responses.go`
  - 修改点：responses API 回落到 chat completions 时，Claude 非流式与流式出口都改走新 translator；流式场景用 `ClaudeConvertInfo` 保存 usage 与状态机状态
  - 目的：覆盖 responses fallback 场景下的 Claude 输出链路

- `relay/claude_handler.go`
  - 修改点：`chatCompletionsViaResponses(...)` 分支不再调用旧的 `service.ClaudeToOpenAIRequest(...)`，改为保存原始 Claude 请求 JSON 后调用 `openaichannel.ConvertClaudeRequestToOpenAIRequest(...)`
  - 目的：让 Anthropic ingress 的 responses 桥接主入口也走新 request translator

- `relay/compatible_handler.go`
  - 修改点：读取 `request.StreamOptions.IncludeUsage` 时改用 `lo.FromPtrOr(..., true)`；`ForceStreamOption` 下的构造改为 `*bool`
  - 目的：兼容 `StreamOptions` 指针语义，保留客户端显式 `false`

- `relay/common/relay_info.go`
  - 修改点：给 `ClaudeConvertInfo` 增加 `ToolCallBaseIndex`、`ToolCallMaxIndexOffset`、`ToolNameMap`、`OriginalRequestRawJSON`、`StreamTranslatorState`
  - 目的：为 OpenAI <-> Claude translator 提供跨 chunk 状态与原始请求上下文

- `relay/channel/ali/adaptor.go`
  - 修改点：非原生 Anthropic 路径改为保存原始 Claude 请求 JSON 后调用 `openai.ConvertClaudeRequestToOpenAIRequest(...)`；`StreamOptions.IncludeUsage` 改为指针
  - 目的：让 Ali 的 Claude -> OpenAI 兼容路径与主 translator 保持一致

- `relay/channel/gemini/relay-gemini.go`
  - 修改点：Gemini 先转 OpenAI 再回 Claude 的非流式出口改为 `openai.ResponseOpenAI2ClaudeWithTranslator(...)`
  - 目的：避免 Gemini 的 Claude 出口继续停留在旧 response translator

- `relay/channel/ollama/adaptor.go`
  - 修改点：`StreamOptions.IncludeUsage` 改为指针写法
  - 目的：适配 `dto.StreamOptions` 指针语义

- `controller/channel-test.go`
  - 修改点：测试请求构造中的 `StreamOptions.IncludeUsage` 改为 `lo.ToPtr(true)`
  - 目的：适配 `dto.StreamOptions` 指针语义

- `dto/openai_request.go`
  - 修改点：`StreamOptions.IncludeUsage`、`StreamOptions.IncludeObfuscation` 从 `bool` 改为 `*bool`
  - 目的：修复 `StreamOptions` 显式零值语义

- `service/convert.go`
  - 修改点：本次未删除旧实现，保留 `ClaudeToOpenAIRequest(...)`、`ResponseOpenAI2Claude(...)`、`StreamResponseOpenAI2Claude(...)`
  - 目的：保留旧实现，避免破坏性替换；新链路优先改接新增 translator 文件

#### 本次补齐的测试

- `relay/channel/openai/claude_translator_test.go`
  - 补充内容：非流式 OpenAI -> Claude 响应对 sanitized tool name 的恢复测试
  - 目的：覆盖 `mcp/server/read -> mcp_server_read -> mcp/server/read` 这类恢复场景

- `relay/channel/openai/claude_translator_test.go`
  - 补充内容：`sanitizedToolNameMap(...)` 与 `restoreSanitizedToolName(...)` 的 collision / passthrough 测试
  - 目的：覆盖基线后上游新增的 sanitize/collision 行为

---

## 升级迁移操作建议

后续如果 `CLIProxyAPI` 再次升级，可按以下顺序做迁移：

### 1. 先看来源文件是否有变动

优先检查以下文件自基线提交 `8ced7a548f7571876e74748072e638956f9805aa` 之后的变化：

- `internal/translator/openai/claude/openai_claude_request.go`
- `internal/translator/openai/claude/openai_claude_response.go`
- `internal/util/translator.go`
- `internal/util/claude_tool_id.go`
- `internal/translator/common/bytes.go`

如果这些文件有新增提交，优先审查协议修复类改动：

- thinking / reasoning
- tool call / tool result
- usage / cached usage
- tool name / tool id sanitize
- Claude SSE 状态机
- partial JSON 修复

### 2. 以来源文件为主做 diff，不要直接在 new-api 里手写重构

原则：

- 优先保持 `new-api` 新增 translator 文件与来源文件结构接近
- 避免在本地做无收益重写
- 只在必要处做 DTO / 包路径 / helper 调用适配

### 3. 每次升级后同步更新本文档

至少更新：

- 新的来源基线提交
- 每个来源文件最近一次对齐到的提交
- `new-api` 新增/修改文件的实际承接方式
- 如果有本地偏离来源实现的地方，必须明确记录原因

---

## 已知本地约束

迁移时必须遵守 `new-api` 项目约束：

- JSON marshal / unmarshal 使用 `common/*` 封装，不直接调用 `encoding/json`
- optional scalar 使用指针类型，保留显式零值
- 尽量新文件承接逻辑，减少对现有文件的大改
- 不修改受保护项目标识
- 不新增第二套平行主链，只接到现有 `ClaudeHelper + openai.Adaptor` 主链上

### 当前仍保留的本地差异

- `claude_translator_utils.go` 中仅承接当前 OpenAI <-> Claude translator 直接需要的 sanitize 恢复逻辑，未把 `CLIProxyAPI` 的 Gemini/Vertex 相关配套链路整体搬入
- collision 场景当前仅保留“首个映射优先”的行为与测试，未额外引入上游 `log.Warnf(...)` 这类运行时告警

---

## 后续可补充项

如果未来继续补全 Anthropic 兼容能力，可在本文档继续增加以下来源文件记录：

- `internal/translator/openai/claude/init.go`
- `internal/translator/openai/claude/openai_claude_request_test.go`
- `test/thinking_conversion_test.go`
- 与 `/v1/messages/count_tokens` 相关的 translator / token count 实现
