package config

import (
	"bld-backend/core/util/etcdreg"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type Config struct {
	Name   string
	Log    logx.LogConf
	Etcd   etcdreg.Config
	Mysql  sqlx.SqlConf
	EvmRPC string
	// 初始化扫描的区块高度，默认为0，即从创世块开始扫描。
	InitBlockHeight int64
	// RequiredConfirmations 是一个整数，表示一个区块被认为是最终的所需的确认数。默认为12。
	Confirmation int64
	// PollIntervalSeconds 是一个整数，表示区块监视器轮询区块链节点以检查新块的时间间隔（以秒为单位）。默认为10秒。
	PollIntervalSeconds int64
}
