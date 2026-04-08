package config

import (
	"bld-backend/core/util/etcdreg"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type KafkaConf struct {
	Brokers      []string
	GroupID      string
	DepthTopic   string
	TradeTopic   string
	TradeGroupID string
}

type Config struct {
	Name     string
	Log      logx.LogConf
	Etcd     etcdreg.Config
	ListenOn string
	Kafka    KafkaConf
	Mysql    sqlx.SqlConf
}
