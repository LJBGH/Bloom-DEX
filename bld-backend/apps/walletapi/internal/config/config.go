package config

import (
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"

	"bld-backend/core/util/etcdreg"
)

type Config struct {
	rest.RestConf
	// Rpc 与 Rest 同进程；ListenOn 为空时不启动 gRPC。
	Rpc zrpc.RpcServerConf `json:"Rpc"`
	Mysql sqlx.SqlConf
	Etcd  etcdreg.Config
	// CustodyMasterKey is a base64-encoded 32-byte key for AES-256-GCM.
	CustodyMasterKey string
	// EvmRPC is JSON-RPC endpoint for signing & sending transactions.
	EvmRPC string
	// HotWalletPrivateKey is a 0x-prefixed hex private key used to send withdrawals.
	HotWalletPrivateKey string
	// HotWalletAddress is the hot wallet receiver for token sweeps (optional; if empty it will be derived).
	HotWalletAddress string
}
