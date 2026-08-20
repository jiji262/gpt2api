# gpt2api 对照 OpenAI 最新规范 差距审计

> **状态(2026-08-20 更新)**:本报告的 12 条已验证 finding 与 37 条未验证观察
> **已在分支 `feat/openai-compat` 上全部处理**。逐项对照与最终协议行为见
> [`docs/OPENAI_COMPAT.md`](OPENAI_COMPAT.md);约定沉淀见 [`docs/spec/`](spec/)。
> 本文保留为审计过程记录,不再是待办清单。
>
> 唯一**未落地**的一项:chat 与 image 的相对定价(U33)。这是运营决策不是工程问题,
> 换算口径已写进 [`docs/spec/pricing.md`](spec/pricing.md),数字请在后台自行调整。

本次审计以 openai/openai-openapi spec v2.3.0（2026-08-15）与 developers.openai.com 现行文档（2026-08-19 抓取）为规范真源，逐条对照 `本仓库` 的实际代码路径，所有"已核实"结论均有 file:line 与实测复现支撑；局限有三：(1) 上游是 chatgpt.com 网页逆向，能力天花板天然低于官方 Platform API，报告中用 `[能力]` 明确区分"改网关能修"与"上游做不到"；(2) 未起真实服务打端到端流量，SDK 侧行为部分依据 openai-python 3.3.0 / openai-node 源码推断；(3) 报告末尾 37 条为未经代码级核实的观察，需人工确认后再排期。

---

## 结论速览

按**执行顺序**排列（不是按严重度机械排序）。工作量按单人估算，S = 半天内，M = 1–2 天，L = 3 天以上。

| 编号 | 标题 | 分类 | 严重度 | 工作量 |
|---|---|---|---|---|
| F1 | 错误信封 `type` 恒为 `invalid_request_error`、无 `param` 字段 | 协议 | high | S（2h） |
| F2 | `data[].url` 是相对路径，任何非同 origin 客户端拿到即不可用 | 生态 | high | S（3h） |
| F3 | 图像模型走 `/v1/chat/completions` 时无视 `stream:true`，钱已扣、图已生成、用户拿不到 | 协议 | high | S（2h） |
| F4 | 多模态 `content` 数组直接 400，并把 Go 反序列化错误原文吐给客户端 | 协议 | high | 止血 S（4h）／补能力 L |
| F5 | `tools` / `tool_choice` 被静默吞掉，Agent 类客户端空转到 max_iterations | 协议+能力 | high | S（2h） |
| F6 | `response_format` / `json_schema` 被静默吞掉，Structured Outputs 变自由文本 | 能力 | high | S（1h） |
| F7 | 约 30 个 OpenAI 请求参数被静默丢弃；`max_tokens` 只影响计费不截断输出 | 协议 | high | M（1d） |
| F8 | 上游零输出时把中文运维文案当 assistant 正文，返回 HTTP 200 并计费成功 | 协议 | high | M（1d） |
| F9 | 流中途上游中断/截断仍下发 `finish_reason:"stop"` + `[DONE]` | 协议 | high | M（1d） |
| F10 | `usage.prompt_tokens` 恒为 0；流式路径完全没有 usage | 协议 | medium | S（4h） |
| F11 | images 响应永远没有 `b64_json`，`response_format` 声明后零引用 | 协议 | high | M（1d） |
| F12 | images 响应缺 `usage`，不回显 `size`/`quality`/`background`/`output_format` | 协议 | medium | S（3h） |

**一句话判断**：当前版本对外主推的能力（图像模型）在**任何非同 origin 的标准客户端上都拿不到图**（F2 + F3 + F11 三条叠加），而 chat 通路的参数面几乎是空壳（F7）。这两块是本次审计的主矛盾。

---

## 建议执行顺序

### P0 — 立刻做（不做就有客户端跑不通）

**目标**：让 openai-python / openai-node / Cherry Studio / NextChat / OpenWebUI 用标准写法能跑通图像链路；让所有"做不到"的参数从静默失败变成明确 400。

| 顺序 | 条目 | 耗时 |
|---|---|---|
| 1 | F1 错误信封（后面所有 400 的地基，必须先做） | 2h |
| 2 | F2 图片 URL 补 host | 3h |
| 3 | F3 image-as-chat 的 SSE 分叉 | 2h |
| 4 | F4 多模态 content 止血（扁平化 + 图片 part 明确 400） | 4h |
| 5 | F5 + F6 + F7 统一参数校验层（共用 F1 的 helper） | 1d |

**合计约 2.5 人天。** 做完这批，README.md:344 那句"所有 API 完全兼容 OpenAI 官方 SDK，把 base_url 换成你的部署地址即可"才不算虚假宣传——同时这句话本身也必须在这批里改掉（见 F5 改法第 4 条）。

### P1 — 一周内（体验与正确性）

**目标**：消除所有"HTTP 200 但内容是错的"的静默失败；补齐计量。

| 顺序 | 条目 | 耗时 |
|---|---|---|
| 6 | F8 零输出走错误路径 + 退款 + usage_logs 标失败 | 1d |
| 7 | F9 流中断不再伪装 stop（含 EOF 截断路径） | 1d |
| 8 | F10 usage 补全（非流式真值 + `stream_options.include_usage`） | 4h |
| 9 | F11 `response_format: b64_json` 实装 | 1d |

**合计约 3 人天。** F8 与 F9 是同一段代码（`streamOpenAI` / `collectOpenAI`），建议一个 PR 一起改，否则会互相冲突。

另需在这一周内**人工核实并处理**两条来自未验证批次的高危项（见文末表 U13、U33）：
- `gpt-image-2` 的 `upstream_model_slug` 硬编码成灰度号 `gpt-5-3`——若该号已下线，付费账号出图链路全挂而免费账号反而正常，故障表现反直觉。
- chat 与 image 的相对定价差约三个数量级，`/v1/chat/completions` 路由始终开放，任何持 key 的人现在就能用近乎免费的价格跑 chat。

### P2 — 择机（长期演进）

| 条目 | 耗时 | 说明 |
|---|---|---|
| F12 images 响应字段补全 | 3h | 只补能确定填对的，**不要编造 token 数** |
| F4 完整 vision 能力（`image_url` → 上游 attachment） | L，3–5d | 上游 attachment 协议已跑通（生产在用于图生图），但 `PrepareFChat` 的消息形态一致性需抓包验证 |
| 模型目录治理 | M | 上游模型列表探测 + 下架已退役 slug + `context_window` 等元数据 |
| 定价口径统一 | M | 先定死"1 积分 = 多少厘"，再重算 chat 价 |

**关于要不要做 Responses API：建议"先不做，但必须做两件替代止血"。**

理由：
1. Chat Completions 在 openai-openapi v2.3.0 里 `deprecated` 字段是 `None`，**没有任何 sunset 日期**；被判死刑的是 Assistants API（2026-08-26）。所以这不是"迁移"问题。
2. 真正的痛点是**多个客户端按模型名硬路由到 `/v1/responses`，无视 base_url**：langchain-openai 对任何含 `codex` 的模型名强制走 Responses（`base.py:624-635`）；LobeChat 对 gpt-5.x 且 minorVersion≥2、含 `-codex`/`-pro` 的强制走且用户设置覆盖不了；Vercel AI SDK 的 `openai('id')` 默认就是 Responses。
3. 完整垫片成本约 400–600 行 Go（请求翻译 + 非流式包装 + 具名 SSE 事件翻译 + golden test），且 `previous_response_id` / `conversation` / `background` 需要新建持久化——本仓库没有任何本地会话状态层（出图完就 `PATCH is_visible=false`，chat 每次新开会话）。

**替代止血（合计 S，约 3h，收益 90%）**：
- (a) 把 `/v1/` 前缀下未命中的路由从 `c.Status(404)`（空 body，见 `internal/server/spa.go:56-70`）改成结构化 404 + 可读 message，明确列出本网关支持的端点。这样被硬路由过来的客户端至少能看到原因。
- (b) 把模型 slug 里的 `codex` 字样去掉。`sql/migrations/20260417000001_init_schema.sql:212` seed 的 `gpt-5-codex-max` 是唯一触发 LangChain 硬路由的条目，且它从未被验证过（不在 20260418 的扩展表里，无 HAR 佐证），直接 `enabled=0` 或改名为 `gpt-5-code-max`。
- (c) 补 `GET /v1/models/{id}`（约 20 行，`Registry.BySlug` 现成）。

做完这三条，Responses 的紧迫性从 P1 降到 P2。若之后真有用户投诉被硬路由，再做无状态单向转译垫片（**明确不做** `store` 之外的服务端状态，**绝对不发 `data: [DONE]`**——我核对过 spec，7 处 `[DONE]` 全在 Assistants 的 DoneEvent 和 chat 的 stream_options 描述里，Responses 一处都没有）。

---

## 逐条详述

### F1 · 错误信封 `type` 恒为 `invalid_request_error`、无 `param` 字段

**分类** 协议 | **严重度** high | **工作量** S（2h）

#### 现状

`internal/gateway/chat.go:626-634`：

```go
func openAIError(c *gin.Context, httpStatus int, code, msg string) {
	c.AbortWithStatusJSON(httpStatus, gin.H{
		"error": gin.H{
			"message": msg,
			"type":    "invalid_request_error",   // 硬编码
			"code":    code,
		},
	})
}
```

这是网关唯一的错误出口。401 `missing_api_key`（chat.go:105）、403 `model_not_allowed`（chat.go:139）、429 `rate_limit_rpm`（chat.go:186）、402 `insufficient_balance`（chat.go:213）、502 `upstream_error`（chat.go:566/571）、503 `no_account_available` 全部顶着 `invalid_request_error` 这个 type，且**没有 `param` 键**。`internal/apikey/middleware.go:66-74` 的 `openAIAuthError` 是第二份独立实现，形状相同。

400 的 message 还直接拼 go-playground/validator 的英文原文（chat.go:112），例如 `Key: 'ChatCompletionsRequest.Model' Error:Field validation for 'Model' failed on the 'required' tag`。

#### 官方要求

openai-openapi v2.3.0 的 `Error` schema：`{code: string|null, message: string, param: string|null, type: string}`，**四个字段全部在 required 列表里**——`param` 与 `code` 必然存在、可为 null，不能靠字段缺失判断。
来源：https://raw.githubusercontent.com/openai/openai-openapi/master/openapi.yaml

诚实说明：官方**没有给出 `type` 的完整枚举**（spec 里 type 是裸 string 无 enum），全站只出现过 `invalid_request_error` / `server_error` / `insufficient_quota` 三个字面量。所以下面的映射表里 `rate_limit_error` 等值没有 spec 背书，但它们是 LiteLLM / LangChain 实际分支用的值，比继续发 `invalid_request_error` 正确得多。

#### 影响

按 `error.type` 分支的中间层会做错事：LiteLLM Router、one-api/new-api 的上游聚合、LangChain 的 error handler 普遍用 type 区分"要不要换 key / 换渠道 / 退避重试"。当前 502 上游故障、429 限流、402 余额不足在这些客户端里被一律当成"你的请求参数写错了"，既不重试也不给正确提示。Cherry Studio / NextChat 面板上用户看到的是"请求参数错误"。

缺 `param` 则让 F5/F6/F7 的所有 400 都无法定位到具体字段——这是它必须排在最前面的原因。

#### 改法

在 `internal/gateway/chat.go` 扩展而非替换（保留旧签名做薄封装，避免改遍全仓）：

```go
// openAIErrorFull 输出符合 OpenAI Error schema 的四字段错误对象。
// param 为空时序列化成 null（不是省略）。
func openAIErrorFull(c *gin.Context, httpStatus int, typ, code, param, msg string) {
	e := gin.H{"message": msg, "type": typ}
	if code != "" {
		e["code"] = code
	} else {
		e["code"] = nil
	}
	if param != "" {
		e["param"] = param
	} else {
		e["param"] = nil
	}
	c.AbortWithStatusJSON(httpStatus, gin.H{"error": e})
}

func openAIError(c *gin.Context, httpStatus int, code, msg string) {
	openAIErrorFull(c, httpStatus, typeForStatus(httpStatus), code, "", msg)
}

func typeForStatus(s int) string {
	switch {
	case s == 401:
		return "authentication_error"
	case s == 403:
		return "permission_error"
	case s == 404:
		return "not_found_error"
	case s == 429:
		return "rate_limit_error"
	case s >= 500:
		return "server_error"
	default:
		return "invalid_request_error"
	}
}
```

同时把 `internal/apikey/middleware.go:66-74` 的 `openAIAuthError` 删掉改调这里（需要抽到共享包，或让 middleware 导入 gateway 会成环——建议新建 `pkg/oaierr`，两边都调）。

chat.go:112 的 validator 原文换成人话：把 err 断言为 `validator.ValidationErrors` 后按字段名生成中文提示并填 `param`。

#### 工作量

S，约 2h。改动面：新增 helper + 两处调用点收敛 + validator 文案。无行为风险（只加字段、不删字段）。

---

### F2 · `data[].url` 是相对路径，任何非同 origin 客户端拿到即不可用

**分类** 生态 | **严重度** high | **工作量** S（3h）

#### 现状

`internal/gateway/images_proxy.go:78-85`：

```go
func BuildImageProxyURL(taskID string, idx int, ttl time.Duration) string {
	...
	return fmt.Sprintf("/p/img/%s/%d?exp=%d&sig=%s", taskID, idx, expMs, sig)
}
```

返回**相对 path，没有 scheme 和 host**。`internal/config/config.go:30` 有 `app.BaseURL`，且它已在 `cmd/server/main.go:192`、`main.go:219` 被使用（邮件 / 充值回调），但**从未用于图片 URL**。README.md:326 的配置表却把 `base_url` 注释成"对外 base URL（**签名图片代理用**）"——文档与实现直接背离。

调用点共 **4 处**：`internal/gateway/images.go:306`（ImageGenerations）、`:349`（ImageTask）、`:498`（handleChatAsImage，拼成 markdown `![generated](...)`）、`:813`（ImageEdits）。全部是 `*ImagesHandler` 方法，且都持有 `*gin.Context`。

`/p/img` 路由挂在 root 且无 apikey 中间件（`internal/server/router.go:343-345`），所以补上 host 后确实能跨域直取。

#### 官方要求

dall-e 时代的 `url` 是可直接 GET 的绝对地址（有效期 60 分钟）。任何声明返回 url 的实现，客户端都会直接对该字符串发 HTTP 请求。
来源：https://developers.openai.com/api/reference/resources/images.md

#### 影响

- openai-python `requests.get(img.url)` → `Invalid URL: No scheme supplied`
- openai-node `fetch(img.url)` → 同理
- Cherry Studio / NextChat 的 `<img src>` 只在同 origin 下有效——项目自带 web UI 一直没暴露这个问题，因为它只用 `/api/me/*`
- 项目自己的 API 文档示例 `web/src/views/personal/ApiDocs.vue:158` `print(resp.data[0].url)` 打印出来就是个相对路径

`deploy/nginx.conf:21` 与 `deploy/caddy/*.caddy:8` 都已设置 `X-Forwarded-Proto`，说明真实部署一定是反代 + 非同 origin 客户端。

**修正一处夸大**：网上流传的"重启后历史图集体变砖"不成立——`ImageProxyTTL` 本来就只有 24h（images_proxy.go:73），且 `images_proxy.go:59-60` 注释明写进程重启后旧签名失效"**这是故意的**（防止长期有效的 URL 泄漏）"。真正的矛盾是它和同文件 `:72` 的注释"24h，够前端离线展示一段时间"自相打架。因此**密钥可配置化这条优先级降到 low**，不在本次范围内。

#### 改法

不要只靠注入的 `BaseURL`——`configs/config.example.yaml:5` 的默认值是 `http://localhost:8080`，照抄 example 的部署者会得到一个"绝对但错"的 URL。所有 4 个调用点都有 `*gin.Context`，用请求推导兜底：

```go
// internal/gateway/images_proxy.go
func (h *ImagesHandler) absImageProxyURL(c *gin.Context, taskID string, idx int, ttl time.Duration) string {
	p := BuildImageProxyURL(taskID, idx, ttl)
	if b := strings.TrimRight(h.BaseURL, "/"); b != "" && !strings.Contains(b, "localhost") {
		return b + p
	}
	scheme := c.GetHeader("X-Forwarded-Proto") // nginx.conf:21 / caddy:8 已设置
	if scheme == "" {
		scheme = "http"
	}
	if c.Request.Host == "" {
		return p
	}
	return scheme + "://" + c.Request.Host + p
}
```

`BaseURL` 从 `cmd/server/main.go:146-150` 的 `gateway.ImagesHandler{...}` 字面量注入 `cfg.App.BaseURL`。

**必须改全 4 处**：images.go:306 / 349 / **498** / 813。漏掉 498 则 Cherry Studio / NextChat 走 `/v1/chat/completions` 的图仍然全裂——那条路径的裂图面比 `/v1/images/generations` 更广。

同时订正假文档：删掉 README.md:413-421 的 7.4 async 段（`ImageGenRequest` 无 async/sync 字段，`ShouldBindJSON` 会静默吞掉，照抄会永远同步阻塞 3 分钟）、改 `docs/FRAMEWORK.md:18` 与 `:572` 的 sync/async 表述。

#### 工作量

S，约 3h。含 4 处调用点 + 一个 handler 方法 + 文档订正。

---

### F3 · 图像模型走 `/v1/chat/completions` 时无视 `stream:true`

**分类** 协议 | **严重度** high | **工作量** S（2h）

#### 现状

`internal/gateway/chat.go:153-162`：

```go
if m.Type == modelpkg.TypeImage {
	if h.Images == nil { ... }
	h.Images.handleChatAsImage(c, rec, ak, m, &req, startAt)
	return
}
```

整段**没有任何 `req.Stream` 判断**。`grep -n "Stream" internal/gateway/images.go` 全文件 0 命中。`handleChatAsImage`（images.go:375 起）末尾 `images.go:519` 固定 `c.JSON(http.StatusOK, resp)`，返回 `Content-Type: application/json`。

不是死路径：`cmd/server/main.go:151` 无条件 `gwH.Images = imagesH`；`sql/migrations/20260417000002_seed_gpt_image_2.sql:22` seed 了 enabled 的 `gpt-image-2`；`ListModels`（chat.go:637-652）对 `model.type` **不做任何过滤**，所以 gpt-image-2 必然出现在任何 chat 客户端的模型下拉里——触发条件是"随手一选"，不是"用户特意去用"。

**钱已扣、图已生成**：`images.go:481` `Billing.Settle` 在 `:519` 写响应之前执行，之后无任何退款路径；图已落 `image.Task`（images.go:441）。

#### 官方要求

`stream:true` 必须返回 `Content-Type: text/event-stream`，逐条 `data: {chat.completion.chunk}`，以 `data: [DONE]` 收尾。
来源：https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events

#### 影响

Cherry Studio / NextChat / OpenWebUI / LobeChat 默认全部 `stream:true`。

**订正一处常见误述**：HTTP 响应是完整结束的，不会挂住。openai-python / openai-node 的 SSE 解码器读到单行 JSON，`partition(":")` 后 fieldname 不是 event/data/id/retry，全部丢弃，且没有空行触发 dispatch，最终 yield 零个 chunk 后正常结束——表现是**空回复**，不是 APIError 也不是超时。会真正报错的是自己校验 content-type 的客户端（NextChat 用的 `@fortaine/fetch-event-source` 的 `onopen` 会抛 `Expected content-type to be text/event-stream`）。

两种表现都是：付费成功、图生成了、用户拿不到。

#### 改法

只在 `images.go:500-519` 处按 `req.Stream` 分叉即可：

```go
// images.go:519 处替换
if req.Stream {
	writeImageChatStream(c, m.Slug, sb.String())
	return
}
c.JSON(http.StatusOK, resp)
```

```go
// images.go 新增，头部字段与 chat.go:412-418 的 streamOpenAI 保持一致
func writeImageChatStream(c *gin.Context, model, content string) {
	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	f, _ := w.(http.Flusher)

	id := "chatcmpl-" + uuid.NewString()
	writeChunk(w, f, id, model, DeltaMsg{Role: "assistant"}, nil)
	writeChunk(w, f, id, model, DeltaMsg{Content: content}, nil)
	stop := "stop"
	writeChunk(w, f, id, model, DeltaMsg{}, &stop)
	fmt.Fprint(w, "data: [DONE]\n\n")
	if f != nil {
		f.Flush()
	}
}
```

**错误分支一行都不用改。** images.go:381-387（prompt 空）、403-409（RPM）、414-427（计费）、469-478（生图失败）全部发生在任何 write 之前，`openAIError` 仍然安全——而且这正是 OpenAI 官方行为：`stream:true` 但请求在出第一个 chunk 前失败时，返回带正确 HTTP status 的 JSON error，客户端 SDK 认这个。把它们改成 SSE 收尾反而会把 402/429/502 全压成 200，破坏错误上报。

可选增强（不属于最小闭环）：`images.go:454` 给上游留了 3 分钟超时，上面的方案下客户端在这 3 分钟内一个字节都收不到，可能被中间代理 idle timeout 掐断。若要规避，就在函数开头写 SSE 头 + role delta + 定期 `: ping\n\n` keepalive——但那时错误分支就必须改成 delta 文本 + `finish_reason` 收尾。**两个方案不要混着抄。**

建议顺带把生成的 `taskID` 写进 delta 文本，否则流式失败时用户连 `/v1/images/tasks/:id` 的兜底捞回口都没有。

#### 工作量

S，约 2h。

---

### F4 · 多模态 `content` 数组直接 400，并把 Go 内部类型名吐给客户端

**分类** 协议 | **严重度** high | **工作量** 止血 S（4h）／补能力 L（3–5d）

#### 现状

`internal/upstream/chatgpt/conversation.go:17-21`：

```go
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`   // 裸 string
}
```

`internal/gateway/types.go:8` 请求侧直接复用该类型：`Messages []chatgpt.ChatMessage \`json:"messages" binding:"required"\``；`types.go:29` 同时把它当响应类型（另见 chat.go:530、images.go:507）——所以**不能直接改 `chatgpt.ChatMessage`**。

`chat.go:111-112` 绑定失败即 `openAIError(c, 400, "invalid_request_error", "请求参数错误:"+err.Error())`，Go 错误**原样拼接**给客户端。已实测复现（/tmp 独立 repro，同 json tag + gin v1.12.0 的等价 decode 路径）：

```
json: cannot unmarshal array into Go struct field ChatMessage.messages.content of type string
```

同一 repro 还确认：纯文本单元素数组 `[{"type":"text","text":"hi"}]` 报同一个错；`content:null` 不报错（降级为空串，无需处理）。

全仓无任何兜底：`grep -rn --include='*.go' -E "image_url|ContentPart"` 在 `internal/gateway` 下零命中；`router.go:330` 的 v1 组只挂 `apikey.Middleware`，没有前置 body 改写。

#### 官方要求

user message 的 `content` 可以是 string 或数组，元素 oneOf 4 种：`text` / `image_url{url,detail}` / `input_audio` / `file`。system 与 developer 消息的数组形态只允许 text part。
来源：https://raw.githubusercontent.com/openai/openai-openapi/master/openapi.yaml（`ChatCompletionRequestUserMessageContentPart`）

#### 影响

两层伤害。

第一层：所有 vision 调用 100% 在 bind 阶段就 400，不可达业务逻辑——openai-python/node 的标准 vision 写法、LangChain 的多段 HumanMessage、OpenWebUI / Cherry Studio 传图全挂。

第二层（更隐蔽）：把纯文本包成 `[{"type":"text","text":"..."}]` 单元素数组的客户端连普通对话都 400，用户看到的是一句 Go 反序列化术语，无从下手。这类客户端主要是 Cline / Roo 这种内部用 Anthropic content-block 表示再转 OpenAI 的实现（openai-python 直传字符串、LangChain str content、Vercel AI SDK 会折叠回字符串，那些不受影响）。

#### 改法

**止血（S，本次做）** —— 在 gateway 层加一个请求专用消息类型，不碰 `chatgpt.ChatMessage`：

```go
// internal/gateway/types.go
type ReqMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Name    string          `json:"name,omitempty"`
}

// Flatten 把 content 归一成纯文本；遇到非 text part 返回错误（含 param 路径）。
func (m ReqMessage) Flatten(idx int) (string, string, error) {
	if len(m.Content) == 0 || bytes.Equal(m.Content, []byte("null")) {
		return "", "", nil
	}
	var s string
	if json.Unmarshal(m.Content, &s) == nil {
		return s, "", nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(m.Content, &parts); err != nil {
		return "", fmt.Sprintf("messages[%d].content", idx),
			errors.New("content 必须是字符串或 content part 数组")
	}
	var sb strings.Builder
	for j, p := range parts {
		if p.Type != "text" {
			return "", fmt.Sprintf("messages[%d].content[%d].type", idx, j),
				fmt.Errorf("暂不支持 content part 类型 %q，本网关当前仅支持 text", p.Type)
		}
		sb.WriteString(p.Text)
	}
	return sb.String(), "", nil
}
```

`ChatCompletionsRequest.Messages` 改成 `[]ReqMessage`，在 chat.go:114 附近扁平化后再转回 `[]chatgpt.ChatMessage`。**只有三处会编译失败**，改动面很小：
- `internal/gateway/chat.go:192` `roughEstimateTokens(req.Messages)`（签名在 chat.go:92，参数是 `[]chatgpt.ChatMessage`）
- `internal/gateway/chat.go:342` `Messages: req.Messages` 喂给 `chatgpt.FChatOpts.Messages`
- `internal/gateway/images.go:380` `extractLastUserPrompt(req.Messages)`（签名在 images.go:523，`chatMsg` 是同一别名）

非 text part 用 F1 的 `openAIErrorFull(c, 400, "invalid_request_error", "unsupported_content_part", param, msg)` 报错。

**补能力（L，P2）** —— 比想象中省事，不必新写下载逻辑：
- `internal/gateway/images.go:871 decodeReferenceInputs` 与 `images.go:900 fetchReferenceBytes` 已实现 http(s) URL / dataURL / 裸 base64 三种输入 → 字节，带 4 张上限（images.go:32）和 20MB 单张上限（images.go:29）。把 `image_url.url` 直接喂给 `fetchReferenceBytes`，**不要重复造下载器**。
- `internal/image/runner.go:222-240` 已经是**多张**参考图循环 `UploadFile`，所以 `FChatOpts` 加 `Attachments []*UploadedFile` 复数形态无上游障碍。
- **真实风险点**：文字通路的 `PrepareFChat`（fchat.go:43 起）payload 里 `partial_query` 也是一条完整 user message，带附件时那条 prepare 消息大概率要同步带 `multimodal_text` / `attachments`，否则与 f/conversation 的消息形态不一致——按该文件 `fchat.go:154-158` 的注释，容易触发上游 silent rejection。这一步需要抓包验证。

#### 工作量

止血 S（4h）；补能力 L（3–5d，含抓包验证）。

---

### F5 · `tools` / `tool_choice` 被静默吞掉，Agent 类客户端空转

**分类** 协议（静默吞掉）+ 能力（原生工具调用做不到） | **严重度** high | **工作量** S（2h）

#### 现状

已实测复现（在 `internal/gateway/` 里放临时 test，用真实 `ShouldBindJSON` + 真实 `ChatCompletionsRequest`，跑完已删）：

```
BOUND OK. messages = [{"role":"user","content":"weather in SF?"},{"role":"assistant","content":""},{"role":"tool","content":"ok"}]
Extra = map[string]interface {}(nil)
upstream msg[1] author.role="assistant" parts=[""]      ← tool_calls 消息被压成空 parts
upstream msg[2] author.role="tool"      parts=["ok"]     ← role=tool 原样发给 chatgpt.com
```

支撑事实：
- `internal/gateway/types.go:6-15` 请求结构体只有 7 个字段，无 tools/tool_choice；`types.go:14` 的 `Extra map[string]interface{} \`json:"-"\`` 是**死字段**（`json:"-"` 保证永不填充，全仓零读取方）
- 全仓 `grep -rn "tool_choice\|parallel_tool_calls\|tool_calls\|tool_call_id\|ToolCalls\|ToolChoice" --include="*.go"` → **0 命中**
- 全仓无 `DisallowUnknownFields`，未知字段静默吞掉是确定行为
- 上游确无槽位：`internal/upstream/chatgpt/fchat.go:184-209` 的 payload 是硬编码的 15 个 key（action / messages / parent_message_id / model / client_prepare_state / timezone / system_hints / …），没有任何工具位置
- 响应侧无出口：`types.go:29` `Message chatgpt.ChatMessage`（只有 Role/Content），`chat.go:531` 写死 `FinishReason: "stop"`
- `internal/gateway/delta.go:20-22`、`:95-101`、`:141-146` 主动丢弃 `recipient != "all"` 的增量（上游自带的 python / image_gen 工具调用），进一步证明无工具出口
- `internal/upstream/chatgpt/fchat.go:164` `"author": map[string]string{"role": m.Role}` **原样透传 role，无白名单**
- 计费真实发生：`chat.go:210` PreDeduct 在上游调用之前，成功返回即不退款

#### 官方要求

`tools` 支持 function 与 custom 两类，`tool_choice` 支持 `none`/`auto`/`required`/`allowed_tools`/具名；模型选择调用工具时 `finish_reason="tool_calls"`，非流式给 `message.tool_calls`、流式给按 index 累积的 `ChatCompletionMessageToolCallChunk`。参数不支持时正确做法是 400 + `error.param`。
来源：https://developers.openai.com/api/docs/guides/function-calling

#### 影响

这是本网关对 Agent 生态的一票否决项，且失败形式最隐蔽：Cline / Roo Code / LangChain `create_tool_calling_agent` / OpenAI Agents SDK / 任何 MCP-over-OpenAI 桥接客户端发出 tools 后，网关吞掉，模型不知道有工具存在，只输出自然语言 + `stop`。agent loop 空转到 `max_iterations` 才失败，用户看到的是"模型不会用工具"而不是"网关不支持"，排查成本极高，**且每一轮空转都真实扣费**。

多轮 function calling 的历史更糟：assistant 的 `tool_calls` 消息被压成 `parts:[""]`，等于给上游发一串无意义空消息，上下文直接被污染。全程 HTTP 200。

放大器：`README.md:39` 写"**完全兼容 OpenAI API**"、`README.md:344` 写"所有 API 完全兼容 OpenAI 官方 SDK，把 base_url 换成你的部署地址即可"。

**关于"上游做不到"的诚实表述**：原生保真的工具调用不可能（fchat.go 无槽位、delta.go 无出口）；基于 prompt 注入 + 输出解析的降级模拟**可行但不可靠**（保真度差、不支持 `parallel_tool_calls`）。正确说法是"本期不做"，不是"论证死了"。

#### 改法

1. **触发条件必须收紧**（否则会误伤大量现在跑得通的客户端）：LiteLLM / 部分 LangChain 路径 / OpenWebUI 默认就带 `"tools": []` 和 `"tool_choice": "auto"`。

```go
// internal/gateway/chat.go，绑定成功后、计费之前
if len(req.Tools) > 0 {                       // 空数组放行
	openAIErrorFull(c, 400, "invalid_request_error", "unsupported_parameter", "tools",
		"本网关上游为 chatgpt.com 网页版，不支持 function calling")
	return
}
if len(req.Functions) > 0 {                   // deprecated 的 functions 同理
	openAIErrorFull(c, 400, "invalid_request_error", "unsupported_parameter", "functions", "...")
	return
}
// tool_choice 为 "none" / 缺省 / null 时放行；其余值配合非空 tools 才拒（上面已拦）
```

2. **消息级校验必须先换绑定方式**。`chat.go:111` 用的是 `c.ShouldBindJSON(&req)`，Gin 的这个方法**不缓存 request body**，想再解一层原始 JSON 必须改成 `c.ShouldBindBodyWith(&req, binding.JSON)` 再用 `c.Get(gin.BodyBytesKey)`；或者更省事——`io.ReadAll(c.Request.Body)` 一次，对同一份 bytes 做两次 `json.Unmarshal`。拿到 `[]map[string]json.RawMessage` 后：

```go
// 命中即 400，避免空 parts 污染上游上下文
// param = messages[i].tool_calls  /  messages[i].role
```

3. **role 白名单**（fchat.go:164 原样透传）：只放行 `system` / `developer` / `user` / `assistant`；`tool` 与 `function` 在上游 `author.role` 里没有对应语义，既然 tools 已硬拒，这两个 role 也一并拒掉。

4. **必须同步改文档**（优先级不低于代码）：`README.md:39` 与 `README.md:344` 降级表述，显式列出不支持 function calling。

5. **`/v1/models` 加 `capabilities` 字段这条降为可选**：Cherry Studio / LobeChat 的工具开关读客户端本地模型配置，不读 `/v1/models` 的非标准字段，实际收益接近零。

#### 工作量

S，约 2h（与 F6、F7 共用同一个校验层，一起做更省）。

---

### F6 · `response_format` / `json_schema` 被静默吞掉

**分类** 能力 | **严重度** high | **工作量** S（1h）

#### 现状

全仓 `grep -rn "response_format" --include="*.go"` 仅 2 处命中，**都在 images 路径**：`internal/gateway/images.go:68`（`ResponseFormat string` // url | b64_json）与 `images.go:590`（ImageEdits 文档注释）。chat 路径零命中，`json_schema` / `json_object` 全仓零命中。

`ChatCompletionsRequest`（types.go:6-15）没有槽位；`chat.go:109` `c.ShouldBindJSON` 无 `DisallowUnknownFields` → 静默丢弃 → 200。

上游 payload（fchat.go:184-213）无结构化输出槽位，也无 refusal 通道。

**无任何兜底、无任何声明**：全仓 `grep '```' internal/gateway/` 零命中（不剥 markdown 围栏）；chat.go 不注入任何"请输出 JSON"的 system prompt；`grep -rn "response_format|json_schema|结构化输出" --include=*.md --include=*.sql --include=*.vue` 零命中——调用方在 README、模型目录、`/v1/models` 里都拿不到"不支持"的信号。

#### 官方要求

`response_format` oneOf `{type:"text"}` / `{type:"json_object"}` / `{type:"json_schema", json_schema:{name, schema, strict}}`；`strict=true` 时官方保证输出严格符合 schema，被拒绝时走 `message.refusal`。
来源：https://developers.openai.com/api/docs/guides/structured-outputs

#### 影响

依赖 Structured Outputs 的调用方（LangChain `with_structured_output`、Instructor、任何 `JSON.parse(choices[0].message.content)` 的业务代码）会拿到带 markdown ```json 围栏或前言"好的，以下是结果"的自然语言，解析随机崩溃。因为是 200 且内容"看起来差不多对"，这类 bug 会在生产上零星复现、无法稳定重放。OpenWebUI 的会话标题生成也传这个参数。

`response_format` 是"声明了但从不读"这一族里唯一会让下游 `JSON.parse` 崩溃而不是"输出略有不同"的成员——这是它排在 F7 前面的原因。

#### 改法

**不要做 prompt 伪实现**（上游无 schema 约束通道、无 refusal 通道，伪实现只会把随机失败变成"更像成功的随机失败"）。走结构体字段，跟 images.go:61-68 的写法一致，绕开 body 已消费的问题：

```go
// internal/gateway/types.go，ChatCompletionsRequest 加字段
ResponseFormat json.RawMessage `json:"response_format,omitempty"`
```

```go
// internal/gateway/chat.go，在 fail 闭包定义之后（约 137 行）、ak.ModelAllowed 之前插入
if len(req.ResponseFormat) > 0 && !bytes.Equal(req.ResponseFormat, []byte("null")) {
	var rf struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(req.ResponseFormat, &rf)
	if rf.Type == "json_schema" || rf.Type == "json_object" {
		fail("unsupported_parameter")
		openAIErrorFull(c, http.StatusBadRequest, "invalid_request_error",
			"unsupported_parameter", "response_format",
			"不支持 response_format="+rf.Type+"：上游 chatgpt.com 网页协议无 schema 约束能力，"+
				"请在 prompt 中自行要求 JSON 并做容错解析")
		return
	}
	// {"type":"text"} 等价默认行为，放行
}
```

放在 `fail` 之后是为了让这次拒绝进 usage_logs（defer 块在 chat.go:125-135），放在计费/调度之前是为了不白扣积分。

**注意不要用 `bytes.Contains(req.ResponseFormat, []byte(`"text"`))` 判断**——任何 json_schema 只要 schema 里有个叫 `text` 的属性（极常见）就会误命中并放行。必须解析后按 `type` 分支。

**放弃 ListModels 加 `capabilities` 的方案**：仓库里不存在 `capabilities` 概念（全仓零命中，`internal/model/model.go:15-29` 无该列），需要新加 DB 列 + migration + Registry 改造，成本远超一个 400 拦截；而且它不是 OpenAI `/v1/models` 的标准字段，SDK 和客户端都不会读。要给调用方信号，改 README 或用现成的 `Model.Description`（model.go:24）更直接。

#### 工作量

S，约 1h。

---

### F7 · 约 30 个请求参数被静默丢弃；`max_tokens` 只影响计费不截断输出

**分类** 协议 | **严重度** high | **工作量** M（1d）

#### 现状

`internal/gateway/types.go:6-15` 只声明 7 个 JSON 字段（model / messages / stream / temperature / top_p / max_tokens / user）+ 一个死字段 `Extra`。openai-openapi v2.3.0 的 chat/completions 共 **37 个**请求参数，所以实际被吞的是**约 30 个**（不是网上说的"40+"）。

更精确的分类是两类，都表现为静默无效：

**A. 连解析都没有**（未声明 → `ShouldBindJSON` 静默丢弃，全仓无 `DisallowUnknownFields`）：
`n` / `stop` / `seed` / `logprobs` / `top_logprobs` / `logit_bias` / `presence_penalty` / `frequency_penalty` / `tools` / `tool_choice` / `parallel_tool_calls` / `response_format` / `reasoning_effort` / `verbosity` / `service_tier` / `store` / `metadata` / `max_completion_tokens` / `stream_options` / `prediction` / `modalities` / `audio` / `web_search_options` / `safety_identifier` / `prompt_cache_key` / …

**B. 解析了但从没人读**：`req.Temperature` / `req.TopP` / `req.User` 在 `internal/` 下 grep **零命中**——进结构体即死。

**C. 半有效（最难排查的一类）**：`req.MaxTokens` 只有两个消费点，都在计费：
- `internal/gateway/chat.go:193` `estTokens := req.MaxTokens`（<=0 时兜底 2048）
- `internal/gateway/chat.go:197` `estCost := billing.EstimateChat(m, promptTokens, req.MaxTokens, ratio)`

它**从不进 FChatOpts**（chat.go:340-345 只有 UpstreamModel / Messages / ChatToken / ProofToken），**从不用来截断输出**，`finish_reason` 在 chat.go:470 与 chat.go:531 硬编码 `"stop"`，全仓 `"length"` / `"content_filter"` / `"tool_calls"` 零命中。

**D. 缺 alias**：`max_completion_tokens`（OpenAI 已把 `max_tokens` 标 deprecated 的替代参数）不在结构体里，实测 `MaxTokens=0` → 预扣退回 2048 默认值。新版 SDK / 客户端普遍发这个字段。

实测（临时 test，已删）：把 A 类 19 个参数一次性塞进 body 走 `ShouldBindJSON`，绑定成功，`MaxTokens=0`，`Extra=nil`，全部静默丢弃。

#### 官方要求

来源：https://raw.githubusercontent.com/openai/openai-openapi/master/openapi.yaml

第三方兼容网关的通行契约是：能力做不到就返回 400 + `error.param` 指明哪个参数，绝不能收下再丢弃。`max_tokens` 被上限截断时 `finish_reason` 必须是 `length`。

#### 影响

- `n:3` 的调用方只拿到 1 个 choice 却收 200，以为模型只生成了一条
- `stop:["\n\n"]` 的调用方拿到含停止序列的超长文本
- `logprobs:true` 的评测脚本拿到 `choices[0].logprobs = None`（openai-python 走 Optional 默认值，报 AttributeError 而非 KeyError）
- `logit_bias` / penalties 的调优实验全部无效但看起来"跑通了"
- **`max_tokens=100` 想封顶成本的调用方**：网关按 100 预扣，模型不受限地生成几千 token，最终按实际结算，`finish_reason` 还骗它说是 `stop`。参数对"行为"无效、对"计费"半有效，比单纯忽略更难排查
- **只发 `max_completion_tokens` 的新版客户端**：预扣一律按 2048 估，TPM 桶和积分预扣全部错配

openai-python / node、LangChain、LlamaIndex 传这些参数从不报错，问题只会以"模型行为不对"的形式暴露。

#### 改法

**核心原则：语义判空，不是 presence 判断。** 真实客户端常无条件带上默认字段（`"tools":[]`、`"stop":[]`、`"logit_bias":{}`、`"parallel_tool_calls":false`、`"response_format":{"type":"text"}`、`"tool_choice":null`），按"出现即 400"会把今天完全正常的请求全部打成 400——用"静默做错"换来"大面积炸掉"。

新建 `internal/gateway/unsupported.go`：

```go
// hasMeaningfulValue 判断该参数是否带了"会改变行为的实际值"。
// null / 空数组 / 空对象 / bool false / 数值 0 一律视为未设置。
func hasMeaningfulValue(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	switch s {
	case "", "null", "[]", "{}", "false", "0":
		return false
	}
	return true
}

// hardReject: 上游确实做不到，且带了实际值 → 400
var hardReject = []string{
	"n", "stop", "logprobs", "top_logprobs", "logit_bias",
	"presence_penalty", "frequency_penalty", "seed", "prediction",
	"tools", "functions", "tool_choice", "function_call",
	"response_format", "modalities", "audio", "web_search_options",
}

// softIgnore: 接受但不生效，加响应头告知，不拦
var softIgnore = []string{
	"temperature", "top_p", "user", "store", "metadata", "service_tier",
	"safety_identifier", "prompt_cache_key", "parallel_tool_calls",
	"reasoning_effort", "verbosity",
}
```

三点必须注意：

1. **`reasoning_effort` / `verbosity` 不该进 hardReject**。这两个是 gpt-5 系客户端（Cherry Studio / LobeChat 的思考档位开关）会主动发的；上游 chatgpt.com 本身有 thinking 档位概念，未来可映射到 model slug（目录里已 seed 了 `gpt-5-2-thinking` / `gpt-5-4-thinking` 等）。放 hardReject 等于提前焊死。放 softIgnore + warning header，message 里指路"请直接选择带 `-thinking` 的模型 slug"。

2. **`max_completion_tokens` 应 alias 而不是拒**：

```go
// types.go，用指针区分"未传"与"传了 0"
MaxTokens           *int `json:"max_tokens,omitempty"`
MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`

func (r *ChatCompletionsRequest) OutputCap() int {
	if r.MaxCompletionTokens != nil && *r.MaxCompletionTokens > 0 { return *r.MaxCompletionTokens }
	if r.MaxTokens != nil && *r.MaxTokens > 0 { return *r.MaxTokens }
	return 0
}
```
chat.go:193 / 197 全部改用 `OutputCap()`。

3. **`stream_options.include_usage` 上游能力上做得到**（网关自己就在算 usage），归进硬拒或静默忽略都是浪费——见 F10。

**顺带把 `max_tokens` 变成真实生效的成本闸门**（网关侧软截断，上游截不了但本地能）：在 `streamOpenAI` 的循环（chat.go:449-459）内累计到上限就发 `finish_reason:"length"` 收尾；`collectOpenAI`（chat.go:481-539）同理把 chat.go:531 的硬编码 `"stop"` 改成变量。

**还有一个类型过严的坑要顺手修**：`types.go:9-13` 的 `int` 会让 `"max_tokens":100.0` 直接 400（Go 不接受浮点转 int），而 JS 客户端算出来的值常带小数点。改成 `json.Number` 或自定义 `UnmarshalJSON` 容忍整数值浮点。

拿原始参数 map 需要先解决 body 已被 `ShouldBindJSON` 消费的问题——推荐 `io.ReadAll(c.Request.Body)` 一次后对同一份 bytes 做两次 `json.Unmarshal`，比依赖 gin 的 `ShouldBindBodyWith` 缓存语义更省事。

#### 工作量

M，约 1 天。含参数分档表、helper、软截断、`max_completion_tokens` alias、类型放宽。

---

### F8 · 上游零输出时把中文运维文案当 assistant 正文返回 HTTP 200

**分类** 协议 | **严重度** high | **工作量** M（1d）

#### 现状

流式 `internal/gateway/chat.go:466-468`：

```go
// 兜底:上游接受了请求但没产出任何可见文本
if total.Len() == 0 && evCount > 0 {
	writeChunk(w, flusher, id, model, DeltaMsg{Content: emptyReplyMessage(freeAccount, silentlyRejected)}, nil)
}
stop := "stop"
writeChunk(w, flusher, id, model, DeltaMsg{}, &stop)
fmt.Fprintf(w, "data: [DONE]\n\n")
```

非流式 `chat.go:516-518` 同构，`chat.go:528-539` 组装 `FinishReason:"stop"` 并 `c.JSON(http.StatusOK, resp)`。

`chat.go:595-608` 的 `emptyReplyMessage` 返回的是运维文案，例如"上游检测到当前账号为免费版(chatgpt-freeaccount)…请联系管理员更换 ChatGPT Plus / Team 账号后再试。"

计费与观测：`chat.go:386-391` 用 `lastCompletionTokens`（= 文案长度/4）调 `Billing.Settle`。`internal/billing/engine.go:94-115` 会退还预扣差额，所以**实际扣的是 prompt tokens + 约 38 个"文案 token"，不是全额预扣**（网上说的"全额扣费"不准确）。真正的问题是 `chat.go:403` `rec.Status = usage.StatusSuccess`——**usage_logs 把这次彻底失败记为成功，运维侧完全看不出异常**。

`evCount == 0` 分支（fchat.go:243-247 只对 HTTP>=400 报错；200 后立即 EOF，或首帧 `ev.Err != nil` 在 chat.go:431/488 break）连文案都没有，客户端拿到 `content:""` + `stop` + 200。

`isSilentRejection`（chat.go:585-592）用带空格字面量 `"is_visually_hidden_from_conversation": true` 做 `strings.Contains`——这只影响选哪条文案，不影响兜底是否触发，属于防御性加固而非独立 bug。

#### 官方要求

服务端故障 / 上游拒绝必须通过 HTTP 4xx/5xx + `{"error":{message,type,code,param}}` 表达，不能塞进 `choices[].message.content`。`message.content` 的语义是模型生成内容。
来源：https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create

#### 影响

1. 所有 SDK 判定为成功，异常处理与重试全部绕过
2. Cherry Studio / NextChat / OpenWebUI 直接把中文运维提示渲染成模型回答，用户以为是模型在说话
3. LangChain PydanticOutputParser / 任何 JSON mode 用法拿到中文散文 → 解析异常，且因为 200 不会重试
4. `evCount==0` 分支返回空 content，Cline / Roo Code 判为"no assistant message"并进入重试空转
5. 运维侧 usage_logs 标成功，故障不可回溯

#### 改法

**非流式（`collectOpenAI`）**：条件改为 `content.Len() == 0`（去掉 `evCount > 0`），但**不能只改函数内部**——该函数目前自己 `c.JSON` 写响应。需改签名返回错误，并在 `ChatCompletions` 里于 chat.go:385 之前提前 return：

```go
// ChatCompletions 内
resp, err := h.collectOpenAI(...)
if errors.Is(err, errUpstreamEmpty) {
	refund("upstream_empty_response")            // chat.go:222 的 refund 闭包
	rec.Status = usage.StatusFailed
	rec.ErrorCode = "upstream_empty_response"
	openAIErrorFull(c, http.StatusBadGateway, "server_error",
		"upstream_empty_response", "", emptyReplyMessage(freeAccount, silentlyRejected))
	return                                        // 关键：跳过 chat.go:388 Settle 与 :403 StatusSuccess
}
```

注意 `gatewayFail` 与 `writeStreamError` 在本仓库都**不存在**（grep 0 命中），要新建。

**流式（`streamOpenAI`）**：状态码已在 chat.go:418 写出，只能发 SSE error payload 且**不发 `[DONE]`、不发 `finish_reason:"stop"`**：

```go
func writeStreamError(w io.Writer, f http.Flusher, code, msg string) {
	b, _ := json.Marshal(gin.H{"error": gin.H{
		"message": msg, "type": "server_error", "param": nil, "code": code,
	}})
	fmt.Fprintf(w, "data: %s\n\n", b)
	if f != nil { f.Flush() }
}
```

openai-python / openai-node 看到带顶层 `error` 键的 SSE data 会抛 APIError，这正是我们要的。同样要把"本次失败"回传给 `ChatCompletions` 触发 refund + 失败状态（用 `c.Set("stream_failed", true)`，defer 块读它）。

`isSilentRejection` 建议顺手加固为空格无关（同时兼容紧凑与带空格序列化），或直接 `json.Unmarshal` 判 `message.author.role=="system" && message.metadata.is_visually_hidden_from_conversation==true`，只在前若干帧做以避免全量反序列化开销。

#### 工作量

M，约 1 天。与 F9 同一段代码，建议一个 PR。

---

### F9 · 流中途上游中断/截断仍下发 `finish_reason:"stop"` + `[DONE]`

**分类** 协议 | **严重度** high | **工作量** M（1d）

#### 现状

```go
// chat.go:428-432 流式
for ev := range stream {
	if ev.Err != nil {
		logger.L().Warn(...)
		break              // 不记状态
	}
	...
}
// chat.go:470-475 无条件执行
stop := "stop"
writeChunk(w, flusher, id, model, DeltaMsg{}, &stop)
fmt.Fprintf(w, "data: [DONE]\n\n")
```

非流式 `chat.go:485-489` break 后落到 `chat.go:531` 硬编码 `FinishReason: "stop"` + `chat.go:541` `c.JSON(200)`。

全仓 `finish_reason` 字面量只有 chat.go:470 / chat.go:531 / images.go:511 三处 `"stop"`，`"length"` / `"content_filter"` / `"tool_calls"` 零命中。

**触发面比表面看起来更大（这是关键）**：`ev.Err` 只在 `conversation.go:170` 产生，条件是 `bufio.ReadString` 返回**非 EOF** 错误。而 `parseSSE` 的超时参数被丢弃（`conversation.go:147` `func parseSSE(r io.ReadCloser, out chan<- SSEEvent, _ time.Duration)`），`settings.GatewaySSEReadTimeoutSec` 从未生效——**"读超时→ev.Err"这条设想的路径实际不存在**。

真正高频的中断形态是**上游提前干净关闭 body**：`ReadString` 返回 `io.EOF`，`parseSSE` 走 `conversation.go:172 flush()` 后正常 `close(out)`，`ev.Err` 全程为 nil，`final` 从未为 true，照样落到 chat.go:470 的假 stop。**只补 `ev.Err` 分支，问题仍然存在。**

同样地，`chat.go:403 rec.Status = usage.StatusSuccess` 与 `chat.go:388 Settle` 照常执行——服务端 usage 记录也标成功。

#### 官方要求

`finish_reason` 是 `stop|length|tool_calls|content_filter` 的枚举，`stop` 的语义是「模型自然结束」；`data: [DONE]` 是「流正常终止」的哨兵。规范并对 `include_usage` 明确提示 "If the stream is interrupted or cancelled, you may not receive the final usage chunk."，即中断流应表现为不完整。
来源：https://raw.githubusercontent.com/openai/openai-openapi/master/openapi.yaml

#### 影响

本次审计里最严重的一条——**静默数据损坏**。上游账号被封、被限流、TLS 中断、EOF 截断导致回答只吐了一半时，客户端收到的是一个语法完全合法的"正常完成"响应：

1. Cline / Roo Code / aider 会把被截断的 diff 或代码块当作完整输出写进用户文件，破坏工作区
2. LangChain / LlamaIndex 的 output parser 拿到半截 JSON 解析失败，但因为 HTTP 200 + stop，retry 策略不触发，异常直接冒泡到业务层
3. OpenWebUI / Cherry Studio 把半截回答渲染成完整答案，用户无从察觉
4. 任何自动化 pipeline 都无法区分"模型答完了"和"上游断了"
5. 运维侧 usage_logs 全部是 success

#### 改法

**1. 中断判据必须同时跟踪"有没有见到结束标记"**。`delta.go:58` 的 `Extract` 已能识别 `[DONE]` / `type:message_stream_complete` / `/message/status == finished_successfully` 三种 final：

```go
var streamErr error
sawFinal := false
for ev := range stream {
	if ev.Err != nil { streamErr = ev.Err; break }
	...
	if final { sawFinal = true; break }
}
interrupted := streamErr != nil || !sawFinal
```

`!sawFinal` 分支**上线前建议先只打 metric / 日志观察一轮真实流量**，确认 f/conversation 正常收尾时 100% 落到这三种 final 之一，再让它走中断路径，避免把正常完成误判成中断。

**2. 中断时发 SSE error payload，绝不发 `[DONE]`**（复用 F8 的 `writeStreamError`）：

```go
if interrupted {
	writeStreamError(w, flusher, "upstream_stream_interrupted",
		"上游连接中断，本次回答不完整，请重试")
	c.Set("stream_failed", true)
	return                       // 关键：不写 finish_reason:"stop"，不写 [DONE]
}
```

关于 hijack：`gin.ResponseWriter` 接口本身内嵌 `http.Hijacker`，所以 `w.(http.Hijacker)` 恒为 true，HTTP/2 下 `hj.Hijack()` 必然返回 error，"直接断连"的写法会静默什么都不做。要做就显式处理：

```go
if hj, ok := w.(http.Hijacker); ok {
	if conn, _, err := hj.Hijack(); err == nil { _ = conn.Close() }
}
```

但这只是对付裸 SSE 客户端的加固，**不是必需项**——发了 `data: {"error":{...}}` 且不发 `[DONE]`，openai-python / node 就会抛 APIError。

**3. 同步改状态与结算**：
- `chat.go:403` 的 `rec.Status = usage.StatusSuccess` 改为 `usage.StatusFailed` + `ErrorCode = "upstream_stream_interrupted"`
- 已吐出部分内容的流式请求仍按实际 token 结算（保留 chat.go:388 Settle）
- 非流式因为整个响应作废，走 refund + `openAIErrorFull(c, 502, "server_error", "upstream_stream_interrupted", "", ...)`

**4. `finish_reason` 常量化**，与 F7 的 `"length"` 软截断一起收进一处。

#### 工作量

M，约 1 天。

**顺带记录（建议单独立项，不在本条范围）**：chat.go:432 / :489 break 后不 drain channel；若走 `final` break 而上游仍在发，`parseSSE` 会阻塞在 64 缓冲的 channel 上，`res.Body` 的 `defer r.Close()` 永不执行 → goroutine + 连接泄漏。修法是给 `StreamFChat` 传可取消 context，defer 里 `cancelStream()` + `go func(){ for range stream {} }()`。同时 `settings.GatewaySSEReadTimeoutSec`（settings/model.go:125）在 chat 链路完全未接线。

---

### F10 · `usage.prompt_tokens` 恒为 0；流式路径完全没有 usage

**分类** 协议 | **严重度** medium | **工作量** S（4h）

#### 现状

`internal/gateway/chat.go:533-537`：

```go
Usage: ChatCompletionUsage{
	PromptTokens:     0,                 // 由外层填
	CompletionTokens: completionTokens,
	TotalTokens:      completionTokens,
},
```

chat.go:539 立即 `c.JSON` 写出，函数返回后再无回填点——注释里的"由外层填"从未发生。`promptTokens` 在 chat.go:192 由 `roughEstimateTokens` 算出，消费点只有 chat.go:197 / 202 / 387 / 404，全与响应体无关。

`internal/gateway/images.go:513-517` 三项全 0。

**更严重的一半（常被漏掉）**：流式路径根本没有 usage 对象——`ChatCompletionChunk`（types.go:40-46）**没有 Usage 字段**，`streamOpenAI`（chat.go:412-479）只写 delta + `[DONE]`；`ChatCompletionsRequest` 不解析 `stream_options.include_usage`（全仓无该字符串）。而 Cline / Roo / Cherry Studio / OpenWebUI 默认全走 `stream:true`——它们看到的是"完全没有 usage"，不是"prompt_tokens=0"。

无任何兜底：`router.go:69-74` 的中间件只有 RequestID/Recover/AccessLog/CORS，不改 response body；`internal/gateway/` 下无相关测试。

**内部计费口径是对的**：`billing/pricing.go:9-20 ComputeChatCost` 收到的是真 promptTokens，`chat.go:404 rec.InputTokens` 也是真值——漏计费只影响以本网关为上游的第三方计量层。

#### 官方要求

`CompletionUsage` 的 `prompt_tokens` / `completion_tokens` / `total_tokens` 均为 required，`total_tokens` 定义为 prompt + completion。
`stream_options.include_usage=true` 时，必须在 `data: [DONE]` **之前**额外推一个 chunk，其 `usage` 为整次统计、`choices` 为**空数组**。
来源：https://raw.githubusercontent.com/openai/openai-openapi/master/openapi.yaml

#### 影响

usage 对象在非流式下结构上存在（值类型无 omitempty），所以没有客户端会崩——危害是"安静地全错"：

- LiteLLM / Helicone / Langfuse / OpenLLMetry 这类以网关为上游的观测与计费层，账单一律记 prompt=0，多租户转售直接漏计费
- 流式下 LiteLLM 的 stream cost tracking 拿不到最终 usage chunk，按 0 记
- LangChain 的 `stream_usage=True` 拿不到 `AIMessageChunk.usage_metadata`
- OpenWebUI / Cherry Studio 的 token 与成本显示为空
- `total_tokens` 与 prompt+completion 不自洽，做一致性校验的中间件会报警

Cline / Roo 的 auto-condense 依赖 usage 维护 context 占用——但要注意大多数客户端把**缺失** usage 当 unknown 处理而不是当 0，所以"恒为 0 导致从不压缩"这条因果链在流式主流客户端上不完全成立。

#### 改法

**1. 非流式（3 行）**：把 promptTokens 传进 `collectOpenAI`，chat.go:533-537 改为

```go
PromptTokens:     promptTokens,
CompletionTokens: completionTokens,
TotalTokens:      promptTokens + completionTokens,
```

`images.go:513-517` 同样填 `PromptTokens: roughEstimateTokens(req.Messages)`（`handleChatAsImage` 已持有 req，images.go:376；`chatMsg` 是 `chatgpt.ChatMessage` 的别名，images.go:35，类型直接匹配），`CompletionTokens` 用 0 但 `TotalTokens` 必须等于两者之和。

**2. 流式（真正被主流客户端吃到的那一半）**：

```go
// types.go
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}
// ChatCompletionsRequest 增加:
StreamOptions *StreamOptions `json:"stream_options,omitempty"`
// ChatCompletionChunk 增加(指针 + omitempty，未开启时字段整体不出现，符合规范):
Usage *ChatCompletionUsage `json:"usage,omitempty"`
```

在 chat.go:471 的 finish_reason chunk 之后、`[DONE]` 之前：

```go
if includeUsage {
	ct := (total.Len() + 3) / 4
	chunk := ChatCompletionChunk{
		ID: id, Object: "chat.completion.chunk", Created: created, Model: model,
		Choices: []ChatCompletionChunkChoice{},   // 必须是空数组，不能是 nil（会序列化成 null）
		Usage: &ChatCompletionUsage{promptTokens, ct, promptTokens + ct},
	}
	b, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", b)
	if flusher != nil { flusher.Flush() }
}
```

`nil` slice 会序列化成 `null`，LiteLLM 的 stream chunk builder 会因此报错——必须显式初始化。

**3. tiktoken-go 精度改造请拆成独立条目**：它是新增外部依赖（go.mod 目前无）、要嵌 BPE 词表、且会同时改变计费金额（`billing/pricing.go:16-17` 直接吃这个数），风险面和本条 3 行修复完全不同。另外网上流传的"中文低估 25%"缺依据——`len/4` 字节 ⇒ 每汉字 0.75 token，o200k_base 对中文约 0.6–1.0 token/字，**偏差方向不定**，重构前应先用真实语料测。

#### 工作量

S，约 4h（非流式 3 行 + 流式一个 chunk）。

---

### F11 · images 响应永远没有 `b64_json`，`response_format` 声明后零引用

**分类** 协议 | **严重度** high | **工作量** M（1d）

#### 现状

`internal/gateway/images.go:80-84`：

```go
type ImageGenData struct {
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	FileID        string `json:"file_id,omitempty"`
}
```

无 `b64_json`。`images.go:68` 声明了 `ResponseFormat string \`json:"response_format,omitempty"\` // url | b64_json(暂仅支持 url)`，但**运行时零读取**（全仓只有该声明行 + images.go:590 的文档注释）。传 `b64_json` 既不报错也不生效。

四处响应构造一律只填 URL：`images.go:300-311`（generations）、`images.go:345-354`（ImageTask）、`images.go:807-819`（edits）、`images.go:498`（chat-as-image 拼 markdown）。

`ImageEdits`（images.go:609-628）只读 prompt / model / n / size / upscale + multipart 文件，**完全不读 `response_format`**。

字节来源现状：`internal/image/runner.go:418-446` 为了落盘已经 `FetchImage` 拉过一次完整字节，**但拉完就丢弃**——`RunResult`（runner.go:58-63、406-408）既不带字节也不带 localOK 标记。落盘是 best-effort（runner.go:414-419 注释明说任一张失败就不打 `local_stored` 标记，保持回源兜底）。

#### 官方要求

GPT image 系列（gpt-image-2/1.5/1/1-mini）**始终返回 base64**：'This parameter isn't supported for the GPT image models, which always return base64-encoded images.'；`url` 只在已下线的 dall-e-2/3 上有效。即当前唯一在产的返回形态就是 `b64_json`。
来源：https://developers.openai.com/api/reference/resources/images.md

#### 影响

按 gpt-image 官方语义写的客户端拿不到图：openai-python 的 `resp.data[0].b64_json` 恒为 None；LobeChat / NextChat / OpenWebUI 的 gpt-image 分支读 `b64_json`，一律拿到空值或 NoneType 异常。传 `response_format='b64_json'` 不报错也不生效，用户无从排查。

**与 F2 叠加后是致命的**：`url` 形态返回的是相对路径，对跨 origin 客户端同样不可用——也就是说**当前两种返回形态对外部客户端都不通**，连绕过相对 URL 的逃生口都没有。这让 b64_json 支持的价值比单看这一条更高。

#### 改法

**1. 加字段**（`internal/gateway/images.go:80`）：

```go
B64JSON string `json:"b64_json,omitempty"`
```

**2. 默认值保持 `url`，不要改成 `b64_json`**，只在客户端显式传 `b64_json` 时返 base64；同时对非法值 400：

```go
switch req.ResponseFormat {
case "", "url", "b64_json":
default:
	openAIErrorFull(c, http.StatusBadRequest, "invalid_request_error",
		"invalid_value", "response_format", "response_format 仅支持 url | b64_json")
	return
}
```

理由：同一 handler 同时服务 `/v1`（router.go:336-337）和管理端 Playground（router.go:143-144），而 `web/src/views/personal/OnlinePlay.vue:891` 与 `:1032` 读的是 `img.url`（`<img :src="img.url">`）。改默认值 = 在线体验页立刻白图，`ApiDocs.vue:158` 示例也一起失效。若确实要向官方 gpt-image 语义对齐（默认 b64），必须**同一个 PR 里**把 OnlinePlay.vue 两处改成"优先 url，回退 `data:image/png;base64,${b64_json}`"。

**3. 字节来源：优选改 `RunResult`，不要走"假定已 local_stored"**。

- **优选**：给 `image.RunResult` 加 `Bytes [][]byte`（或 `LocalStored bool`），把 runner.go:422 已经拉下来、当前被丢弃的字节直接带回给 handler。零额外 IO、零回源，天然覆盖落盘失败的情况。
- **次选**：抽 `imageBytes(ctx, t, idx)`，内部必须是"`readLocalImage`（images_proxy.go:227 已存在）→ 失败则回源 `ImageDownloadURL` + `FetchImage`"的完整两段，不能省掉回源分支；且回源依赖 `h.ImageAccResolver` 与 `t.AccountID`（images_proxy.go:172-175 未注入时 503），生成时刻的签名 URL 只有 15 分钟有效。

**4. ImageEdits 补 `c.Request.FormValue("response_format")`** 读取 + 同一套校验（插在 images.go:628 附近），并同步更新 images.go:590 的文档注释。

**5. 注意响应体积**：`n` 最大 4（images.go:117-119），叠加 upscale 4K PNG 时单响应 base64 可到 20–30MB。建议 `upscale != ""` 且请求 b64_json 时明确文档说明或限制。

#### 工作量

M，约 1 天（含 `RunResult` 改造与 4 处构造点）。

---

### F12 · images 响应缺 `usage`，不回显 `size`/`quality`/`background`/`output_format`

**分类** 协议 | **严重度** medium | **工作量** S（3h）

#### 现状

`internal/gateway/images.go:87-91`：

```go
type ImageGenResponse struct {
	Created int64          `json:"created"`
	Data    []ImageGenData `json:"data"`
	TaskID  string         `json:"task_id,omitempty"`
}
```

全仓仅 3 处引用（定义 + `images.go:300-304` generations + `images.go:807-811` edits），无补写逻辑、无中间件兜底。

相关事实：
- 入参侧 `images.go:61-77` 也没有 `background` / `output_format`，连收都不收
- `req.Size` 只落库（images.go:221），**不进 `image.RunOptions`**（runner.go:43-55 无 Size 字段），`RunResult`（runner.go:58-69）无尺寸/格式回传——**服务端根本不知道实际生效的分辨率**
- 内部计费按张不按 token：`images.go:188 billing.ComputeImageCost(m, req.N, ratio)`
- 平行的 chat-as-image 路径 `images.go:513-517` 已经返回全 0 的 usage

#### 官方要求

官方 `ImagesResponse` 顶层含 `created`(required) + `data` + `background` + `output_format` + `size` + `quality` + `usage`；usage 仅 GPT image 模型返回，required 字段 `total_tokens` / `input_tokens` / `output_tokens` / `input_tokens_details{text_tokens,image_tokens}`。
来源：https://developers.openai.com/api/reference/resources/images.md

#### 影响

不影响生图主流程（`data[].url` 正常返回，只读图不读用量的客户端完全不受影响），且 openai-python 把这批字段全声明为 Optional，缺失只是 `None`。危害是：

- 直接 `response.usage.total_tokens` 的代码 AttributeError/undefined
- Go/Java 强类型 SDK 反序列化到非指针 usage 字段时得零值，静默记账为 0
- 回显字段缺失使调用方无法确认服务端实际用了什么 size/quality（尤其在 size 根本没生效的情况下）

因此判 **medium 而非 high**：网关自身计费正确（按张积分），不存在自己算错钱。

#### 改法

分两步，**不要编造 token 数**。

**1. 只补"确定能填对"的字段**（低风险纯收益）：

```go
type ImageGenResponse struct {
	Created      int64          `json:"created"`
	Data         []ImageGenData `json:"data"`
	Background   string         `json:"background,omitempty"`    // "opaque"
	OutputFormat string         `json:"output_format,omitempty"` // 从 res.ContentTypes[0] 推导
	Quality      string         `json:"quality,omitempty"`       // 请求值原样回显，README 注明不区分档位
	TaskID       string         `json:"task_id,omitempty"`
}
```

`output_format` 必须从 `RunResult.ContentTypes`（runner.go:64）映射，**不要硬编码 png**——`internal/image/upscale.go:18` 注释明写"上游 CDN 偶发 webp"，`internal/image/localstore.go:34` 也处理 `image/webp`。

**2. `size` 与 `usage` 默认都 omit**，并在 README 的 `/v1/images/generations` 小节写明：

> 本网关上游为 chatgpt.com 网页链路，不返回 token 计量，故不返回 `usage`；`size` 当前不透传上游，因此不回显，避免误导。

理由：缺字段 → 客户端得 `None`/`undefined`，是**可被发现**的失败；编造 token → LiteLLM 按 gpt-image 单价算出**看起来合理但错误**的金额，是**静默**的失败，明显更糟。

**特别提醒**：网上流传的档位表（Low 272/408/400、Medium 1056/1584/1568、High 4160/6240/6208）是 **gpt-image-1** 的官方计费表；本服务默认模型是 `gpt-image-2`（images.go:112），上游走 `system_hints=["picture_v2"]` 网页链路（runner.go:85-88），token 口径无对应关系。**不要套用。**

若坚持要有 usage 值，必须同时做到：(i) `size` 必须 decode 首张图真实 `Bounds()`（复用 upscale.go:75-83 的 decode 路径），不能回显请求值；(ii) usage 数字必须是本网关积分口径换算或明确置 0。

**3. 顺带修 `images.go:513-517`**（chat-as-image 的全 0 usage），别再复制这个模式——见 F10。

**4. `ImageGenData.file_id`（images.go:83）是非标准扩展**，建议在 README 标注，避免下游误以为是官方字段。

#### 工作量

S，约 3h。

---

## 上游能力天花板

上游是 chatgpt.com 网页版逆向（`internal/upstream/chatgpt/` 只实现了 conversation / f-conversation(+prepare) / sentinel(+prepare/finalize) / files(+download) / rate_limits / me 六类端点），下列 OpenAI 能力**在当前上游协议下根本做不到**。对这些能力，唯一正确的做法是**明确报错，绝不静默忽略**。

### 做不到的清单

| 能力 | 证据 | 现状 |
|---|---|---|
| function calling（原生保真） | `fchat.go:184-209` payload 无工具槽位；`delta.go:20-22/95-101/141-146` 主动丢弃 `recipient != "all"` 的工具增量；`types.go:29` 响应无 tool_calls 出口 | 静默吞（F5） |
| Structured Outputs / `json_schema` strict | 上游无 schema 约束通道，也无 `refusal` 通道 | 静默吞（F6） |
| `logprobs` / `top_logprobs` / `logit_bias` / `seed` | payload 无对应字段 | 静默吞（F7） |
| `n > 1` / `stop` 序列 / `presence_penalty` / `frequency_penalty` / `temperature` / `top_p` | 同上；`req.Temperature`/`req.TopP` 全仓零引用 | 静默吞（F7） |
| `max_tokens` 硬截断 | 上游无长度上限入口 | **可在网关侧软截断**（F7 改法） |
| `reasoning_effort` / `verbosity` 档位 | 上游唯一的推理开关是**模型 slug 本身**（目录已 seed `-thinking` 变体） | 可做 slug 映射（不焊死） |
| 图像 `size` / `quality` 精确控制 | `req.Size` 只落库（images.go:221），不进 `image.RunOptions`（runner.go:43-55 无该字段） | 静默无效（未核实，见 U2） |
| 图像 mask inpainting | 上游无 mask 通道；`images.go:846` 把 mask 与 image 塞进同一数组当参考图 | 静默错误 + 挤占 4 张配额（未核实，见 U3） |
| `background:transparent` / `output_format` / `stream`(图像) | 上游只产出固定形态 | 静默无效（未核实，见 U1） |
| embeddings / moderations / audio / batch / realtime / vector_stores | 上游无任何对应调用面 | 空 body 404（未核实，见 U37） |
| Responses API 的服务端状态（`previous_response_id` / `conversation` / `background`） | 本仓无任何本地会话状态层；出图完即 `PATCH is_visible=false`（`image.go:416-428`） | 空 body 404 |

**做得到但没做的**（不属于天花板，属于漏做）：`stream_options.include_usage`（网关自己就在算 usage）、`b64_json`（字节已在 runner 里拉过）、`max_completion_tokens` alias、`GET /v1/models/{id}`。

### 统一报错方案

新建 `internal/gateway/unsupported.go`，三档处理 + 一条硬规则。

**硬规则：语义判空，不是 presence 判断。** 真实客户端常无条件带默认字段（`"tools":[]`、`"stop":[]`、`"logit_bias":{}`、`"tool_choice":null`、`"response_format":{"type":"text"}`、`"parallel_tool_calls":false`），只在参数带了"会改变行为的实际值"时才拒。

```go
package gateway

// unsupportedParam 输出统一的 400 unsupported_parameter，param 指向具体字段。
// 依赖 F1 的 openAIErrorFull。
func unsupportedParam(c *gin.Context, param, why string) {
	openAIErrorFull(c, http.StatusBadRequest, "invalid_request_error",
		"unsupported_parameter", param,
		fmt.Sprintf("参数 %s 暂不支持：%s（本网关上游为 chatgpt.com 网页版）", param, why))
}

// hasMeaningfulValue 见 F7。
```

**三档策略**：

| 档 | 语义 | 处理 | 适用 |
|---|---|---|---|
| 硬拒 | 上游做不到，且用户明确要了该行为 | 400 + `param` + `code=unsupported_parameter` | tools（非空）/ response_format(json_*) / n>1 / stop(非空) / logprobs / logit_bias / penalties / seed / mask / stream(图像) / background:transparent |
| 软忽略 | 不影响正确性，只是不生效 | 200 + 响应头 `X-Gateway-Ignored-Params: temperature,top_p` + Warn 日志 | temperature / top_p / user / store / metadata / service_tier / parallel_tool_calls / reasoning_effort / verbosity |
| Alias | 换个名字就能生效 | 静默映射 | `max_completion_tokens` → `MaxTokens` |

**报错文案三要素**：说清"哪个参数"（`param`）、"为什么"（上游是网页版）、"怎么办"（替代做法，例如"请直接选择带 `-thinking` 的模型 slug"、"请在 prompt 中自行要求 JSON 并做容错解析"）。

**同时必须做的文档订正**：`README.md:39`「完全兼容 OpenAI API」与 `README.md:344`「所有 API 完全兼容 OpenAI 官方 SDK」——同文件 `README.md:85` 的能力表已如实列出只有 5 个端点，首段的宣称与它自相矛盾。改成「兼容 OpenAI Chat Completions / Images 端点」并链到能力表。**这是所有静默失败的主要放大器，优先级不低于代码改动。**

---

## 未经核实的观察

以下 37 条来自初筛，**均未经过代码级对抗性验证**，行号、grep 结论、影响判断都可能有偏差，排期前需人工确认。已与上文正式条目重复的用「→ 已并入 F#」标注（重复项不必单独排期）。

| 编号 | 标题 | 分类 | 严重度(初筛) | 工作量(初筛) | 需人工确认的点 |
|---|---|---|---|---|---|
| U1 | 7 个官方图像参数字段根本不存在（background / output_format / output_compression / moderation / partial_images / stream / input_fidelity） | 协议 | high | M | `ImageGenRequest` 字段清单；n 超范围静默钳位到 4 是否真的按钳位后的值计费 |
| U2 | `size` 只落库不下发上游，任何尺寸请求都不改变出图 | 能力 | high | M | `RunOptions` 确无 Size 字段；prompt 注入比例提示的实际效果 |
| U3 | edits 把 `mask` 当普通参考图，inpainting 语义完全丢失还占配额 | 能力 | high | M | `collectEditFiles`(images.go:827-857) 是否真把 mask 与 image 混进同一数组；去重键 `filename\|size` 的碰撞概率 |
| U4 | `GET /v1/models/{model}` 未实现，落到无 body 的裸 404 | 生态 | high | S | `spa.go:56-70` 的 NoRoute 行为；**建议与 P2 的 Responses 止血一起做** |
| U5 | 错误信封缺 param、type 恒 invalid_request_error | 协议 | high | M | → **已并入 F1**（我已直接核实 chat.go:626-634） |
| U6 | HTTP 状态码语义偏离：模型 404→400 / 配额 401 / IP 401 / 余额 402 | 协议 | high | M | 官方 error-codes 表把所有额度耗尽定为 429 + insufficient_quota，**表里没有 402**；402 不在 openai-python 命名异常映射内。改 401→429 前需评估 LiteLLM Router 把 401 渠道永久拉黑的现存影响 |
| U7 | 429 无 `Retry-After`、全站零 `x-ratelimit-*` 头 | 协议 | high | M | `token_bucket.go:57-72` 的 Allow() 已返回 remaining 但调用方用 `_` 丢弃；reset 值须是 Go duration 字符串（`"6m0s"`）不是秒数 |
| U8 | 上游 429/401 被压成 502，SDK 拿不到正确重试语义 | 协议 | high | M | `handleUpstreamErr`(chat.go:552-572) 的分支；`UpstreamError` 是否保存了上游响应头 |
| U9 | 流式中断仍下发 finish_reason=stop | 协议 | high | S | → **已并入 F9** |
| U10 | CORS `Allow-Headers` 硬编码，浏览器端 openai-js 预检必挂 | 生态 | high | S | Stainless SDK 在浏览器必发 `x-stainless-*` 系列头；另需确认 `cors_origins:"*"` + `Allow-Credentials:true` 的现存安全暴露面 |
| U11 | 图像模型走 chat 忽略 stream | 协议 | high | S | → **已并入 F3** |
| U12 | 模型目录停在 2026-04-18，且全仓没有任何上游模型列表探测 | 目录 | high | M | `o3`（2026-08-26 从 ChatGPT 退役，即 7 天后）与 `gpt-5-1`（2026-03-11 起不可用）在 seed 里仍 `enabled=1`；失效 slug 会被误报成"限流"（chat.go:601 兜底文案），运维排查方向完全错误 |
| U13 | **gpt-image-2 的上游 slug 硬编码成灰度号 `gpt-5-3`，是出图主链路单点** | 目录 | high | S | `seed_gpt_image_2.sql:23`；`runner.go:251-262` 付费账号原样透传该 slug、免费账号才强制 `auto`。**故障表现是"免费号能出图、付费号出不了"，与直觉相反。建议 P1 内优先核实** |
| U14 | seed 的 `gpt-5-codex-max` 会让 LangChain 强制打 `/v1/responses` 拿空 404 | 生态 | high | S | `init_schema.sql:212`；LangChain `_model_prefers_responses_api()` 对任何含 `codex` 的模型名强制路由。**建议并入 P2 的 Responses 止血（改名或下架）** |
| U15 | `stream_options.include_usage` 不认，非流式 prompt_tokens=0 | 协议 | medium | M | → **已并入 F10** |
| U16 | `max_completion_tokens` 完全不认，max_tokens 只影响计费 | 协议 | medium | M | → **已并入 F7** |
| U17 | `messages:[]` 绕过 required 校验，跑完 PoW 占完账号才以 502 收场 | 协议 | medium | S | `binding:"required"` 对空 slice 不生效；每次 502 会触发 SDK 重试 2 次 → 3 倍 PoW / lease / chat-requirements 消耗。**低成本高收益，建议并入 F7 的校验层** |
| U18 | error.type 恒 invalid_request_error 缺 param | 生态 | medium | S | → **已并入 F1** |
| U19 | `reasoning_effort` / `verbosity` 被吞，而目录里明明有 `-thinking` 变体 | 能力 | medium | S | → 部分并入 F7（softIgnore 档）；slug 映射方案会改变计费单价，上线前需对齐账单口径 |
| U20 | chunk 结构无 usage 字段 | 协议 | medium | S | → **已并入 F10** |
| U21 | max_tokens 不截断，finish_reason 永不为 length | 协议 | medium | M | → **已并入 F7** |
| U22 | 图像模型走 chat 时 stream 被忽略 | 协议 | medium | S | → **已并入 F3** |
| U23 | 消费者 break 后不 drain 也不 cancel 上游流，parseSSE goroutine 与连接可能永久泄漏 | 协议 | medium | M | `conversation.go:136-137` channel 缓冲 32/64；`out <- SSEEvent` 无条件阻塞发送 → `defer r.Close()` 永不执行。加了 F7 的 max_tokens 软截断后会变成**高频命中**。另 `parseSSE` 的 ReadTimeout 参数被丢弃、`GatewaySSEReadTimeoutSec` 是死配置 |
| U24 | `/v1/models` 不按 Key 白名单过滤，顺序不稳定，created 是迁移时间 | 协议 | medium | S | 客户端下拉框会列出点了就 403 的模型；`Registry.ListEnabled` 缓存命中时遍历 map，Go map 迭代顺序随机 |
| U25 | 错误对象 type 恒 invalid_request_error、缺 param | 协议 | medium | S | → **已并入 F1** |
| U26 | 官方在产 slug `gpt-image-1`/`1.5`/`1-mini` 全部不可用，仅认自造名 | 目录 | medium | S | `gpt-image-1` 官方到 2026-10-23 才关停，存量客户端与教程绝大多数仍写它；`seed_gpt_image_2.sql:13-16` 把它置为 `enabled=0` |
| U27 | Redis 故障时限流 fail-open，RPM/TPM 静默全失效 | 协议 | medium | S | `if ok, _, err := ...; err == nil && !ok` 的写法在 Redis 报错时直接放行，且 err 连日志都没打。Redis 一挂账号池会在几十秒内被打穿 |
| U28 | `/v1` 未命中路由与 panic 返回非 OpenAI 形状 | 协议 | medium | S | `mountSPA` 只在 `web/dist` 存在时被调用（router.go:348）——纯 API 部署下这段 NoRoute 根本没注册，需把 /v1 的 404 处理挪到 `router.New()` 里无条件注册。**建议并入 U4** |
| U29 | 限流按 key_id 分桶稀释分组限额；images 无 TPM；TPM 只扣不还 | 协议 | medium | M | 同一用户建 N 把 key 就拿到 N 倍分组限额；`AdjustTPM` 的 delta<0 分支是空实现（占位两行） |
| U30 | models 表无 `context_window` / `max_output_tokens` / 能力标记 | 目录 | medium | M | DAO 用 `SELECT *`，加列不需改查询，只需补 db tag + Create/Update 列清单。注意 `context_window` 真实值取决于上游账号档位，**拿不到就留 0，别照抄 OpenAI 的 1,050,000** |
| U31 | `/v1/responses` 缺失 | 协议 | medium | L | → 见 P2 的明确建议（先不做完整实现，做三条替代止血） |
| U32 | `mapUpstreamModelSlug` 编译期硬编码，与 DB 列构成两套真源，且 image 路径完全绕过 | 目录 | medium | S | chat.go:81-89 的 switch 只有一条 `gpt-5→gpt-5-3`，images.go:260/462/775 直接用 `m.UpstreamModelSlug` 不经过它——**同一个后台配置项在两条路径上行为不同**。目录里 `gpt-5` 与 `gpt-5-3` 最终打同一上游、价格相同，对客户端是两个等价重复 id |
| U33 | **chat 与 image 的相对定价差约三个数量级** | 目录 | medium | M | 两个 seed 迁移的换算注释互相矛盾（`20260418000002.sql:10` 说 1 积分=0.0001 元，`20260417000002.sql:9` 说 1 积分=0.01 元，**差 100 倍**）。当前 500000 ÷ 75000 = 一张图等于 6.67M 个 gpt-5 output token。`/v1/chat/completions` 路由始终开放、16 个 chat 模型照常出现在 `/v1/models`——**任何持 key 的人现在就能用近乎免费的价格跑 chat**。`ENABLE_CHAT_MODEL=false` 只关了 UI。建议 P1 内核实 |
| U34 | `GET /v1/models/{id}` 缺失 | 协议 | medium | S | → **已并入 U4** |
| U35 | 响应字段级：message 无 `refusal`、choices 无 `logprobs`、chunk `created` 每帧漂移 | 协议 | low | S | 主流 SDK 全是 Optional 不会崩；真实风险只在 schema 校验型中间层与按 `(id, created)` 去重的侧车。`created` 漂移是 `writeChunk` 每次调 `time.Now().Unix()` |
| U36 | Bearer 前缀大小写敏感，不支持 `api-key` / `x-api-key` | 协议 | low | S | RFC 7235 规定 auth-scheme 大小写不敏感；当前小写 `bearer` 会报"API Key 格式不正确"，把人引向错误方向 |
| U37 | 上游做不到的端点应显式 501 而非空 body 404 | 能力 | low | S | → 见「上游能力天花板」统一方案；**建议并入 U4/U28 一起做** |

**去重后实际待排期的新条目为 26 条**（U1–U4、U6–U8、U10、U12–U14、U17、U19、U23、U24、U26、U27、U29、U30、U32、U33、U35、U36 及并入项）。其中 **U13（image 上游 slug 单点）与 U33（定价三个数量级偏差）建议提到 P1 内优先人工核实**——前者是可用性单点，后者是收入口子。