package config

type (
	// RateLimitConf controls gateway rate limiting behavior.
	RateLimitConf struct {
		// Mode: "off" | "memory" | "redis"
		Mode string `json:",default=redis,options=off|memory|redis"`

		// Default applies to most endpoints.
		Default RateSpec
		// Strict applies to write endpoints like create/cancel order.
		Strict RateSpec

		// KeyPrefix is used by redis limiter to build keys.
		KeyPrefix string `json:",default=gw:rl:v1"`
	}

	RateSpec struct {
		Rate  int `json:",default=50"`
		Burst int `json:",default=100"`
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
