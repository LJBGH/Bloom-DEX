package model

// SpotOrderKafkaMsg 现货订单 Kafka 消息体（生产者与消费者共用契约）。
type SpotOrderKafkaMsg struct {
	OrderID         uint64  `json:"order_id"`                  // 订单 ID
	UserID          uint64  `json:"user_id"`                   // 用户 ID
	MarketID        int     `json:"market_id"`                 // 市场 ID
	CreatedAtMs     int64   `json:"created_at_ms"`             // 创建时间
	Side            string  `json:"side"`                      // 买卖方向
	OrderType       string  `json:"order_type"`                // 订单类型
	AmountInputMode string  `json:"amount_input_mode"`         // 市价单数量维度
	Price           *string `json:"price,omitempty"`           // 价格
	Quantity        string  `json:"quantity"`                  // 数量
	MaxQuoteAmount  *string `json:"max_quote_amount,omitempty"`// 市价买按成交额模式预算
	FilledQuantity  string  `json:"filled_quantity"`           // 已成交数量
	RemainingQty    string  `json:"remaining_quantity"`        // 剩余数量
	AvgFillPrice    *string `json:"avg_fill_price,omitempty"`  // 平均成交价格
	BaseAssetID     int     `json:"base_asset_id"`             // 基础币种 ID
	QuoteAssetID    int     `json:"quote_asset_id"`            // 报价币种 ID
	MakerFeeRate    string  `json:"maker_fee_rate"`            //  maker 手续费率
	TakerFeeRate    string  `json:"taker_fee_rate"`            //  taker 手续费率
	ClientOrderID   *string `json:"client_order_id,omitempty"` // 客户端订单 ID
	Status          string  `json:"status"`                    // 订单状态
}
