package types

type SpotMarketItem struct {
	MarketID   int     `json:"market_id"`
	Symbol     string  `json:"symbol"`
	Status     string  `json:"status"`
	BaseAssetID  int    `json:"base_asset_id"`
	QuoteAssetID int    `json:"quote_asset_id"`
	BaseSymbol string  `json:"base_symbol"`
	QuoteSymbol string `json:"quote_symbol"`

	MakerFeeRate string `json:"maker_fee_rate"`
	TakerFeeRate string `json:"taker_fee_rate"`

	MinPrice     string `json:"min_price"`
	MinQuantity  string `json:"min_quantity"`
}

type SpotMarketsResp struct {
	Items []SpotMarketItem `json:"items"`
}

