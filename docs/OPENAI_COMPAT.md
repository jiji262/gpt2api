# OpenAI 兼容性对照表

上游是 **chatgpt.com 网页版**,不是 OpenAI Platform API。协议形状严格对齐官方,
但能力天花板不同。**做不到的一律明确报错,不静默忽略** —— 静默丢弃会让调用方
以为参数生效了,拿到错误结果还查不出原因。

## 端点

| 端点 | 状态 | 说明 |
|---|---|---|
| `POST /v1/chat/completions` | ✅ | 流式 / 非流式,含 `stream_options.include_usage` |
| `POST /v1/responses` | ✅ 无状态垫片 | 具名 SSE 事件,无 `[DONE]`;有状态字段明确拒绝 |
| `GET /v1/models` | ✅ | 按 Key 白名单过滤,按 id 稳定排序 |
| `GET /v1/models/{model}` | ✅ | |
| `POST /v1/images/generations` | ✅ | `url` 与 `b64_json` 两种返回 |
| `POST /v1/images/edits` | ✅ | 多图参考;`mask` 明确拒绝 |
| `GET /v1/images/tasks/{id}` | ✅ 扩展 | 非官方端点,查历史任务 |
| `GET`/`DELETE` `/v1/responses/{id}` | ⛔ 501 | 无状态垫片,不留存响应 |
| `/v1/embeddings` `/v1/moderations` `/v1/audio/*` `/v1/files` `/v1/batches` `/v1/realtime` `/v1/vector_stores` `/v1/completions` | ⛔ 501 | 上游无对应通道,响应体里说明原因 |

未匹配的 `/v1/*` 路径回 **404 + OpenAI 错误信封**,并列出当前支持的端点。

## Chat Completions 参数

### ✅ 生效

`model` `messages` `stream` `stream_options.include_usage` `max_tokens`
`max_completion_tokens`(别名到 `max_tokens`) `user`

`max_tokens` 在网关侧**软截断**:上游没有长度上限入口,不截断的话
`finish_reason` 永远不会是 `length`。

### ⛔ 硬拒(400 `unsupported_parameter`,`param` 指向字段)

| 参数 | 原因 | 替代做法 |
|---|---|---|
| `tools` / `tool_choice` / `functions` / `function_call` | 上游没有工具调用通道 | 在 prompt 里描述格式,自行解析 |
| `response_format`(`json_object` / `json_schema`) | 上游没有 schema 约束通道 | 同上,并对结果做容错解析 |
| `n > 1` | 上游一次请求只产出一条回复 | 发多次请求 |
| `stop` | 上游不接受停止序列 | 自行在客户端截断 |
| `logprobs` / `top_logprobs` / `logit_bias` | 上游不返回 token 概率、不接受偏置 | — |
| `presence_penalty` / `frequency_penalty` | 上游不接受采样惩罚项 | — |
| `seed` | 上游不支持确定性采样 | — |
| `modalities`(非 text) / `audio` | 上游只产出文本 | — |
| `prediction` / `web_search_options` | 无对应通道 | — |
| `messages[].role` = `tool` / `function` | 依赖工具调用回填 | — |
| `content` 里的 `input_audio` / `file` part | 无对应通道 | — |
| `content` 里的 `image_url` part | 默认关闭,见下 | 开 `gateway.vision_enabled` |

**判定用"语义有值"而不是"字段存在"**:客户端会无条件带上 `"tools":[]`、
`"stop":null`、`"n":1` 这类默认值,按 presence 判断会把正常请求全拒掉。

### ⚠ 软忽略(200,响应头 `X-Gateway-Ignored-Params` 列出)

`temperature` `top_p` `reasoning_effort` `verbosity` `service_tier`
`store` `metadata` `safety_identifier` `prompt_cache_key`

只在值有实际意义时才记 —— `temperature:1` 是官方默认值,不算忽略。

### 图片输入(vision)

默认关闭。开关是 `gateway.vision_enabled`,原因与验证步骤见
[`docs/spec/vision.md`](spec/vision.md):text 通路 + attachments 的组合
没有 HAR 抓包实证,默认开会把一条本来能用的文本通路变成静默失败。

## 响应

### 非流式

```jsonc
{
  "id": "chatcmpl-...", "object": "chat.completion", "created": 1700000000,
  "model": "gpt-5",
  "choices": [{
    "index": 0,
    "message": { "role": "assistant", "content": "...",
                 "refusal": null, "annotations": [] },
    "finish_reason": "stop",   // stop | length
    "logprobs": null
  }],
  "usage": { "prompt_tokens": 11, "completion_tokens": 3, "total_tokens": 14,
             "prompt_tokens_details": { "cached_tokens": 0 } }
}
```

- `prompt_tokens` 是 `len/4` 的估算值(上游不返回 token 计数)
- `cached_tokens` 恒为 0:上游不提供缓存命中信息
- `refusal` 恒为 null:上游不区分"拒绝回答"和"正常回答",被审核拦下时表现为零输出

### 流式

- 首 chunk 带 `delta.role = "assistant"`
- 每个 chunk 的 `created` 与整条响应一致(不逐帧刷新)
- `stream_options.include_usage` 为真时,倒数第二个 chunk 是 `choices: []` + `usage`
- 以 `data: [DONE]` 收尾

**上游中断 / 零输出时不发 `finish_reason:"stop"`**,改为下发一条
OpenAI 错误信封的 SSE 事件,并全额退款。半截回答不会被伪装成完整答案。

## Responses API

与 Chat Completions 的三处关键差异:

1. 流式用**具名事件**(`event: response.output_text.delta`),不是匿名 data chunk
2. **没有 `[DONE]`**,终止靠 `response.completed` / `response.failed` / `response.incomplete`
3. usage 字段名是 `input_tokens` / `output_tokens`

明确拒绝的字段:`previous_response_id` `conversation` `background:true` `store:true`
—— 本网关不持有服务端会话状态,假装支持只会给出错误结果。

## 错误

四个字段全部输出,空值为 `null`(不是省略):

```json
{"error":{"message":"...","type":"invalid_request_error","param":"tools","code":"unsupported_parameter"}}
```

| 状态码 | type | 场景 |
|---|---|---|
| 400 | `invalid_request_error` | 参数错误、上游做不到的参数 |
| 401 | `authentication_error` | 缺少 / 无效 API Key |
| 402 | `insufficient_quota` | 积分不足(官方用 429,但那会让 SDK 无限退避一个不会自愈的状态) |
| 403 | `permission_error` | 模型未授权、IP 不在白名单 |
| 404 | `not_found_error` | 模型不存在、未知端点 |
| 429 | `rate_limit_error` | RPM / TPM 超限、上游限流,带 `Retry-After` |
| 501 | `server_error` | 上游做不到的端点 |
| 502 / 503 / 504 | `server_error` | 上游故障、账号池空 |

上游的 401/403 会被压成 502:那是"账号池出问题",不是调用方 key 有问题,
原样透传会让 LiteLLM 之类把渠道永久拉黑。

## 响应头

| 头 | 说明 |
|---|---|
| `x-ratelimit-limit-requests` / `-remaining-requests` / `-reset-requests` | RPM 维度 |
| `x-ratelimit-limit-tokens` / `-remaining-tokens` / `-reset-tokens` | TPM 维度 |
| `Retry-After` | 429 时的确切等待秒数 |
| `X-Gateway-Ignored-Params` | 本次被软忽略的参数清单 |
| `X-Gateway-Ratelimit-Degraded` | 限流器降级(Redis 不可用),本次是兜底放行 |

`reset` 用官方的 Go duration 风格(`"1m30s"`),不是裸秒数。
以上全部在 CORS 的 `Access-Control-Expose-Headers` 里,浏览器端 SDK 可读。

## 鉴权

- `Authorization: Bearer sk-xxx`(scheme 大小写不敏感)
- 也接受 `api-key` / `X-Api-Key` 头
- `OpenAI-Organization` / `OpenAI-Project` / `OpenAI-Beta` 被接受且忽略
