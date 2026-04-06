package entity

import (
	"database/sql"
	"time"
)

// SpotFundFlow 对应表 spot_fund_flows。
type SpotFundFlow struct {
	ID             uint64         `db:"id"`
	UserID         uint64         `db:"user_id"`
	AssetID        int            `db:"asset_id"`
	MarketID       sql.NullInt64  `db:"market_id"`
	OrderID        sql.NullInt64  `db:"order_id"`
	TradeID        sql.NullInt64  `db:"trade_id"`
	FlowType       string         `db:"flow_type"`
	Reason         sql.NullString `db:"reason"`
	AvailableDelta string         `db:"available_delta"`
	FrozenDelta    string         `db:"frozen_delta"`
	TxHash         sql.NullString `db:"tx_hash"`
	CreatedAt      time.Time      `db:"created_at"`
}
