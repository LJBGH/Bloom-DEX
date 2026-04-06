package mq

import (
	"context"

	"github.com/IBM/sarama"
)

// DepthPublisher 推送订单簿快照到 Kafka。
type DepthPublisher interface {
	Publish(ctx context.Context, topic string, partition int32, key string, value []byte) error
}

type saramaDepthProducer struct {
	producer sarama.SyncProducer
}

// NewDepthProducer 创建深度快照同步生产者。
func NewDepthProducer(brokers []string) (DepthPublisher, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForLocal
	cfg.Producer.Retry.Max = 3
	cfg.Producer.Return.Successes = true
	cfg.Version = sarama.V3_4_0_0
	p, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return nil, err
	}
	return &saramaDepthProducer{producer: p}, nil
}

// Publish 推送订单簿快照到 Kafka。
func (p *saramaDepthProducer) Publish(_ context.Context, topic string, partition int32, key string, value []byte) error {
	_, _, err := p.producer.SendMessage(&sarama.ProducerMessage{
		Topic:     topic,
		Partition: partition,
		Key:       sarama.StringEncoder(key),
		Value:     sarama.ByteEncoder(value),
	})
	return err
}
