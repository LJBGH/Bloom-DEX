package config

type (
	// 限流配置
	RateLimitConf struct {
		// 模式: "off" | "memory" | "redis"
		Mode string

		// 默认限流配置
		Default RateSpec
		// 严格限流配置
		Strict RateSpec

		// KeyPrefix 用于 redis 限流器构建键
		KeyPrefix string
	}

	// 限流规格
	RateSpec struct {
		// 速率: 每秒请求数
		Rate int
		// 突发: 最大并发请求数
		Burst int
	}
)

// Normalize 规范化限流配置
func (c RateLimitConf) Normalize() RateLimitConf {
	if c.Default.Rate <= 0 {
		c.Default.Rate = 50
	}
	if c.Default.Burst <= 0 {
		c.Default.Burst = 100
	}
	if c.Strict.Rate <= 0 {
		c.Strict.Rate = 10
	}
	if c.Strict.Burst <= 0 {
		c.Strict.Burst = 20
	}
	if c.KeyPrefix == "" {
		c.KeyPrefix = "gw:rl:v1"
	}
	if c.Mode == "" {
		c.Mode = "redis"
	}
	return c
}
