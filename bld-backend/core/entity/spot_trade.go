package entity

import (
	"database/sql"
	"time"
)

// SpotTrade 对应表 spot_trades。
type SpotTrade struct {
	ID           uint64         `db:"id"`
	MarketID     int            `db:"market_id"`
	MakerOrderID uint64         `db:"maker_order_id"`
	TakerOrderID uint64         `db:"taker_order_id"`
	Price        string         `db:"price"`
	Quantity     string         `db:"quantity"`
	FeeAssetID   sql.NullInt64  `db:"fee_asset_id"`
	FeeAmount    string         `db:"fee_amount"`
	TxHash       sql.NullString `db:"tx_hash"`
	CreatedAt    time.Time      `db:"created_at"`
}
