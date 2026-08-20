// Package ratelimit 封装 API Key / 用户分组的 RPM / TPM 限流。
//
// 两层分桶:
//
//	rpm:<key_id>   - 每分钟请求次数(capacity=RPM)
//	tpm:<key_id>   - 每分钟 token 额度(capacity=TPM)
//
// RPM 在网关入口预检;TPM 先按估算值扣,结算时按实际值"补差"。
// 依赖 pkg/ratelimit 的 TokenBucket 原语。
package ratelimit

import (
	"context"
	"fmt"
	"time"

	pkgrl "github.com/jiji262/gpt2api/pkg/ratelimit"
)

// Scope 是限流的分桶维度。
//
// 必须与"额度从哪来"一致:额度来自用户分组时按 key 分桶,
// 同一用户建 N 把 key 就能拿到 N 倍分组限额 —— 分组限流形同虚设。
type Scope struct {
	KeyID  uint64
	UserID uint64
	// ByUser 为真表示当前额度来自用户分组,应按 user 分桶。
	ByUser bool
}

// bucket 返回该 scope 对应的 Redis key 前缀部分。
func (s Scope) bucket() string {
	if s.ByUser {
		return fmt.Sprintf("u:%d", s.UserID)
	}
	return fmt.Sprintf("k:%d", s.KeyID)
}

// Limiter 是限流服务。
type Limiter struct {
	tb *pkgrl.TokenBucket
}

func New(tb *pkgrl.TokenBucket) *Limiter { return &Limiter{tb: tb} }

// Result 是一次限流判定的完整结果,用于生成 x-ratelimit-* 响应头。
//
// 此前调用方一律用 `_` 丢弃 remaining,导致全站没有任何限流头:
// openai-python / openai-node 靠这些头做预测性退避,拿不到就只能撞上 429 再退避。
type Result struct {
	Allowed   bool
	Limit     int64
	Remaining int64
	// ResetAfter 是桶回满到够下一次请求所需的时间。
	ResetAfter time.Duration
	// Degraded 为真表示 Redis 出错、本次是放行兜底,不是真的有额度。
	Degraded bool
}

// unlimited 是 capacity<=0(不限流)时的结果。
func unlimited() Result { return Result{Allowed: true} }

// resultOf 把令牌桶原语的返回值组装成 Result。
func resultOf(allowed bool, remaining float64, capacity, cost int64) Result {
	refill := float64(capacity) / 60.0
	r := Result{
		Allowed:   allowed,
		Limit:     capacity,
		Remaining: int64(remaining),
	}
	if r.Remaining < 0 {
		r.Remaining = 0
	}
	if !allowed && refill > 0 {
		need := float64(cost) - remaining
		if need < 0 {
			need = 0
		}
		r.ResetAfter = time.Duration(need/refill*float64(time.Second)) + time.Second
	}
	return r
}

// AllowRPM 消费 1 个 RPM 令牌。capacity<=0 表示不限。
//
// Redis 出错时放行(fail-open):限流器本身不该成为网关的单点故障。
// 但必须把 Degraded 标出来并在调用方打 Error 日志 —— 此前是静默放行,
// Redis 一挂,RPM/TPM 全失效且日志里一个字都没有。
func (l *Limiter) AllowRPM(ctx context.Context, sc Scope, capacity int) Result {
	if capacity <= 0 {
		return unlimited()
	}
	key := "rl:rpm:" + sc.bucket()
	ok, remaining, err := l.tb.Allow(ctx, key, int64(capacity), 1, float64(capacity)/60.0)
	if err != nil {
		return Result{Allowed: true, Limit: int64(capacity), Degraded: true}
	}
	return resultOf(ok, remaining, int64(capacity), 1)
}

// AllowTPM 预扣 tokens,若额度不足返回 Allowed=false。capacity<=0 不限。
func (l *Limiter) AllowTPM(ctx context.Context, sc Scope, capacity, tokens int64) Result {
	if capacity <= 0 || tokens <= 0 {
		return unlimited()
	}
	key := "rl:tpm:" + sc.bucket()
	ok, remaining, err := l.tb.Allow(ctx, key, capacity, tokens, float64(capacity)/60.0)
	if err != nil {
		return Result{Allowed: true, Limit: capacity, Degraded: true}
	}
	return resultOf(ok, remaining, capacity, tokens)
}

// AdjustTPM 结算时对 TPM 做补差:delta>0 扣更多、delta<0 还回去。
//
// delta<0 分支此前是空实现(两行占位):入口按 max_tokens 或默认 2048 预扣,
// 真实输出常常只有几十 token,不还回去等于长期按最坏情况限流,
// 用户会莫名其妙地被 TPM 挡住。补扣/归还失败不报错,下一分钟桶过期会自愈。
func (l *Limiter) AdjustTPM(ctx context.Context, sc Scope, capacity, delta int64) {
	if capacity <= 0 || delta == 0 {
		return
	}
	key := "rl:tpm:" + sc.bucket()
	refill := float64(capacity) / 60.0
	if delta > 0 {
		_, _, _ = l.tb.Allow(ctx, key, capacity, delta, refill)
		return
	}
	_, _ = l.tb.Refund(ctx, key, capacity, -delta, refill)
}
