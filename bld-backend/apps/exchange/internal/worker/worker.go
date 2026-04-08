package worker

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	walletpb "bld-backend/api/wallet"
	"bld-backend/apps/exchange/internal/config"
	"bld-backend/apps/exchange/internal/consumer"
	"bld-backend/apps/exchange/internal/matcher"
	"bld-backend/apps/exchange/internal/mq"
	"bld-backend/apps/exchange/internal/store"
	"bld-backend/apps/exchange/internal/wal"
	"bld-backend/apps/exchange/internal/wal/wal_analysis"
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

	// 创建深度生产者
	var depthPub mq.DepthPublisher
	if len(c.Kafka.Brokers) > 0 {
		p, err := mq.NewDepthProducer(c.Kafka.Brokers)
		if err != nil {
			logx.Errorf("depth kafka producer disabled: %v", err)
		} else {
			depthPub = p
		}
	}

	// 创建钱包客户端
	var wcli walletpb.WalletClient
	if t := strings.TrimSpace(c.WalletRpc.Target); t != "" {
		gconn, err := grpc.NewClient(t, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logx.Errorf("wallet grpc dial %q disabled: %v", t, err)
		} else {
			wcli = walletpb.NewWalletClient(gconn)
		}
	}

	// 创建订单簿持久化写入器
	var walWriter *wal.Writer
	if c.WAL.Enabled {
		w, err := wal.New(c.WAL.Path, time.Duration(c.WAL.FlushIntervalMs)*time.Millisecond, c.WAL.QueueSize)
		if err != nil {
			logx.Errorf("wal disabled due to init error: %v", err)
		} else {
			w.Start(ctx)
			walWriter = w
			defer func() { _ = w.Close() }()
		}
	}

	// 创建撮合引擎
	eng := matcher.New(st, depthPub, depthTopic, tradeTopic, c.Kafka.Partitions, wcli, walWriter)
	if c.WAL.Enabled && strings.TrimSpace(c.WAL.Path) != "" {
		ckptPath := strings.TrimSpace(c.WAL.CheckpointPath)
		if ckptPath == "" {
			ckptPath = c.WAL.Path + ".ckpt"
		}
		ckptPath = filepath.Clean(ckptPath)
		logx.Infof("exchange recovering order books from WAL only checkpoint=%s", ckptPath)
		lastLSN, err := wal_analysis.ReadCheckpoint(ckptPath)
		if err != nil {
			logx.Errorf("read wal checkpoint failed, fallback from lsn=0: %v", err)
			lastLSN = 0
		}
		replayed, maxLSN, err := replayWalPendingOrdersSince(ctx, eng, c.WAL.Path, lastLSN)
		if err != nil {
			if errors.Is(err, wal.ErrLegacyJSONWAL) {
				return err
			}
			logx.Errorf("wal replay skipped due to error: %v", err)
		} else if maxLSN > 0 {
			if err := wal_analysis.WriteCheckpoint(ckptPath, maxLSN); err != nil {
				logx.Errorf("write wal checkpoint failed: %v", err)
			}
			logx.Infof("exchange wal replay done replayed=%d lsn=%d->%d", replayed, lastLSN, maxLSN)
		} else {
			// WAL 为空或未产生有效 order 事件时，从 DB 恢复一次订单簿作为基线。
			logx.Info("wal empty, recovering order books from DB...")
			if err := eng.Recover(ctx); err != nil {
				return err
			}
		}
	} else {
		// 未启用 WAL 时，按 DB 恢复。
		logx.Info("exchange recovering order books from DB...")
		if err := eng.Recover(ctx); err != nil {
			return err
		}
	}
	// 消费 Kafka 订单消息
	logx.Infof("exchange consuming topic=%q group=%q", c.Kafka.Topic, c.Kafka.GroupID)
	// 启动 Kafka 消费
	return consumer.StartConsumerGroup(ctx, c.Kafka.Brokers, c.Kafka.GroupID, c.Kafka.Topic, eng) // 返回错误
}

// 回放 WAL 中自 checkpoint 之后的订单。
func replayWalPendingOrdersSince(ctx context.Context, eng *matcher.Engine, walPath string, fromLSN uint64) (int, uint64, error) {
	events, maxLSN, err := wal_analysis.LoadOrderEventsSince(walPath, fromLSN)
	if err != nil {
		return 0, maxLSN, err
	}
	if len(events) == 0 {
		return 0, maxLSN, nil
	}
	replayed := 0
	for _, ev := range events {
		raw := ev.Msg
		if raw == nil || raw.OrderID == 0 {
			continue
		}
		if err := eng.HandleWalReplayMessage(ctx, raw); err != nil {
			logx.Errorf("wal replay order_id=%d failed: %v", raw.OrderID, err)
			continue
		}
		replayed++
	}
	return replayed, maxLSN, nil
}
