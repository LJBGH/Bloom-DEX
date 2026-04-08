package entity

import (
	"database/sql"
	"time"
)

// SpotOrder 对应表 spot_orders。
type SpotOrder struct {
	ID                uint64         `db:"id"`
	UserID            uint64         `db:"user_id"`
	MarketID          int            `db:"market_id"`
	Side              string         `db:"side"`
	OrderType         string         `db:"order_type"`
	AmountInputMode   string         `db:"amount_input_mode"`
	Price             sql.NullString `db:"price"`
	Quantity          string         `db:"quantity"`
	MaxQuoteAmount    sql.NullString `db:"max_quote_amount"`
	FilledQuoteAmount string         `db:"filled_quote_amount"`
	FilledQuantity    string         `db:"filled_quantity"`
	RemainingQuantity string         `db:"remaining_quantity"`
	AvgFillPrice      sql.NullString `db:"avg_fill_price"`
	Status            string         `db:"status"`
	ClientOrderID     sql.NullString `db:"client_order_id"`
	CreatedAt         time.Time      `db:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at"`
	CancelledAt       sql.NullTime   `db:"cancelled_at"`
}
