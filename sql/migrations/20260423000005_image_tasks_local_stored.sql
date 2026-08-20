-- +goose Up
-- +goose StatementBegin

-- image_tasks 扩展:"生图成功后立即把 N 张图的字节落盘到本地"模式。
--
-- 背景:IMG2 回来的图片大部分是 sediment 引用(/backend-api/conversation/{cid}
-- /attachment/{sid}/download),这类 URL 一旦 conversation 被 hide(PATCH
-- is_visible=false)或原对话被删,服务端会直接 404。为了让"生图后删对话"功能
-- 可用,并且同时把 24h 代理 URL 变得不依赖 chatgpt 侧存活性,我们把所有生图
-- 的字节在 Runner 成功时就立刻下载并写到 /app/data/images/{task_id}/{idx}.{ext}。
--
-- local_stored=1 时,images_proxy 直接读本地文件,完全不回源 chatgpt。
-- local_content_type 记录实际 MIME(多是 image/png,偶有 image/webp 等)。

ALTER TABLE `image_tasks`
    ADD COLUMN `local_stored`       TINYINT(1)  NOT NULL DEFAULT 0 AFTER `result_urls`,
    ADD COLUMN `local_content_type` VARCHAR(64) NOT NULL DEFAULT '' AFTER `local_stored`;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
ALTER TABLE `image_tasks`
    DROP COLUMN `local_content_type`,
    DROP COLUMN `local_stored`;
-- +goose StatementEnd
