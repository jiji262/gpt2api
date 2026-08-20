package ratelimit

import (
	"testing"
	"time"
)

// TestResultOfComputesReset 覆盖 429 的 Retry-After 来源:
// 需要多少令牌、按当前回填速率要等多久,是服务端唯一知道的确切信息。
func TestResultOfComputesReset(t *testing.T) {
	// capacity=60/min → 1 token/s。剩 0 个,要 1 个 → 约 1s + 1s 保险。
	r := resultOf(false, 0, 60, 1)
	if r.Allowed {
		t.Fatal("应判定为拒绝")
	}
	if r.Limit != 60 {
		t.Errorf("Limit = %d", r.Limit)
	}
	if r.ResetAfter < time.Second || r.ResetAfter > 3*time.Second {
		t.Errorf("ResetAfter = %v, 期望 1~3s", r.ResetAfter)
	}
}

func TestResultOfAllowedHasNoReset(t *testing.T) {
	r := resultOf(true, 42, 60, 1)
	if !r.Allowed || r.Remaining != 42 {
		t.Fatalf("r = %#v", r)
	}
	if r.ResetAfter != 0 {
		t.Errorf("放行时不必给 reset: %v", r.ResetAfter)
	}
}

func TestResultOfClampsNegativeRemaining(t *testing.T) {
	// 令牌桶可能返回极小的负数浮点,不能让它变成负的 remaining 头。
	if r := resultOf(false, -0.5, 60, 1); r.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", r.Remaining)
	}
}

func TestUnlimitedIsAllowedWithoutLimit(t *testing.T) {
	r := unlimited()
	if !r.Allowed {
		t.Fatal("不限流应放行")
	}
	if r.Limit != 0 {
		t.Errorf("不限流时 Limit 应为 0(调用方据此不写头),实际 %d", r.Limit)
	}
	if r.Degraded {
		t.Error("不限流不是降级")
	}
}

// TestNilLimiterPathsAreSafe 保证 capacity<=0 的快路径不碰 Redis。
// Limiter 里 tb 为 nil 时仍必须能返回,否则未配 Redis 的部署直接 panic。
func TestUnlimitedShortCircuitsBeforeRedis(t *testing.T) {
	l := New(nil) // 故意不给 TokenBucket
	sc := Scope{KeyID: 1, UserID: 2}
	if r := l.AllowRPM(nil, sc, 0); !r.Allowed {
		t.Error("capacity=0 应直接放行且不碰 Redis")
	}
	if r := l.AllowTPM(nil, sc, 0, 100); !r.Allowed {
		t.Error("capacity=0 应直接放行且不碰 Redis")
	}
	if r := l.AllowTPM(nil, sc, 1000, 0); !r.Allowed {
		t.Error("tokens=0 应直接放行且不碰 Redis")
	}
	// delta=0 / capacity=0 都不该走到 Redis。
	l.AdjustTPM(nil, sc, 0, -100)
	l.AdjustTPM(nil, sc, 1000, 0)
}

// TestScopeBucketMatchesQuotaOwner 覆盖 U29:
// 额度来自用户分组时仍按 key 分桶的话,同一用户建 N 把 key
// 就拿到 N 倍分组限额,分组限流形同虚设。
func TestScopeBucketMatchesQuotaOwner(t *testing.T) {
	byKey := Scope{KeyID: 7, UserID: 42}
	byUser := Scope{KeyID: 7, UserID: 42, ByUser: true}

	if byKey.bucket() != "k:7" {
		t.Errorf("key 维度 = %q", byKey.bucket())
	}
	if byUser.bucket() != "u:42" {
		t.Errorf("user 维度 = %q", byUser.bucket())
	}
	// 同一用户的两把不同 key,在分组额度下必须落进同一个桶。
	other := Scope{KeyID: 99, UserID: 42, ByUser: true}
	if byUser.bucket() != other.bucket() {
		t.Errorf("同用户不同 key 应共用一个桶: %q vs %q", byUser.bucket(), other.bucket())
	}
	// 不同用户必须分开。
	third := Scope{KeyID: 99, UserID: 43, ByUser: true}
	if third.bucket() == byUser.bucket() {
		t.Error("不同用户不该共用桶")
	}
}
