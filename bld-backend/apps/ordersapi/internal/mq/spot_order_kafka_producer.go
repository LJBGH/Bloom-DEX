package mq

import (
	"context"
	"encoding/json"

	"bld-backend/core/model"

	"github.com/IBM/sarama"
	"github.com/zeromicro/go-zero/core/trace"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc/metadata"
)

// KafkaSpotOrderProducer 现货订单 Kafka 生产者接口。
type KafkaSpotOrderProducer interface {
	Publish(ctx context.Context, topic string, partition int32, key string, msg *model.SpotOrderKafkaMsg) error
}

type saramaSpotOrderProducer struct {
	producer sarama.SyncProducer
}

// NewSaramaSpotOrderProducer 创建 Sarama 同步生产者。
func NewSaramaSpotOrderProducer(brokers []string) (KafkaSpotOrderProducer, error) {
	// 创建一个 Kafka 配置
	cfg := sarama.NewConfig()
	// 设置生产者确认机制
	cfg.Producer.RequiredAcks = sarama.WaitForLocal
	// 设置生产者重试次数
	cfg.Producer.Retry.Max = 3
	// 设置生产者返回成功
	cfg.Producer.Return.Successes = true
	// 与部署集群对齐：confluentinc/cp-kafka:7.4.x ≈ Apache Kafka 3.4
	cfg.Version = sarama.V3_4_0_0
	// 创建一个 Kafka 同步生产者

	p, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return nil, err
	}
	return &saramaSpotOrderProducer{producer: p}, nil
}

// Publish 发布 Kafka 订单消息。
func (p *saramaSpotOrderProducer) Publish(
	ctx context.Context,
	topic string,
	partition int32,
	key string,
	msg *model.SpotOrderKafkaMsg,
) error {
	// 以 HTTP/RPC handler 的 span 上下文为父 span，在 producer 侧再拆一个子 span
	// 用于观测 "订单创建/撤单 -> Kafka 生产" 的链路耗时与失败原因。
	tracer := trace.TracerFromContext(ctx)
	// 创建一个子 span
	_, span := tracer.Start(ctx, "kafka.produce")
	defer span.End()

	if topic != "" {
		span.SetAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", topic),
			attribute.Int("messaging.kafka.partition", int(partition)),
			attribute.String("messaging.kafka.message_key", key),
		)
	}
	if msg != nil {
		span.SetAttributes(
			attribute.Int64("app.order_id", int64(msg.OrderID)),
			attribute.Int64("app.user_id", int64(msg.UserID)),
		)
	}

	value, err := json.Marshal(msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	headers := buildTraceHeaders(ctx)

	_, _, err = p.producer.SendMessage(&sarama.ProducerMessage{
		Topic:     topic,
		Partition: partition,
		Key:       sarama.StringEncoder(key),
		Value:     sarama.ByteEncoder(value),
		Headers:   headers,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return err
}

// buildTraceHeaders 构建 trace 头信息。
func buildTraceHeaders(ctx context.Context) []sarama.RecordHeader {
	if ctx == nil {
		return nil
	}

	// 构建 metadata 头信息
	md := metadata.New(nil)
	// 注入 trace 头信息
	trace.Inject(ctx, otel.GetTextMapPropagator(), &md)
	if len(md) == 0 {
		return nil
	}

	// 构建 sarama 头信息
	headers := make([]sarama.RecordHeader, 0, len(md))
	for key, values := range md {
		for _, value := range values {
			headers = append(headers, sarama.RecordHeader{
				Key:   []byte(key),
				Value: []byte(value),
			})
		}
	}

	return headers
}
