// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"bld-backend/core/util/etcdreg"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Mysql sqlx.SqlConf
	Etcd  etcdreg.Config

	// WalletRpc 连接 walletapi 的 zrpc（etcd Key 与 walletapi.yaml 中 Rpc.Etcd.Key 一致，如 walletapi.rpc）。
	WalletRpc zrpc.RpcClientConf `json:"WalletRpc"`

	Kafka KafkaConfig

	// Redis 用于 Bloom/幂等等（可选，不配置则禁用相关能力）。
	Redis RedisConf `json:"Redis"`
	// Bloom 配置（可选）。
	Bloom BloomConf `json:"Bloom"`

	// SnowflakeNode 雪花算法节点
	SnowflakeNode int `json:"SnowflakeNode"`
}

type KafkaConfig struct {
	Brokers    []string
	Topic      string
	Partitions int32
}

type RedisConf struct {
	Addr     string `json:"Addr"`
	Password string `json:"Password,omitempty"`
	DB       int    `json:"DB,omitempty"`
}

type BloomConf struct {
	// OrdersKey Redis key，用来存订单 ID 的 Bloom bitset。
	OrdersKey string `json:"OrdersKey,omitempty"`
	// ExpectedInsertions 预估插入数量（影响 bitset 大小）。
	ExpectedInsertions uint64 `json:"ExpectedInsertions,omitempty"`
	// FalsePositiveRate 误判率（如 0.01）。
	FalsePositiveRate float64 `json:"FalsePositiveRate,omitempty"`
}
