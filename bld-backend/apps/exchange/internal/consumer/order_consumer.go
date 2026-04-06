package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"bld-backend/apps/exchange/internal/matcher"
	"bld-backend/core/model"

	"github.com/IBM/sarama"
	"github.com/zeromicro/go-zero/core/logx"
)

// OrderHandler 处理单条 Kafka 订单消息。
type OrderHandler struct {
	Engine *matcher.Engine
}

// Setup 在 ConsumerGroup 初始化时调用。
func (h *OrderHandler) Setup(_ sarama.ConsumerGroupSession) error { return nil }

// Cleanup 在 ConsumerGroup 关闭时调用。
func (h *OrderHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim 消费 Kafka 订单消息。
func (h *OrderHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			var m model.SpotOrderKafkaMsg
			if err := json.Unmarshal(msg.Value, &m); err != nil {
				logx.Errorf("exchange kafka unmarshal: %v", err)
				sess.MarkMessage(msg, "")
				continue
			}
			ctx := context.Background()
			if err := h.Engine.HandleKafkaMessage(ctx, &m); err != nil {
				logx.Errorf("exchange match order_id=%d: %v", m.OrderID, err)
				continue
			}
			sess.MarkMessage(msg, "")
		case <-sess.Context().Done():
			return nil
		}
	}
}

// StartConsumerGroup 阻塞直至 ctx 取消。
func StartConsumerGroup(ctx context.Context, brokers []string, groupID, topic string, eng *matcher.Engine) error {
	if len(brokers) == 0 {
		return fmt.Errorf("kafka brokers empty")
	}
	if topic == "" {
		return fmt.Errorf("kafka topic empty")
	}
	if groupID == "" {
		return fmt.Errorf("kafka group_id empty")
	}

	// 创建一个 Kafka 配置
	cfg := sarama.NewConfig()
	// 与部署集群对齐：confluentinc/cp-kafka:7.4.x ≈ Apache Kafka 3.4
	cfg.Version = sarama.V3_4_0_0
	// 设置平衡策略
	cfg.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	// 设置偏移量初始值
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	// 创建一个 Kafka 消费者组
	g, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return err
	}

	// 关闭 Kafka 消费者组
	defer func() { _ = g.Close() }()

	// 创建一个订单处理器
	handler := &OrderHandler{Engine: eng}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := g.Consume(ctx, []string{topic}, handler)
		if err != nil {
			return err
		}
		// Consume 正常返回表示本轮 session 结束；立即再调会空转、GC/TotalAlloc 暴涨
		time.Sleep(time.Second)
	}
}
