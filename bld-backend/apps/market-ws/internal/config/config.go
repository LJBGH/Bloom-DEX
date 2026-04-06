package config

import (
	"bld-backend/core/util/etcdreg"

	"github.com/zeromicro/go-zero/core/logx"
)

type KafkaConf struct {
	Brokers      []string
	GroupID      string `json:",default=market-ws-depth"`
	DepthTopic   string `json:",optional"`
	TradeTopic   string `json:",optional"`
	TradeGroupID string `json:",optional"`
}

type Config struct {
	Name     string `json:",default=market-ws"`
	Log      logx.LogConf
	Etcd     etcdreg.Config
	ListenOn string `json:",default=0.0.0.0:9201"`
	Kafka    KafkaConf
}
