package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"bld-backend/core/util/etcdreg"
)

type Config struct {
	rest.RestConf
	Mysql sqlx.SqlConf
	Etcd  etcdreg.Config
}
