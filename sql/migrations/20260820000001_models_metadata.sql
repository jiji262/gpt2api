-- +goose Up
-- 给 models 表补客户端选型用的元数据。
--
-- 值未知时留 0:上游的真实上下文窗口取决于 ChatGPT 账号档位(Free/Plus/Team/Pro),
-- 网关探测不到,照抄官方 Platform 的数字会误导调用方按错误的上限切分 prompt。
-- 管理员抓包确认后可在后台按模型填写。
ALTER TABLE models
  ADD COLUMN context_window INT NOT NULL DEFAULT 0 COMMENT '上下文窗口 token 数,0=未知',
  ADD COLUMN max_output_tokens INT NOT NULL DEFAULT 0 COMMENT '单次最大输出 token 数,0=未知';

-- +goose Down
ALTER TABLE models
  DROP COLUMN context_window,
  DROP COLUMN max_output_tokens;
