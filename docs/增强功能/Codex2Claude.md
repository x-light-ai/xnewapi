# Codex2Claude

## 文档目的

本文档用于整理 `new-api` 中新增 “Claude CLI / Anthropic Messages ingress + Codex upstream” 能力的最小修改方案。

目标场景：

```text
Claude CLI -> NewAPI /v1/messages -> Anthropic Claude 渠道 -> Codex upstream
Codex upstream -> NewAPI -> Claude CLI
```

这里的关键点是：页面上的主渠道类型仍然应保持为 `Anthropic Claude`，因为 Claude CLI 请求入口是 `/v1/messages`，NewAPI 当前会按该入口生成 `RelayFormatClaude`。如果把主渠道类型直接改为 `Codex`，当前代码会进入 `codex.Adaptor.ConvertClaudeRequest(...)`，该函数现在明确返回 `/v1/messages endpoint not supported`。

因此，本方案不是把渠道改成 Codex 类型，而是在 `Anthropic Claude` 渠道下新增一个“上游协议类型”设置，用于告诉转换层真实上游是 Codex。

---

## 当前结论

| 项目 | 当前代码行为 | 结论 |
| --- | --- | --- |
| Claude CLI 请求格式 | `/v1/messages`，NewAPI 识别为 `RelayFormatClaude` | 客户端入口是 Claude 格式 |
| 页面主渠道类型 `Anthropic Claude` | `ChannelTypeAnthropic = 14` -> `APITypeAnthropic` -> `claude.Adaptor` | 请求入口可用，但默认认为上游也是 Anthropic `/v1/messages` |
| 页面主渠道类型 `Codex` | `ChannelTypeCodex = 57` -> `APITypeCodex` -> `codex.Adaptor` | 当前不支持 Claude ingress，直接报错 |
| 真实需求 | 客户端 Claude，真实上游 Codex | 需要在 Anthropic Claude 渠道里额外标记上游协议 |

建议目标链路：

```text
Claude -> Codex Responses -> Claude
```

不建议继续使用当前容易错配的间接链路：

```text
Claude -> OpenAI Chat -> Responses -> Codex
Codex/Responses -> OpenAI Chat -> Claude
```

---

## 配置方案

### 页面位置

新增配置建议放在：

```text
渠道管理 -> 新增/编辑渠道 -> 高级配置 -> 额外设置
```

现有相邻设置包括：

- `Claude 强制 beta=true`
- `允许 inference_geo 透传`
- `允许 speed 透传`
- `允许 service_tier 透传`

### 推荐字段

推荐新增到 `channel.settings` / `dto.ChannelOtherSettings`：

```json
{
  "upstream_protocol": "codex"
}
```

可选值建议：

| 值 | 含义 |
| --- | --- |
| 空 / `anthropic` | 默认行为，上游按 Anthropic Claude `/v1/messages` 处理 |
| `codex` | 上游按 Codex Responses 格式处理；实际 URL 和鉴权由 key 形态分流 |

`upstream_protocol=codex` 下的 key 分流规则：

| 渠道密钥形态 | 上游 URL | 鉴权 |
| --- | --- | --- |
| JSON 对象，例如 `{"access_token":"...","account_id":"..."}` | `/backend-api/codex/responses` | `Authorization: Bearer <access_token>` + `chatgpt-account-id` |
| 普通字符串，例如 `sk-...` | `/v1/responses` | `Authorization: Bearer <key>` |

这样同一个 `Anthropic Claude` 主渠道既能支持官方 Codex OAuth key，也能支持第三方供应商的 Codex/Responses 兼容接口。该分流只根据 key 形态判断，不对上游做试探请求。

不建议优先使用：

```json
{
  "upstream_channel_type": 57
}
```

原因：

- `upstream_protocol` 表达的是协议/转换目标，不是 NewAPI 的完整渠道类型。
- 避免把路由、计费、模型列表等 Codex 渠道行为误绑到 Anthropic Claude 主渠道上。
- 字符串字段更适合最小侵入，未来也可扩展 `openai_responses`、`anthropic` 等值。

---

## 后端最小修改点

### 1. `dto/channel_settings.go`

给 `ChannelOtherSettings` 增加字段：

```go
UpstreamProtocol string `json:"upstream_protocol,omitempty"`
```

建议配套增加常量，避免散落字符串：

```go
const (
    UpstreamProtocolAnthropic = "anthropic"
    UpstreamProtocolCodex     = "codex"
)
```

### 2. `web/src/components/table/channels/modals/EditChannelModal.jsx`

在 `inputs.type === 14` 的 `额外设置` 区域新增选择项：

```text
上游协议类型: Anthropic / Codex
```

保存到 `settings.upstream_protocol`。

读取已有渠道时，从 `data.settings` 解析：

```js
data.upstream_protocol = parsedSettings.upstream_protocol || 'anthropic';
```

提交时，仅 Anthropic Claude 渠道保留该字段：

```js
if (localInputs.type === 14) {
  settings.upstream_protocol = localInputs.upstream_protocol || 'anthropic';
} else {
  delete settings.upstream_protocol;
}
```

### 3. `relay/claude_handler.go`

在 Claude ingress 中保留主渠道类型为 Anthropic Claude，但根据 `info.ChannelOtherSettings.UpstreamProtocol` 分流：

```go
if info.ChannelOtherSettings.UpstreamProtocol == dto.UpstreamProtocolCodex {
    return CodexClaudeHelper(c, info, request)
}
```

这里不建议把 `info.ApiType` 全局改成 `APITypeCodex`，因为 `ApiType` 当前还承担主渠道适配器选择、日志、测试、计费上下文等语义。最小改动应只在 Claude ingress 的转换和请求发送路径里使用 Codex upstream。

### 4. `relay/channel/codex/`

新增 Codex <-> Claude 直接 translator 文件，优先靠近现有 `codex.Adaptor`：

- `relay/channel/codex/claude_request_translator.go`
- `relay/channel/codex/claude_response_translator.go`
- `relay/channel/codex/claude_translator_utils.go`
- `relay/channel/codex/claude_sse_bytes.go`

建议从 CLIProxyAPI 迁移：

- `internal/translator/codex/claude/codex_claude_request.go`
- `internal/translator/codex/claude/codex_claude_response.go`
- `internal/translator/codex/claude/init.go`
- 相关 util：tool name、tool id、JSON 修复、signature/reasoning 兼容逻辑

### 5. Claude -> Codex 专用 helper

推荐新增一个隔离的 Claude -> Codex helper 文件，例如：

- `relay/channel/codex/claude_helper.go`

该 helper 只承接 `Anthropic Claude` 渠道中 `upstream_protocol=codex` 的专用分支：

```text
ClaudeHelper
  -> ChannelOtherSettings.UpstreamProtocol == "codex"
  -> codex.ClaudeHelper(...)
      -> codex/claude request translator
      -> 复用 codex.Adaptor 的 URL/header/request 发送能力
      -> codex/claude response translator
```

推荐做法：

- 复用 `codex.Adaptor.Init(...)`、`ConvertOpenAIResponsesRequest(...)`、`DoRequest(...)`、`SetupRequestHeader(...)`、`GetRequestURL(...)`。
- 在 helper 内局部保存并恢复 `info.RelayMode`、`info.RequestURLPath`，临时按 Responses 请求发送。
- 不改全局 `info.ApiType`。
- 不把主渠道类型改成 `Codex`。
- 不让普通 Codex 主渠道承担 `/v1/messages`。
- 不复制 Codex URL/header/request 发送逻辑。

需要注意：

- 当前 `codex.Adaptor.GetRequestURL(...)` 只接受 `RelayModeResponses` / `RelayModeResponsesCompact`。
- Claude ingress 原始 `info.RelayMode` 不是 Responses，需要在专用 helper 中局部设置为 `RelayModeResponses`。
- `codex.Adaptor.DoResponse(...)` 当前返回 OpenAI Responses 格式，Claude ingress 分支应使用新增的 Codex -> Claude response translator，不走普通 Responses handler。

这样对现有代码的侵入最小：`claude_handler.go` 只增加一个早期分支，`codex.Adaptor` 保持普通 Codex 渠道主行为不变，Codex <-> Claude 的协议细节集中放在新增文件中，后续合并 NewAPI 上游变化时冲突更少。

---

## CLIProxyAPI 来源文件

来源仓库：

```text
F:\My Work\develop\XCPA\refer\CLIProxyAPI
```

### 请求转换

- 来源文件：`internal/translator/codex/claude/codex_claude_request.go`
- 用途：Claude request -> Codex Responses request
- 最近一次来源提交：`aee7a5fbc533298974e4ba5ccb5392b9143279dd`
- 提交日期：`2026-05-29`
- 提交说明：`feat: intercept incompatible signature replay`

重点能力：

- Claude `system` -> Codex developer input
- Claude text/image/tool_use/tool_result -> Codex Responses `input`
- Claude tools -> Codex `tools`
- Claude thinking -> Codex reasoning
- tool name 缩短与恢复映射
- incompatible signature replay 拦截
- Codex 所需 `instructions`、`store=false` 等字段处理

### 响应转换

- 来源文件：`internal/translator/codex/claude/codex_claude_response.go`
- 用途：Codex Responses stream/non-stream -> Claude response/SSE
- 最近一次来源提交：`8bc2eff58a02a92a56ed8ee36aea1cb2f566fba0`
- 提交日期：`2026-05-18`
- 提交说明：`fix: shorten claude codex tool call ids`

重点能力：

- Codex `response.created` / `response.output_*` / `response.completed` -> Claude SSE event
- Codex `function_call` -> Claude `tool_use`
- Codex `function_call_arguments.delta` 累积
- Codex reasoning summary / encrypted content -> Claude thinking/signature 兼容输出
- text block、thinking block、tool block 的状态机管理
- tool call id 缩短，避免 Claude CLI 不兼容

### 测试来源

建议同步迁移或改写以下测试：

- `internal/translator/codex/claude/codex_claude_request_test.go`
- `internal/translator/codex/claude/codex_claude_response_test.go`

---

## 实现后的目标行为

### 配置

页面配置：

| 字段 | 值 |
| --- | --- |
| 渠道类型 | `Anthropic Claude` |
| API 地址 | Codex upstream base URL，例如 ChatGPT/Codex backend base |
| 密钥 | 当前上游要求的 key |
| 上游协议类型 | `Codex` |

底层保存：

```json
{
  "upstream_protocol": "codex"
}
```

### 请求

Claude CLI 请求：

```http
POST /v1/messages
```

NewAPI 内部：

```text
RelayFormatClaude
ChannelTypeAnthropic
ChannelOtherSettings.UpstreamProtocol = codex
```

上游请求：

```http
POST /backend-api/codex/responses  # JSON OAuth key，官方 Codex backend
POST /v1/responses                 # 普通 sk key，第三方 Responses 兼容上游
```

请求体应是 Codex Responses 格式，不再经过 OpenAI Chat 中转。

### 响应

Codex upstream 返回：

```text
response.output_item.added / response.function_call_arguments.delta / response.completed ...
```

NewAPI 输出给 Claude CLI：

```text
message_start
content_block_start type=tool_use/text/thinking
content_block_delta
content_block_stop
message_delta
message_stop
```

工具调用必须是 Claude 结构化 `tool_use` block，不能作为文本 `<tool_use ...>` 直接输出。

---

## 自动检测的边界

当前 NewAPI 的“渠道测试自动检测”主要根据：

- 测试模型名
- endpoint type
- 请求 path
- 渠道类型

它不能可靠判断“同一个 Anthropic Claude 渠道的真实上游到底是 Anthropic 还是 Codex”。

原因：

- 两者可能共用同一个 base URL 代理层。
- 仅靠返回错误或模型名猜测会误判。
- Codex upstream 的认证、URL、响应事件都与 Anthropic 不同，失败后再猜会影响真实请求。

因此建议显式配置 `upstream_protocol`，不要依赖自动检测。

---

## 测试建议

### 后端单元测试

建议新增：

- `ChannelOtherSettings` 能解析 `upstream_protocol=codex`
- Claude request + `upstream_protocol=codex` 会走 Codex translator
- Claude tools 转 Codex tools 时保留 name 映射
- Claude `tool_result` 转 Codex `function_call_output`
- Codex `function_call` 响应转 Claude `tool_use`
- Codex 流式 response 转 Claude SSE 顺序正确
- reasoning/signature/encrypted_content 兼容场景

### 集成测试

至少覆盖：

| 场景 | 预期 |
| --- | --- |
| Anthropic Claude 渠道 + 默认设置 | 仍请求 `/v1/messages` |
| Anthropic Claude 渠道 + `upstream_protocol=codex` + JSON OAuth key | 请求 `/backend-api/codex/responses`，使用 OAuth access token 和 account id |
| Anthropic Claude 渠道 + `upstream_protocol=codex` + 普通 `sk-...` key | 请求 `/v1/responses`，使用 `Authorization: Bearer sk-...` |
| Codex 主渠道类型 + `/v1/messages` | 当前仍不支持，避免误配置 |
| Claude CLI 工具调用 | 客户端收到结构化 `tool_use`，不是文本标签 |

---

## 合并上游友好原则

- 主渠道类型不新增、不改名，继续使用 `Anthropic Claude`。
- 新增配置放入 `settings` JSON，不做数据库迁移。
- Codex <-> Claude 转换逻辑尽量独立放入 `relay/channel/codex/` 新文件。
- 现有 `claude.Adaptor`、`codex.Adaptor` 只做少量 glue 修改，Claude -> Codex 分支隔离到新增 helper 文件。
- 不把 `info.ApiType` 全局改写成 Codex，避免影响路由、日志、计费和渠道测试。
- JSON marshal/unmarshal 使用 `common.Marshal` / `common.Unmarshal`。
- 新增 optional scalar 遵守指针语义，显式零值不能被 `omitempty` 吃掉。
