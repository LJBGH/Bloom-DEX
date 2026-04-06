package config

import (
	"bld-backend/core/util/etcdreg"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Config 配置。
type Config struct {
	Name  string
	Log   logx.LogConf
	Mysql sqlx.SqlConf
	Etcd  etcdreg.Config
	Kafka KafkaConf
	// WalletRpc 现货成交后同步调钱包结算；Target 为空则跳过（例如 127.0.0.1:9101）。
	WalletRpc WalletRpcConf `json:",optional"`
}

// WalletRpcConf 钱包 gRPC。
type WalletRpcConf struct {
	Target string `json:",optional"`
}

// KafkaConf 配置 Kafka。
type KafkaConf struct {
	Brokers    []string
	Topic      string
	GroupID    string
	DepthTopic string `json:",optional"`
	TradeTopic string `json:",optional"`
	Partitions int    `json:",default=8"`
}
