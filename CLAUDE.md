# gpt2api

把 `chatgpt.com` 网页版逆向包装成 OpenAI 兼容 API 的网关 + 计费 / 账号池 / 管理后台。
Go 1.26 + Gin + MySQL + Redis,前端 Vue 3 + Element Plus。

## 先读这些

@docs/spec/conventions.md
@docs/spec/pricing.md
@docs/spec/vision.md

对外协议的逐项对照表在 [docs/OPENAI_COMPAT.md](docs/OPENAI_COMPAT.md)。

## 一句话架构

```
客户端 → /v1/*(apikey 中间件)
       → gateway 参数三档校验
       → 模型解析 / 限流 / 预扣积分
       → scheduler 拿账号 lease
       → upstream/chatgpt 调 chatgpt.com
       → renderer 按 chat 或 responses 协议输出
       → 结算或退款 + usage_logs
```

`internal/gateway/chat.go` 的 `runChat` 是主管线,
`/v1/chat/completions` 与 `/v1/responses` 共用它,只有 `renderer` 不同。

## 常用命令

```bash
make test          # go test ./...
make build         # 编译到 bin/
make migrate-up    # goose up
```

## 最容易踩的三个坑

1. **静默失败**:上游做不到的东西必须明确报错。见 conventions 第 1 条。
2. **payload 形状**:改上游 payload 必须有 HAR 抓包证据,形状不对上游不报错只静默拒绝。见第 5 条。
3. **默认值判定**:`"tools":[]` 不等于要用工具。见第 2 条。
