package model

import (
	"database/sql"
	"time"
)

// 模型类型。
const (
	TypeChat  = "chat"
	TypeImage = "image"
)

// Model 对应 models 表。
type Model struct {
	ID                  uint64 `db:"id" json:"id"`
	Slug                string `db:"slug" json:"slug"`
	Type                string `db:"type" json:"type"`
	UpstreamModelSlug   string `db:"upstream_model_slug" json:"upstream_model_slug"`
	InputPricePer1M     int64  `db:"input_price_per_1m" json:"input_price_per_1m"`
	OutputPricePer1M    int64  `db:"output_price_per_1m" json:"output_price_per_1m"`
	CacheReadPricePer1M int64  `db:"cache_read_price_per_1m" json:"cache_read_price_per_1m"`
	ImagePricePerCall   int64  `db:"image_price_per_call" json:"image_price_per_call"`
	Description         string `db:"description" json:"description"`
	// ContextWindow / MaxOutputTokens 是给客户端选型用的元数据,0 表示未知。
	// 上游的真实上下文窗口取决于账号档位,拿不到就留 0,不照抄官方数字。
	ContextWindow   int          `db:"context_window" json:"context_window"`
	MaxOutputTokens int          `db:"max_output_tokens" json:"max_output_tokens"`
	Enabled         bool         `db:"enabled" json:"enabled"`
	CreatedAt       time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time    `db:"updated_at" json:"updated_at"`
	DeletedAt       sql.NullTime `db:"deleted_at" json:"-"`
}
