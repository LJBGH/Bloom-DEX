// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"github.com/zeromicro/go-zero/rest"

	"bld-backend/core/util/etcdreg"
)

type Config struct {
	rest.RestConf
	Etcd          etcdreg.Config

	// UserRestUrl is the base url for user service REST endpoints.
	// Example: http://127.0.0.1:9006
	UserRestUrl string
	// WalletCoreRestUrl is the base url for wallet service REST endpoints.
	// Example: http://127.0.0.1:9008
	WalletCoreRestUrl string
}
