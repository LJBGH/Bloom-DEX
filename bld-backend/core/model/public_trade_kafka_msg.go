package model

// PublicTradeKafkaMsg 单笔公开成交（taker 方向），供 market-ws 推送给前端。
type PublicTradeKafkaMsg struct {
	MarketID int    `json:"market_id"`
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
	// Side 为吃单方方向：BUY 表示主动买成交，SELL 表示主动卖成交。
	Side string `json:"side"`
	TsMs int64  `json:"ts_ms"`
}
