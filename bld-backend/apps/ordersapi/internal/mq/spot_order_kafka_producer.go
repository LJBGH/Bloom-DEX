package mq

import (
	"context"
	"encoding/json"

	"bld-backend/core/model"

	"github.com/IBM/sarama"
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
	_ context.Context,
	topic string,
	partition int32,
	key string,
	msg *model.SpotOrderKafkaMsg,
) error {
	value, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, _, err = p.producer.SendMessage(&sarama.ProducerMessage{
		Topic:     topic,
		Partition: partition,
		Key:       sarama.StringEncoder(key),
		Value:     sarama.ByteEncoder(value),
	})
	return err
}
