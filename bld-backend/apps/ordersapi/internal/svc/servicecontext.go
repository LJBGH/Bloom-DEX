// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	walletpb "bld-backend/api/wallet"
	"bld-backend/apps/ordersapi/internal/config"
	"bld-backend/apps/ordersapi/internal/model"
	"bld-backend/apps/ordersapi/internal/mq"
	"bld-backend/core/util/snowflake"
	"hash/fnv"
	"os"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config           config.Config
	SpotOrderModel   model.SpotOrderModel
	SpotTradeModel   model.SpotTradeModel
	SpotMarketModel  model.SpotMarketModel
	KafkaProducer   mq.KafkaSpotOrderProducer
	IDGen           *snowflake.Generator

	Wallet walletpb.WalletClient // Wallet 为 nil 表示未配置或初始化失败，下单时会拒绝。
}

// NewServiceContext 创建服务上下文
func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	// 创建 Kafka 生产者
	producer, err := mq.NewSaramaSpotOrderProducer(c.Kafka.Brokers)
	if err != nil {
		// 如果 Kafka 生产者创建失败，则设置为 nil
		producer = nil
	}

	// 创建雪花算法生成器
	node := c.SnowflakeNode
	if node < 0 || node > 1023 {
		node = 0
	}
	// 如果雪花算法节点为 0，则使用 hostname + pid 生成节点
	if node == 0 {
		// 使用 hostname + pid 生成节点
		h := fnv.New32a()
		host, _ := os.Hostname()
		_, _ = h.Write([]byte(host))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(os.Getenv("COMPUTERNAME")))
		sum := h.Sum32()
		node = int(sum%1023) + 1
	}
	// 创建雪花算法生成器
	idgen, _ := snowflake.New(node, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	// 创建钱包客户端
	var walletCli walletpb.WalletClient
	// 如果钱包 RPC 配置正确，则创建钱包客户端
	if walletRPCConfigured(c.WalletRpc) {
		// 创建钱包 RPC 客户端
		wconn, err := zrpc.NewClient(c.WalletRpc)
		if err != nil {
			logx.Errorf("wallet zrpc client init failed: %v", err)
		} else {
			walletCli = walletpb.NewWalletClient(wconn.Conn())
		}
	}

	return &ServiceContext{
		Config:           c,
		SpotOrderModel:   model.NewSpotOrderModel(conn),
		SpotTradeModel:   model.NewSpotTradeModel(conn),
		SpotMarketModel:  model.NewSpotMarketModel(conn),
		KafkaProducer:   producer,
		IDGen:           idgen,
		Wallet:          walletCli,
	}
}

func walletRPCConfigured(c zrpc.RpcClientConf) bool {
	if len(c.Endpoints) > 0 || strings.TrimSpace(c.Target) != "" {
		return true
	}
	return len(c.Etcd.Hosts) > 0 && strings.TrimSpace(c.Etcd.Key) != ""
}
