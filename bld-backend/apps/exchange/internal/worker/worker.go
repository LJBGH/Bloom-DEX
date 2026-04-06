package worker

import (
	"context"
	"strings"

	walletpb "bld-backend/api/wallet"
	"bld-backend/apps/exchange/internal/config"
	"bld-backend/apps/exchange/internal/consumer"
	"bld-backend/apps/exchange/internal/matcher"
	"bld-backend/apps/exchange/internal/mq"
	"bld-backend/apps/exchange/internal/store"
	"bld-backend/core/enum"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Run 加载订单簿、启动 Kafka 消费，在 ctx 取消时结束。
func Run(ctx context.Context, c config.Config) error {
	// 创建一个 MySQL 连接
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	// 创建一个 SpotStore
	st := store.NewSpotStore(conn)

	depthTopic := c.Kafka.DepthTopic
	if depthTopic == "" {
		depthTopic = enum.TopicDepthDelta
	}
	tradeTopic := c.Kafka.TradeTopic
	if tradeTopic == "" {
		tradeTopic = enum.TopicMarketTrade
	}
	var depthPub mq.DepthPublisher
	if len(c.Kafka.Brokers) > 0 {
		p, err := mq.NewDepthProducer(c.Kafka.Brokers)
		if err != nil {
			logx.Errorf("depth kafka producer disabled: %v", err)
		} else {
			depthPub = p
		}
	}

	var wcli walletpb.WalletClient
	if t := strings.TrimSpace(c.WalletRpc.Target); t != "" {
		gconn, err := grpc.NewClient(t, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logx.Errorf("wallet grpc dial %q disabled: %v", t, err)
		} else {
			wcli = walletpb.NewWalletClient(gconn)
		}
	}

	eng := matcher.New(st, depthPub, depthTopic, tradeTopic, c.Kafka.Partitions, wcli)
	// 恢复订单簿
	logx.Info("exchange recovering order books from DB...")
	// 恢复订单簿
	if err := eng.Recover(ctx); err != nil {
		return err
	}
	// 消费 Kafka 订单消息
	logx.Infof("exchange consuming topic=%q group=%q", c.Kafka.Topic, c.Kafka.GroupID)
	// 启动 Kafka 消费
	return consumer.StartConsumerGroup(ctx, c.Kafka.Brokers, c.Kafka.GroupID, c.Kafka.Topic, eng) // 返回错误
}
