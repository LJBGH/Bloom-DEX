package model

// DepthPriceLevel 盘口一档（价 + 该价总数量，base 单位）。
type DepthPriceLevel struct {
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
}

// MarketDepthKafkaMsg 订单簿快照（供 market-ws 推送给前端）；按 market_id 分区。
type MarketDepthKafkaMsg struct {
	MarketID int               `json:"market_id"`
	Seq      int64             `json:"seq"`
	Bids     []DepthPriceLevel `json:"bids"`
	Asks     []DepthPriceLevel `json:"asks"`
	TsMs     int64             `json:"ts_ms"`
}
