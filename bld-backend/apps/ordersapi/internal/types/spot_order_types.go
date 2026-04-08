package types

// CreateSpotOrderReq is the request body for POST /v1/spot/orders.
// 注意：go-zero httpx.Parse 对指针字段默认要求 JSON 里“有该键”；可选字段须加 `optional`。
// 冻结金额由 wallet 服务 asset_freezes / wallet_balances 维护，订单表不存冻结字段。
type CreateSpotOrderReq struct {
	UserId    uint64 `json:"user_id"`
	MarketId  int    `json:"market_id"`
	Side      string `json:"side"`       // BUY/SELL
	OrderType string `json:"order_type"` // LIMIT/MARKET
	// AmountInputMode 市价单必填：QUANTITY=按数量 TURNOVER=按成交额；限价单可省略，服务端固定为 QUANTITY
	AmountInputMode *string `json:"amount_input_mode,optional"`
	// Price 仅限价单必填；市价单请勿传（传了也会被忽略，库中 price 为 NULL）
	Price    *string `json:"price,optional"`
	Quantity string  `json:"quantity"` // LIMIT: 必填 base；市价卖: 必填；市价买: 可省略或 0（与成交额二选一）
	// MaxQuoteAmount 市价买「成交额」模式：报价币预算；与 quantity+reference_price 二选一（可并存时优先本字段）
	MaxQuoteAmount *string `json:"max_quote_amount,optional"`
	// ReferencePrice 市价买「数量」模式可选：用于在缺少 max_quote_amount 时估算报价冻结（如卖一价），与 quantity 配合
	ReferencePrice *string `json:"reference_price,optional"`
	ClientOrderId  *string `json:"client_order_id,optional"`
}

type CreateSpotOrderResp struct {
	OrderId string `json:"order_id"`
	Status  string `json:"status"`
}

// CancelSpotOrderReq POST /v1/spot/orders/cancel
type CancelSpotOrderReq struct {
	UserId  uint64 `json:"user_id"`
	OrderId string `json:"order_id"`
}

type CancelSpotOrderResp struct {
	OrderId string `json:"order_id"`
	Status  string `json:"status"`
}

// ListSpotOrdersReq GET /v1/spot/orders
type ListSpotOrdersReq struct {
	UserId   uint64 `form:"user_id"`
	MarketId int    `form:"market_id,optional"`
	Scope    string `form:"scope"` // open | history
	Limit    int    `form:"limit,optional"`
}

type SpotOrderListItem struct {
	OrderId           string  `json:"order_id"`
	MarketId          int     `json:"market_id"`
	Symbol            string  `json:"symbol"`
	BaseSymbol        string  `json:"base_symbol"`
	QuoteSymbol       string  `json:"quote_symbol"`
	Side              string  `json:"side"`
	OrderType         string  `json:"order_type"`
	AmountInputMode   string  `json:"amount_input_mode"`
	TradeInputMode    string  `json:"trade_input_mode"`
	Price             *string `json:"price,omitempty"`
	Quantity          string  `json:"quantity"`
	MaxQuoteAmount    *string `json:"max_quote_amount,omitempty"`
	FilledQuoteAmount string  `json:"filled_quote_amount"`
	MaxTurnover       *string `json:"max_turnover,omitempty"`
	FilledTurnover    string  `json:"filled_turnover"`
	FilledQuantity    string  `json:"filled_quantity"`
	RemainingQuantity string  `json:"remaining_quantity"`
	AvgFillPrice      *string `json:"avg_fill_price,omitempty"`
	Status            string  `json:"status"`
	ClientOrderId     *string `json:"client_order_id,omitempty"`
	CreatedAt         int64   `json:"created_at_ms"`
	UpdatedAt         int64   `json:"updated_at_ms"`
}

type ListSpotOrdersResp struct {
	Items []SpotOrderListItem `json:"items"`
}

// ListSpotTradesReq GET /v1/spot/trades
type ListSpotTradesReq struct {
	UserId   uint64 `form:"user_id"`
	MarketId int    `form:"market_id,optional"`
	Limit    int    `form:"limit,optional"`
}

type SpotTradeListItem struct {
	TradeId    string `json:"trade_id"`
	MarketId   int    `json:"market_id"`
	Symbol     string `json:"symbol"`
	Side       string `json:"side"`
	Role       string `json:"role"`
	Price      string `json:"price"`
	Quantity   string `json:"quantity"`
	FeeAmount  string `json:"fee_amount"`
	CreatedAt  int64  `json:"created_at_ms"`
}

type ListSpotTradesResp struct {
	Items []SpotTradeListItem `json:"items"`
}
