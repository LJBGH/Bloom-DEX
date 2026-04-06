package model

// SpotOrderKafkaMsg 现货订单 Kafka 消息体（生产者与消费者共用契约）。
type SpotOrderKafkaMsg struct {
	OrderID         uint64  `json:"order_id"`
	UserID          uint64  `json:"user_id"`
	MarketID        int     `json:"market_id"`
	Side            string  `json:"side"`
	OrderType       string  `json:"order_type"`
	AmountInputMode string  `json:"amount_input_mode"`
	Price           *string `json:"price,omitempty"`
	Quantity        string  `json:"quantity"`
	ClientOrderID   *string `json:"client_order_id,omitempty"`
	Status          string  `json:"status"`
}
