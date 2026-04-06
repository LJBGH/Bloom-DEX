package model

// SpotMarket 现货交易对（与 spot_markets + assets 联表查询字段一致，供 sqlx/Scan 使用）。
type SpotMarket struct {
	ID           int    `db:"id"`
	Symbol       string `db:"symbol"`
	Status       string `db:"status"`
	BaseAssetID  int    `db:"base_asset_id"`
	QuoteAssetID int    `db:"quote_asset_id"`
	BaseSymbol   string `db:"base_symbol"`
	QuoteSymbol  string `db:"quote_symbol"`
	MakerFeeRate string `db:"maker_fee_rate"`
	TakerFeeRate string `db:"taker_fee_rate"`
	MinPrice     string `db:"min_price"`
	MinQuantity  string `db:"min_quantity"`
}
