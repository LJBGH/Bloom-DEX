package entity

import "time"

// WalletBalance 对应表 wallet_balances。
type WalletBalance struct {
	ID               uint64    `db:"id"`
	UserID           uint64    `db:"user_id"`
	AssetID          int       `db:"asset_id"`
	AvailableBalance string    `db:"available_balance"`
	FrozenBalance    string    `db:"frozen_balance"`
	UpdatedAt        time.Time `db:"updated_at"`
}
