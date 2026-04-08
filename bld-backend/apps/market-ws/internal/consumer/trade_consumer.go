package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"bld-backend/apps/market-ws/internal/kline"
	"bld-backend/apps/market-ws/internal/hub"
	"bld-backend/apps/market-ws/internal/wire"
	"bld-backend/core/model"

	"github.com/IBM/sarama"
	"github.com/zeromicro/go-zero/core/logx"
)

// TradeHandler 消费公开成交 Kafka，推送给订阅该市场的 WebSocket。
type TradeHandler struct {
	Hub   *hub.Hub
	Kline *kline.Store
}

func (h *TradeHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *TradeHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim 消费公开成交。
func (h *TradeHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			var m model.PublicTradeKafkaMsg
			if err := json.Unmarshal(msg.Value, &m); err != nil {
				logx.Errorf("market-ws trade unmarshal: %v", err)
				sess.MarkMessage(msg, "")
				continue
			}
			raw := append([]byte(nil), msg.Value...)
			env, err := wire.TradeEvent(raw)
			if err != nil {
				logx.Errorf("market-ws trade envelope: %v", err)
				sess.MarkMessage(msg, "")
				continue
			}
			h.Hub.Broadcast(m.MarketID, env)

			// 1m kline pre-aggregation + push
			if h.Kline != nil && m.MarketID > 0 {
				openTimeMs := (m.TsMs / 60000) * 60000
				if err := h.Kline.Upsert(sess.Context(), m.MarketID, "1m", openTimeMs, m.Price, m.Quantity); err != nil {
					logx.Errorf("market-ws kline upsert: %v", err)
				} else {
					row, err := h.Kline.Get(sess.Context(), m.MarketID, "1m", openTimeMs)
					if err != nil {
						logx.Errorf("market-ws kline get: %v", err)
						sess.MarkMessage(msg, "")
						continue
					}
					if row == nil {
						sess.MarkMessage(msg, "")
						continue
					}
					km := model.KlineWsMsg{
						MarketID:    row.MarketID,
						Interval:    row.Interval,
						OpenTimeMs:  row.OpenTimeMs,
						Open:        row.Open,
						High:        row.High,
						Low:         row.Low,
						Close:       row.Close,
						Volume:      row.Volume,
						Turnover:    row.Turnover,
						TradesCount: row.TradesCount,
						IsFinal:     false,
						TsMs:        time.Now().UnixMilli(),
					}
					if rawK, err := json.Marshal(km); err == nil {
						if envK, err := wire.KlineEvent(rawK); err == nil {
							h.Hub.Broadcast(m.MarketID, envK)
						}
					}
				}
			}
			sess.MarkMessage(msg, "")
		case <-sess.Context().Done():
			return nil
		}
	}
}

// StartTradeConsumer 阻塞直至 ctx 取消。
func StartTradeConsumer(ctx context.Context, brokers []string, groupID, topic string, h *hub.Hub, ks *kline.Store) error {
	if len(brokers) == 0 {
		return fmt.Errorf("kafka brokers empty")
	}
	if topic == "" {
		return fmt.Errorf("kafka trade topic empty")
	}
	if groupID == "" {
		return fmt.Errorf("kafka trade group_id empty")
	}

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_4_0_0
	cfg.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	g, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = g.Close() }()

	handler := &TradeHandler{Hub: h, Kline: ks}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := g.Consume(ctx, []string{topic}, handler); err != nil {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}
}
