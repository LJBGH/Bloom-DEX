package model

// KlineWsMsg is the websocket payload for channel="kline".
// open_time_ms is the candle open time aligned by interval.
type KlineWsMsg struct {
	MarketID    int    `json:"market_id"`
	Interval    string `json:"interval"`       // e.g. "1m"
	OpenTimeMs  int64  `json:"open_time_ms"`   // candle open time
	Open        string `json:"open"`
	High        string `json:"high"`
	Low         string `json:"low"`
	Close       string `json:"close"`
	Volume      string `json:"volume"`         // base volume
	Turnover    string `json:"turnover"`       // quote turnover
	TradesCount int    `json:"trades_count"`
	IsFinal     bool   `json:"is_final"`       // true when candle is closed (optional for UI)
	TsMs        int64  `json:"ts_ms"`          // server send time
}

