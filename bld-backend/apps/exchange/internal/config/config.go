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
	WalletRpc WalletRpcConf
	WAL       WALConf
}

// WalletRpcConf 钱包 gRPC。
type WalletRpcConf struct {
	Target string
}

// WALConf 订单簿持久化配置。
type WALConf struct {
	Enabled         bool   // 是否启用订单簿持久化
	Path            string // 订单簿持久化路径
	CheckpointPath  string // WAL checkpoint 路径（记录已回放 LSN）
	Format          string // WAL 格式：binary
	StrictRecovery  bool   // 仅回放完整事务批次
	FlushIntervalMs int    // 订单簿持久化刷新间隔
	QueueSize       int    // 订单簿持久化队列大小
}

// KafkaConf 配置 Kafka。
type KafkaConf struct {
	Brokers    []string
	Topic      string
	GroupID    string
	DepthTopic string
	TradeTopic string
	Partitions int
}
