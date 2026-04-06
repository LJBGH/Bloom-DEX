package entity

import (
	"database/sql"
	"time"
)

// DepositEvent 对应表 deposit_events。
type DepositEvent struct {
	ID          uint64         `db:"id"`
	TxHash      string         `db:"tx_hash"`
	LogIndex    int            `db:"log_index"`
	BlockNumber int64          `db:"block_number"`
	Chain       string         `db:"chain"`
	AssetID     int            `db:"asset_id"`
	UserID      uint64         `db:"user_id"`
	Amount      string         `db:"amount"`
	FromAddress sql.NullString `db:"from_address"`
	ToAddress   sql.NullString `db:"to_address"`
	CreatedAt   time.Time      `db:"created_at"`
}
