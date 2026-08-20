# 项目约定

本文件只写本项目特有的约定。通用工程规范不重复。

## 协议层

### 1. 做不到的能力必须明确报错,不静默忽略

上游是 chatgpt.com 网页版,能力天花板低于 OpenAI Platform API。
新增任何 OpenAI 参数时,三选一,**没有第四种**:

| 档 | 何时用 | 怎么做 |
|---|---|---|
| 硬拒 | 上游做不到,且用户明确要了该行为 | `writeUnsupportedParam(c, param, why)` |
| 软忽略 | 不影响正确性,只是不生效 | 记进 `ignoredParams`,走响应头 |
| 别名 | 换个名字就能生效 | 在 `validateChatRequest` 里就地映射 |

静默丢弃会让调用方以为参数生效了,拿到错误结果还查不出原因。
这是本项目返工最多的一类问题。

### 2. 判定用"语义有值",不是"字段存在"

真实客户端会无条件带上默认值:openai-node 总发 `"tools":[]`,
LangChain 总发 `"stop":null`,几乎所有 SDK 都发 `"n":1`。
按 presence 判断会把完全正常的请求全部拒掉。

可选字段一律用**指针或 `json.RawMessage`**,不用零值类型 ——
必须能区分"没传"和"传了默认值"。注意 `"null"` 会解成 4 字节的
`RawMessage` 而不是 nil,用 `rawPresent()` 判。

`TestDefaultValuesAreNotRejected` 固化了 34 种客户端惯常发送的默认值,
新增硬拒参数时必须同步往里加对应的良性取值。

### 3. 错误信封四个键全输出

`code` / `message` / `param` / `type` 在官方 spec 里全是 required,
空值为 `null` 而不是省略。统一走 `pkg/oaierr`,不要在 handler 里手拼 `gin.H`。

`type` 必须与状态码语义一致 —— LiteLLM / LangChain 按它决定
"换 key / 换渠道 / 退避重试"。

### 4. 不编造上游不提供的数字

上游不返回 token 计数、不返回缓存命中、不返回图片 usage。
- 估算值(`prompt_tokens`)可以给,但要在注释里写明是 `len/4` 估算
- 未知的元数据(`context_window`)留 0 并 `omitempty`,**不照抄官方 Platform 的数字**
- 图片响应**不输出 usage**,有测试固化这条

编一个数字比缺字段更糟:成本核算侧车会拿它当真。

## 上游协议

### 5. payload 形状改动必须有 HAR 抓包证据

上游对"客户端类型"的判定极其敏感。形状不对时**不会报错**,
而是下发一条 `is_visually_hidden_from_conversation=true` 的空 system message,
表现为"有 SSE 事件但正文为空"。

已验证的两条通路差异:

| | text 通路 | image 通路 |
|---|---|---|
| `system_hints` | `[]` | `["picture_v2"]` |
| `selected_sources` | `[]` ✅ | 不写 ❌ |

`upstreamSlugFallbacks` 同理 —— 每条映射都要有抓包实证,
有测试防止它膨胀。凭猜测加映射会制造难以排查的静默拒绝。

未验证的组合(如 text + attachments)必须**默认关闭**,
用设置项开启,并在 `docs/spec/` 写清验证步骤。

### 6. SSE 生产者必须感知 context

消费方拿到结束标记就 `break`,不会把 channel 读干。
无条件的 `out <- ev` 会让生产者永久阻塞,`defer r.Close()` 永不执行,
goroutine 与上游连接一起泄漏。用 `select { case out <- ev: case <-ctx.Done(): }`。

## 计费与限流

### 7. 限流分桶维度必须与额度归属一致

额度来自用户分组却按 key 分桶的话,同一用户建 N 把 key 就拿到 N 倍限额。
用 `ratelimit.Scope`,`ByUser` 跟着"额度从哪来"走。

### 8. 预扣了就必须有对应的退款/归还路径

- 积分:失败路径调 `refund()`,包括上游零输出和流中断
- TPM:结算时 `AdjustTPM` 的 **delta<0 分支不能是空实现** ——
  入口按 `max_tokens` 或默认 2048 预扣,真实输出常常只有几十 token

### 9. 改价格数字前先读 `docs/spec/pricing.md`

两份历史 seed 迁移的换算注释互相矛盾。迁移文件的
`ON DUPLICATE KEY UPDATE` 刻意不覆盖 `price` 和 `enabled`,
就是为了不冲掉人工调整 —— 改价走后台,不要新增迁移。

### 10. 拉取用户提供的 URL 必须走 `pkg/safefetch`

vision 的 `image_url`、images 的 `reference_images` 都是"按调用方给的 URL 取内容"。
裸 `http.DefaultClient` 有三个问题:能打到内网(含 `169.254.169.254` 云元数据)、
跟随重定向让前置校验白做、错误文案回传 dial 细节就成了内网探测 oracle。

`safefetch` 在 **DialContext 层按真实 IP** 拦截(不是判 hostname —— DNS 可以在
校验之后、连接之前变更),重定向每一跳都重新走一遍,错误统一脱敏。
新增任何"拉用户 URL"的代码路径都必须走它。

### 11. `site.api_base_url` 生产必填

`publicBaseURL` 在该项留空时从 `X-Forwarded-Proto` / `X-Forwarded-Host` 推导,
这两个头是**客户端可伪造的**。推导出的 base URL 会拼进图片 URL,
而图片 URL 会以 markdown 形式进入下游 LLM / 前端渲染。
生产环境请显式配置,或在反向代理层强制覆盖这两个头。

## 测试

### 12. 测试注释写"为什么",不写"测什么"

本项目的测试大量用于固化"曾经错过的行为"。
`// TestXxx 覆盖 F9:此前上游中断也照发 finish_reason:"stop"` 这种注释,
比重复一遍函数名有用得多 —— 它告诉后来者这条断言不能随手删。
