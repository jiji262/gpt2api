# chat 图片输入(vision)

> 默认**关闭**。开启前必须按本文验证,否则会把一条本来能用的文本通路弄成静默失败。

## 现状

`/v1/chat/completions` 的 `content` 支持 part 数组,`image_url` part 会被解析并保留 URL。
是否真的把图片送给上游,由设置项 `gateway.vision_enabled` 决定:

| 开关 | `image_url` 的行为 |
|---|---|
| 关(默认) | 入口返回 `400 unsupported_parameter`,`param` 指向 `messages[N].content` |
| 开 | 下载/解码 → `UploadFile` 上传 → 挂到该条消息的 `metadata.attachments` |

无论开关如何,`input_audio` / `file` 类型的 part 始终明确拒绝——上游没有对应通道。

## 为什么默认关闭

上游对"客户端类型"的判定非常敏感。已抓包实证的只有两条通路:

| 通路 | `system_hints` | `selected_sources` | `attachments` |
|---|---|---|---|
| text(纯文字对话) | `[]` | `[]` ✅ | 未验证 |
| image(生图/图生图) | `["picture_v2"]` | 不写 ❌ | ✅ 已验证 |

本实现要的是 **text 通路 + attachments** —— 这个组合按上面两行的交集推导而来
(保留 `selected_sources`、不写 `system_hints`、附件形状照抄 image 通路),
理论上正确,但**没有 HAR 实证**。形状不对时上游不会报错,
而是下发一条 `is_visually_hidden_from_conversation=true` 的空 system message,
表现为"有 SSE 事件但正文为空"。

网关侧已有兜底:这种情况会被判为 `upstream_empty_output`,返回 502 并全额退款,
不会把运维文案当正文返回。但用户体验仍是"发图就失败"。

## 开启前的验证步骤

1. 准备一个 **ChatGPT Plus/Team** 账号(免费号会被降级到 `auto`,干扰判断)。
2. 后台「系统设置 → 网关」把「图片输入(vision)」打开。
3. 发一条只有文字的请求,确认仍然正常 —— 确保没有误伤主路径。
4. 发一条带 `image_url` 的请求:

```bash
curl -sS $BASE/v1/chat/completions \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"model":"gpt-5","messages":[{"role":"user","content":[
        {"type":"text","text":"这张图里有什么?"},
        {"type":"image_url","image_url":{"url":"data:image/png;base64,'"$(base64 -i test.png)"'"}}]}]}'
```

5. 判读结果:
   - 正常描述图片内容 → 通路成立,可以保持开启
   - `502 upstream_empty_output` → 形状被上游拒了,**关掉开关**,
     去抓一次浏览器发图的 HAR,对照 `internal/upstream/chatgpt/fchat.go` 的
     `buildTextMessage` 修正 metadata,再重试
   - `400 图片输入处理失败` → 是上传或解码环节的问题,看日志里的具体原因

## 限制

- 单张 ≤ 20MB,单次请求 ≤ 4 张(与图生图路径一致)
- 支持 `data:image/...;base64,...` 与 `http(s)://`
- 远程 URL 用独立的 `http.DefaultClient` 拉取:这是"按调用方给的 URL 取图",
  不带任何账号凭据、不走账号代理
- 任何一张图失败都整体报错,不做"少一张图继续"的降级 ——
  少一张图的回答是错的回答

## 相关代码

- `internal/gateway/vision.go` —— 下载、解码、上传
- `internal/upstream/chatgpt/fchat.go` 的 `buildTextMessage` —— payload 形状(**改这里前先抓包**)
- `internal/gateway/params.go` 的 `blockedParts` —— 开关如何影响校验
