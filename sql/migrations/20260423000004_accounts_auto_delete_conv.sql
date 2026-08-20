-- +goose Up
-- +goose StatementBegin

-- oai_accounts 扩展:生图成功后是否自动在 chatgpt.com 侧"删除"(PATCH is_visible=false)
-- 新建的对话,避免用户自己 chatgpt.com 左侧列表里堆积大量 "IMG2 xxx" 记录。
--
-- 实现细节:ChatGPT 网页点"删除"按钮,真实请求就是
--   PATCH /backend-api/conversation/{id}  body {"is_visible": false}
-- 后端 conversation 记录仍保留,attachment/file 下载端点继续工作,
-- 不影响本项目 24h 代理 URL(`/p/img/<task_id>/<idx>`)的 sediment 图再下载。

ALTER TABLE `oai_accounts`
    ADD COLUMN `auto_delete_conversation` TINYINT(1) NOT NULL DEFAULT 0 AFTER `daily_image_quota`;

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
ALTER TABLE `oai_accounts` DROP COLUMN `auto_delete_conversation`;
-- +goose StatementEnd
