package entity

import (
	"database/sql"
	"time"
)

// WithdrawOrder 对应表 withdraw_orders。
type WithdrawOrder struct {
	ID          uint64         `db:"id"`
	UserID      uint64         `db:"user_id"`
	AssetID     int            `db:"asset_id"`
	DestAddress string         `db:"dest_address"`
	Amount      string         `db:"amount"`
	Status      string         `db:"status"`
	TxHash      sql.NullString `db:"tx_hash"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}
