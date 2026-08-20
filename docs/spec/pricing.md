# 计费口径(唯一真源)

> 本文件是 `models` 表价格字段的换算口径真源。改价前先读这里。
> 起因:两份 seed 迁移的注释互相矛盾,导致 chat 与 image 的相对定价差了约三个数量级。

## 单位

`models` 表的四个价格字段全部是**整数积分**:

| 字段 | 含义 | 计价方式 |
|---|---|---|
| `input_price_per_1m` | 每 100 万输入 token | 按实际 token 数比例折算 |
| `output_price_per_1m` | 每 100 万输出 token | 同上 |
| `cache_read_price_per_1m` | 每 100 万缓存命中 token | 上游不提供缓存信息,当前恒不触发 |
| `image_price_per_call` | 每次成功出图 | 按次,与张数无关 |

**1 积分 = 0.0001 元(厘)。** 这是 `20260418000002_models_catalog_expand.sql` 采用的口径,
也是当前 16 个 chat 模型全部遵循的口径,因此定为准。

`20260417000002_seed_gpt_image_2.sql` 的注释写的是「1 积分 = 0.01 元」,**那句是错的**,
但它写下的数字 `image_price_per_call = 500000` 至今没人改过——所以现状是:

- 一次出图 = 500000 积分 = **50 元**
- 100 万 gpt-5 输出 token = 75000 积分 = **7.5 元**
- 即:一张图 ≈ 667 万个输出 token ≈ 6.67 次"打满 100 万 token 的对话"

## 待决策(未处理)

上面这个比例是不是本意,只有运营方知道。三种可能:

1. 图片本来就该 50 元/张 → 什么都不用改,但 chat 价格显得过低
2. 图片本意是 0.5 元/张 → `image_price_per_call` 应为 `5000`
3. chat 价格本意更高 → 调整 16 个 chat 模型的 input/output

**在定下口径之前不要改数字。** 迁移文件不替运营方做定价决策。

## 相关风险

`/v1/chat/completions` 路由始终开放,16 个 chat 模型照常出现在 `/v1/models`。
`ENABLE_CHAT_MODEL=false` 只影响 UI,不影响 API。
若当前 chat 定价确实偏低,任何持有 API Key 的人都能以接近免费的价格跑 chat。

## 改价方式

优先用后台「模型配置」页面改,不要新增迁移——迁移里的 `ON DUPLICATE KEY UPDATE`
刻意不覆盖 `price` 和 `enabled`,就是为了不冲掉人工调整。
