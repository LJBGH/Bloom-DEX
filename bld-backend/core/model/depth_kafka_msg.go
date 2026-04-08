package model

// DepthPriceLevel 盘口一档（价 + 该价总数量，base 单位）。
type DepthPriceLevel struct {
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
}

// MarketDepthKafkaMsg 订单簿快照（供 market-ws 推送给前端）；按 market_id 分区。
type MarketDepthKafkaMsg struct {
	MarketID int               `json:"market_id"` //市场ID
	Seq      int64             `json:"seq"`       //序列号
	Bids     []DepthPriceLevel `json:"bids"`      //买盘
	Asks     []DepthPriceLevel `json:"asks"`      //卖盘
	TsMs     int64             `json:"ts_ms"`     //时间戳
}
