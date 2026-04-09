package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"

	"bld-backend/apps/gateway/internal/config"

	"golang.org/x/time/rate"
)

// 限流配置
type limiterConfig struct {
	limit rate.Limit
	burst int
}

// 限流器存储
type limiterStore struct {
	cfg limiterConfig
	mu  sync.Mutex
	m   map[string]*rate.Limiter
}

// 创建限流器存储
func newLimiterStore(cfg limiterConfig) *limiterStore {
	return &limiterStore{
		cfg: cfg,
		m:   make(map[string]*rate.Limiter),
	}
}

// 获取限流器
func (s *limiterStore) get(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if lim, ok := s.m[key]; ok {
		return lim
	}
	lim := rate.NewLimiter(s.cfg.limit, s.cfg.burst)
	s.m[key] = lim
	return lim
}

// RateLimitWithConf 内存令牌桶限流（单机）。
func RateLimitWithConf(conf config.RateLimitConf) func(http.HandlerFunc) http.HandlerFunc {
	conf = conf.Normalize()
	defaultStore := newLimiterStore(limiterConfig{limit: rate.Limit(conf.Default.Rate), burst: conf.Default.Burst})
	strictStore := newLimiterStore(limiterConfig{limit: rate.Limit(conf.Strict.Rate), burst: conf.Strict.Burst})

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

			path := r.URL.Path
			// 只限制 API 流量 (网关只用于提供 /api/* 服务)
			if !strings.HasPrefix(path, "/api/") {
				next(w, r)
				return
			}

			// 获取客户端 IP 和路由分组
			ip := clientIP(r)
			group := routeGroup(path)

			// 选择限流器存储
			store := defaultStore
			if group == "ordersapi.order_write" || (group == "ordersapi.orders" && r.Method == http.MethodPost) {
				store = strictStore
			}
			// 构建限流器键
			key := ip + "|" + group
			// 检查是否允许请求
			if !store.get(key).Allow() {
				write429(w, group)
				return
			}

			next(w, r)
		}
	}
}

// RateLimit 保持旧入口（默认参数），方便手工启用。
func RateLimit(next http.HandlerFunc) http.HandlerFunc {
	return RateLimitWithConf(config.RateLimitConf{Mode: "memory"})(next)
}

// 路由分组
func routeGroup(path string) string {
	// 保持分组粗略; 避免巨大的限流器映射大小
	switch {
	case strings.HasPrefix(path, "/api/ordersapi/v1/spot/orders/cancel"):
		return "ordersapi.order_write"
	case strings.HasPrefix(path, "/api/ordersapi/v1/spot/orders"):
		// 两者都共享路径; 写入操作在中间件选择中严格按方法处理
		return "ordersapi.orders"
	case strings.HasPrefix(path, "/api/ordersapi/v1/spot/trades"):
		return "ordersapi.trade"
	case strings.HasPrefix(path, "/api/userapi/"):
		return "userapi"
	case strings.HasPrefix(path, "/api/walletapi/"):
		return "walletapi"
	case strings.HasPrefix(path, "/api/marketws/"):
		return "marketws"
	default:
		return "api.other"
	}
}

// 获取客户端 IP
func clientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		return xr
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

// 写入 429 响应
func write429(w http.ResponseWriter, group string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    429,
		"message": "rate limited",
		"group":   group,
	})
}
