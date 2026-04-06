package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// UserSpotTradeRow 用户成交明细（用户在 maker 或 taker 一侧）。
type UserSpotTradeRow struct {
	ID         uint64
	MarketID   int
	Symbol     string
	Side       string
	Role       string // MAKER | TAKER
	Price      string
	Quantity   string
	FeeAmount  string
	CreatedAt  time.Time
}

type SpotTradeModel interface {
	ListForUser(ctx context.Context, userID uint64, marketID int, limit int) ([]UserSpotTradeRow, error)
}

type defaultSpotTradeModel struct {
	conn sqlx.SqlConn
}

func NewSpotTradeModel(conn sqlx.SqlConn) SpotTradeModel {
	return &defaultSpotTradeModel{conn: conn}
}

func (m *defaultSpotTradeModel) ListForUser(ctx context.Context, userID uint64, marketID int, limit int) ([]UserSpotTradeRow, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	db, err := m.conn.RawDB()
	if err != nil {
		return nil, err
	}
	q := `SELECT
		t.id,
		t.market_id,
		sm.symbol,
		CASE WHEN ot.user_id = ? THEN ot.side ELSE om.side END,
		CASE WHEN ot.user_id = ? THEN 'TAKER' ELSE 'MAKER' END,
		t.price,
		t.quantity,
		t.fee_amount,
		t.created_at
	FROM spot_trades t
	JOIN spot_markets sm ON sm.id = t.market_id
	JOIN spot_orders om ON om.id = t.maker_order_id
	JOIN spot_orders ot ON ot.id = t.taker_order_id
	WHERE (om.user_id = ? OR ot.user_id = ?)
	AND (? = 0 OR t.market_id = ?)
	ORDER BY t.created_at DESC
	LIMIT ?`
	rows, err := db.QueryContext(ctx, q,
		userID, userID, userID, userID, marketID, marketID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]UserSpotTradeRow, 0)
	for rows.Next() {
		var r UserSpotTradeRow
		if err := rows.Scan(
			&r.ID,
			&r.MarketID,
			&r.Symbol,
			&r.Side,
			&r.Role,
			&r.Price,
			&r.Quantity,
			&r.FeeAmount,
			&r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
