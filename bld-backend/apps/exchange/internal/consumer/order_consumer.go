package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bld-backend/apps/exchange/internal/matcher"
	"bld-backend/core/model"

	"github.com/IBM/sarama"
	"github.com/zeromicro/go-zero/core/logx"

	ztrace "github.com/zeromicro/go-zero/core/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
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
			// 反序列化 Kafka 消息。
			var m model.SpotOrderKafkaMsg
			// 反序列化 Kafka 消息。
			if err := json.Unmarshal(msg.Value, &m); err != nil {
				logx.Errorf("exchange kafka unmarshal: %v", err)
				sess.MarkMessage(msg, "")
				continue
			}

			parentCtx := sess.Context()
			kctx, span := startKafkaConsumeSpan(parentCtx, msg)
			logx.Info("trace_id:", span.SpanContext().TraceID().String())
			logx.Info("span_id:", span.SpanContext().SpanID().String())
			logx.Info("flags:", span.SpanContext().TraceFlags().String())

			if err := h.Engine.HandleKafkaMessage(kctx, &m); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				span.End()
				logx.Errorf("exchange match order_id=%d: %v", m.OrderID, err)
				continue
			}
			span.SetStatus(codes.Ok, "ok")
			sess.MarkMessage(msg, "")
			span.End()
		case <-sess.Context().Done():
			return nil
		}
	}
}

// startKafkaConsumeSpan 启动 Kafka 订单消费 span。
func startKafkaConsumeSpan(ctx context.Context, msg *sarama.ConsumerMessage) (context.Context, trace.Span) {
	// 构建 grpc metadata 头信息
	md := metadata.New(nil)
	// 注入 trace 头信息
	for _, h := range msg.Headers {
		if len(h.Key) == 0 {
			continue
		}
		// grpc metadata 键名内部是小写
		key := strings.ToLower(string(h.Key))
		md.Set(key, string(h.Value))
	}

	// 提取 trace 头信息
	bags, spanCtx := ztrace.Extract(ctx, otel.GetTextMapPropagator(), &md)
	// 注入 baggage 头信息
	ctx = baggage.ContextWithBaggage(ctx, bags)
	// 创建一个 tracer
	tr := otel.Tracer(ztrace.TraceName)
	// 创建一个 span 名称
	spanName := "kafka.consume"
	if spanCtx.IsValid() {
		ctx = trace.ContextWithRemoteSpanContext(ctx, spanCtx)
	}

	kafkaAttrs := []attribute.KeyValue{
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", msg.Topic),
		attribute.Int("messaging.kafka.partition", int(msg.Partition)),
	}

	// NOTE: 使用 Consumer span kind 所以 tracing UIs 放在右侧
	return tr.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(kafkaAttrs...),
	)
}

// 启动 Kafka 订单消费者组。
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
