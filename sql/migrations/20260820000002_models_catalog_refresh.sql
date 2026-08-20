-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- 2026-08-20 模型目录刷新
--
-- 目录上一次更新是 2026-04-18,四个月没动。本次只做**不涉及定价决策**的
-- 兼容性修复,价格一律不改(见文末说明)。
-- ============================================================

-- ------------------------------------------------------------
-- 1) 官方在产 slug 的别名(U26)
--
-- gpt-image-1 此前被整条 enabled=0 掉了。它到 2026-10-23 才关停,
-- 存量客户端与几乎所有教程写的都是这个名字;直接禁用的结果是
-- 一大批"照着官方文档写的代码"在本网关上跑不通,且报错是"模型不存在"。
-- 这里把它和 gpt-image-1-mini 作为别名指回同一个上游能力。
-- 价格与 gpt-image-2 保持一致(用子查询取,不写死数字)。
-- ------------------------------------------------------------
INSERT INTO `models`
    (`slug`, `type`, `upstream_model_slug`,
     `input_price_per_1m`, `output_price_per_1m`, `image_price_per_call`,
     `description`, `enabled`)
SELECT 'gpt-image-1', 'image', m.`upstream_model_slug`,
       0, 0, m.`image_price_per_call`,
       '别名 → gpt-image-2(兼容官方在产 slug,能力与计费完全一致)', 1
  FROM `models` m WHERE m.`slug` = 'gpt-image-2'
ON DUPLICATE KEY UPDATE
    `type`                 = VALUES(`type`),
    `upstream_model_slug`  = VALUES(`upstream_model_slug`),
    `image_price_per_call` = VALUES(`image_price_per_call`),
    `description`          = VALUES(`description`),
    `enabled`              = 1;

INSERT INTO `models`
    (`slug`, `type`, `upstream_model_slug`,
     `input_price_per_1m`, `output_price_per_1m`, `image_price_per_call`,
     `description`, `enabled`)
SELECT 'gpt-image-1-mini', 'image', m.`upstream_model_slug`,
       0, 0, m.`image_price_per_call`,
       '别名 → gpt-image-2(上游不区分档位,计费与 gpt-image-2 一致)', 1
  FROM `models` m WHERE m.`slug` = 'gpt-image-2'
ON DUPLICATE KEY UPDATE
    `upstream_model_slug`  = VALUES(`upstream_model_slug`),
    `image_price_per_call` = VALUES(`image_price_per_call`),
    `description`          = VALUES(`description`),
    `enabled`              = 1;

-- ------------------------------------------------------------
-- 2) 下架从未验证过的 codex slug(U14)
--
-- gpt-5-codex-max 是 init_schema 里的种子,不在 20260418 的扩展表里,没有任何
-- HAR 佐证。它还有一个副作用:langchain-openai 对**任何含 codex 的模型名**
-- 强制路由到 /v1/responses(_model_prefers_responses_api),用户无法覆盖。
-- 本网关现在有 responses 垫片了,但一个从未验证过的 slug 留在下拉框里
-- 只会制造"选了就报错"的困惑。
-- ------------------------------------------------------------
UPDATE `models`
   SET `enabled` = 0,
       `description` = '[unverified] 未经抓包验证的 slug,如需启用请先确认上游可用'
 WHERE `slug` = 'gpt-5-codex-max';

-- ------------------------------------------------------------
-- 3) 标注已知失效的上游 slug(U12)
--
-- 刻意**不**改 enabled:是否下架取决于运维对自己账号池的实测,
-- 这里只把已知情况写进 description,让后台看得见。
-- 失效 slug 的表现是上游静默拒绝,会被网关的兜底文案报成"限流",
-- 排查方向完全错误 —— 所以必须留痕。
-- ------------------------------------------------------------
UPDATE `models`
   SET `description` = CONCAT(`description`, ' [请实测确认:该 slug 可能已从 ChatGPT 网页版退役]')
 WHERE `slug` IN ('o3', 'gpt-5-1')
   AND `description` NOT LIKE '%请实测确认%';

-- ============================================================
-- 关于定价:本次刻意不动任何价格数字。
--
-- 现状是两份 seed 迁移的换算注释互相矛盾:
--   20260418000002 说 1 积分 = 0.0001 元
--   20260417000002 说 1 积分 = 0.01 元(差 100 倍)
-- 按前者口径,一张图(500000)= 6.67M 个 gpt-5 output token(75000/1M)。
--
-- 哪个是本意只有运营方知道,而改价直接影响真实账单,不能由迁移替用户决定。
-- 换算口径的唯一真源已写进 docs/spec/pricing.md,请先定口径再调价。
-- ============================================================

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE `models` SET `enabled` = 0 WHERE `slug` IN ('gpt-image-1', 'gpt-image-1-mini');
UPDATE `models` SET `enabled` = 1 WHERE `slug` = 'gpt-5-codex-max';

-- +goose StatementEnd
