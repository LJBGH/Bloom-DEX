package entity

import "time"

// AssetFreeze 对应表 asset_freezes。
type AssetFreeze struct {
	ID           uint64    `db:"id"`
	UserID       uint64    `db:"user_id"`
	AssetID      int       `db:"asset_id"`
	OrderID      uint64    `db:"order_id"`
	TradingType  string    `db:"trading_type"`
	FrozenAmount string    `db:"frozen_amount"`
	IsFrozen     int       `db:"is_frozen"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}
