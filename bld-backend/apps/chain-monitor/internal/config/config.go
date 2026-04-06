package config

import (
	"bld-backend/core/util/etcdreg"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type Config struct {
	Name   string `json:",default=chain-monitor"`
	Log    logx.LogConf
	Etcd   etcdreg.Config
	Mysql  sqlx.SqlConf
	EvmRPC string
	// InitBlockHeight is the first block number to scan when network_offsets doesn't exist.
	InitBlockHeight int64
	// Confirmation ensures only credited after (latest-confirmation) >= current scan end.
	Confirmation int64
	// PollIntervalSeconds is the sleep interval between scan loops.
	PollIntervalSeconds int64
}
