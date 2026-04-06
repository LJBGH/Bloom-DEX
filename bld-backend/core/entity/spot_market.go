package entity

import "time"

// SpotMarket 对应表 spot_markets（纯表字段；联表查询可用 core/model.SpotMarket）。
type SpotMarket struct {
	ID           int       `db:"id"`
	BaseAssetID  int       `db:"base_asset_id"`
	QuoteAssetID int       `db:"quote_asset_id"`
	Symbol       string    `db:"symbol"`
	Status       string    `db:"status"`
	MakerFeeRate string    `db:"maker_fee_rate"`
	TakerFeeRate string    `db:"taker_fee_rate"`
	MinPrice     string    `db:"min_price"`
	MinQuantity  string    `db:"min_quantity"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}
