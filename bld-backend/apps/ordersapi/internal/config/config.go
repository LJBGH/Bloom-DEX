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

	// SnowflakeNode 雪花算法节点
	SnowflakeNode int `json:"SnowflakeNode"`
}

type KafkaConfig struct {
	Brokers    []string
	Topic      string
	Partitions int32
}
