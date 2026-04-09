package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"bld-backend/apps/gateway/internal/config"

	"github.com/zeromicro/go-zero/core/limit"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// 令牌桶限流器存储
type tokenLimiterStore struct {
	rate  int
	burst int
	store *redis.Redis

	mu sync.Mutex
	m  map[string]*limit.TokenLimiter
}

// 创建令牌桶限流器存储
func newTokenLimiterStore(rate, burst int, store *redis.Redis) *tokenLimiterStore {
	return &tokenLimiterStore{
		rate:  rate,
		burst: burst,
		store: store,
		m:     make(map[string]*limit.TokenLimiter),
	}
}

// 获取令牌桶限流器
func (s *tokenLimiterStore) get(key string) *limit.TokenLimiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if lim, ok := s.m[key]; ok {
		return lim
	}
	lim := limit.NewTokenLimiter(s.rate, s.burst, s.store, key)
	s.m[key] = lim
	return lim
}

// RedisRateLimitWithConf 基于 go-zero 的 core/limit + core/stores/redis 实现分布式令牌桶限流。
// - 正常情况：使用 Redis 脚本做分布式 token bucket
// - Redis 异常：go-zero 内部会自动启用进程内 rescue limiter（不需要我们额外处理）
func RedisRateLimitWithConf(rds *redis.Redis, conf config.RateLimitConf) func(http.HandlerFunc) http.HandlerFunc {
	conf = conf.Normalize()
	algo := strings.ToLower(strings.TrimSpace(conf.Algorithm))

	// token bucket
	defaultToken := newTokenLimiterStore(conf.Default.Rate, conf.Default.Burst, rds)
	strictToken := newTokenLimiterStore(conf.Strict.Rate, conf.Strict.Burst, rds)

	// fixed window
	period := conf.PeriodSeconds
	if period <= 0 {
		period = 1
	}
	defaultPeriod := limit.NewPeriodLimit(period, conf.Default.Rate, rds, conf.KeyPrefix+":period:")
	strictPeriod := limit.NewPeriodLimit(period, conf.Strict.Rate, rds, conf.KeyPrefix+":period:")
	prefix := conf.KeyPrefix

	// 返回限流中间件
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r == nil || r.URL == nil {
				next(w, r)
				return
			}
			// 不限制网关自身的健康检查
			if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
				next(w, r)
				return
			}
			// 允许 CORS 预检请求
			if r.Method == http.MethodOptions {
				next(w, r)
				return
			}
			// 只限制 API 流量
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				next(w, r)
				return
			}

			ip := clientIP(r)
			group := routeGroup(r.URL.Path)
			strict := group == "ordersapi.order_write" || (group == "ordersapi.orders" && r.Method == http.MethodPost)
			// key 维度：(ip + group)。TokenLimiter 内部会把 key 映射为两条 redis key（tokens + ts），并用 {} 保证同槽。
			key := prefix + ":" + group + ":" + ip

			ctx := r.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			switch algo {
			case "period":
				// 固定窗口：每 period 秒最多 quota 次；Burst 不生效
				pl := defaultPeriod
				if strict {
					pl = strictPeriod
				}
				state, err := pl.TakeCtx(ctx, key)
				if err == nil && state == limit.OverQuota {
					write429(w, group)
					return
				}
			default:
				// 令牌桶
				store := defaultToken
				if strict {
					store = strictToken
				}
				if !store.get(key).AllowCtx(ctx) {
					write429(w, group)
					return
				}
			}

			next(w, r)
		}
	}
}

// RedisRateLimit 保持旧入口（默认参数），方便手工启用。
func RedisRateLimit(rds *redis.Redis) func(http.HandlerFunc) http.HandlerFunc {
	return RedisRateLimitWithConf(rds, config.RateLimitConf{Mode: "redis"})
}
