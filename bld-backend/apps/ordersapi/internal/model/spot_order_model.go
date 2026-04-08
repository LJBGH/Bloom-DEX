package model

import (
	"context"
	"database/sql"
	"time"

	"bld-backend/core/enum"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// UserSpotOrderRow 用户订单列表行（含交易对展示字段）。
type UserSpotOrderRow struct {
	ID                uint64
	MarketID          int
	Symbol            string
	BaseSymbol        string
	QuoteSymbol       string
	Side              string
	OrderType         string
	AmountInputMode   string
	Price             sql.NullString
	Quantity          string
	MaxQuoteAmount    sql.NullString
	FilledQuoteAmount string
	FilledQuantity    string
	RemainingQuantity string
	AvgFillPrice      sql.NullString
	Status            string
	ClientOrderID     sql.NullString
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type SpotOrderModel interface {
	MarketExists(ctx context.Context, marketID int) (bool, error)
	Create(ctx context.Context, orderID uint64, userID uint64, marketID int, side, orderType, amountInputMode string, price *string, quantity string, maxQuoteAmount *string, clientOrderID *string) (uint64, error)
	// GetByIDAndUser 按订单号与用户加载一行；不存在时 err 为 sql.ErrNoRows。
	GetByIDAndUser(ctx context.Context, orderID uint64, userID uint64) (*UserSpotOrderRow, error)
	// ListOpenByUser 当前委托：PENDING / PARTIALLY_FILLED。marketID=0 表示全部交易对。
	ListOpenByUser(ctx context.Context, userID uint64, marketID int, limit int) ([]UserSpotOrderRow, error)
	// ListHistoryByUser 历史委托：FILLED / CANCELED / REJECTED。
	ListHistoryByUser(ctx context.Context, userID uint64, marketID int, limit int) ([]UserSpotOrderRow, error)
}

type defaultSpotOrderModel struct {
	conn sqlx.SqlConn
}

func NewSpotOrderModel(conn sqlx.SqlConn) SpotOrderModel {
	return &defaultSpotOrderModel{conn: conn}
}

// MarketExists 检查交易对是否存在
func (m *defaultSpotOrderModel) MarketExists(ctx context.Context, marketID int) (bool, error) {
	var id int
	err := m.conn.QueryRowCtx(ctx, &id, "SELECT id FROM spot_markets WHERE id=? LIMIT 1", marketID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Create 创建现货订单
func (m *defaultSpotOrderModel) Create(
	ctx context.Context,
	orderID uint64,
	userID uint64,
	marketID int,
	side, orderType, amountInputMode string,
	price *string,
	quantity string,
	maxQuoteAmount *string,
	clientOrderID *string,
) (uint64, error) {
	var priceAny any = nil
	if price != nil {
		priceAny = *price
	}

	var clientOrderAny any = nil
	if clientOrderID != nil {
		clientOrderAny = *clientOrderID
	}
	var maxQuoteAny any = nil
	if maxQuoteAmount != nil {
		maxQuoteAny = *maxQuoteAmount
	}

	res, err := m.conn.ExecCtx(
		ctx,
		"INSERT INTO spot_orders(id,user_id,market_id,side,order_type,amount_input_mode,price,quantity,max_quote_amount,filled_quote_amount,filled_quantity,remaining_quantity,avg_fill_price,status,client_order_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		orderID,
		userID,
		marketID,
		side,
		orderType,
		amountInputMode,
		priceAny,
		quantity,
		maxQuoteAny,
		"0",
		"0",
		quantity,
		nil, // avg_fill_price
		enum.SOS_Pending.String(),
		clientOrderAny,
	)
	if err != nil {
		return 0, err
	}

	_ = res
	return orderID, nil
}

func (m *defaultSpotOrderModel) GetByIDAndUser(ctx context.Context, orderID uint64, userID uint64) (*UserSpotOrderRow, error) {
	db, err := m.conn.RawDB()
	if err != nil {
		return nil, err
	}
	q := `SELECT o.id, o.market_id, sm.symbol, ab.symbol, aq.symbol,
		o.side, o.order_type, o.amount_input_mode, o.price, o.quantity, o.max_quote_amount,
		o.filled_quote_amount, o.filled_quantity, o.remaining_quantity, o.avg_fill_price, o.status,
		o.client_order_id, o.created_at, o.updated_at
	FROM spot_orders o
	JOIN spot_markets sm ON sm.id = o.market_id
	JOIN assets ab ON ab.id = sm.base_asset_id
	JOIN assets aq ON aq.id = sm.quote_asset_id
	WHERE o.id = ? AND o.user_id = ?
	LIMIT 1`
	rows, err := db.QueryContext(ctx, q, orderID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanUserSpotOrderRows(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, sql.ErrNoRows
	}
	return &list[0], nil
}

func (m *defaultSpotOrderModel) listByUser(
	ctx context.Context,
	userID uint64,
	marketID int,
	limit int,
	statusIn string,
) ([]UserSpotOrderRow, error) {
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
	q := `SELECT o.id, o.market_id, sm.symbol, ab.symbol, aq.symbol,
		o.side, o.order_type, o.amount_input_mode, o.price, o.quantity, o.max_quote_amount,
		o.filled_quote_amount, o.filled_quantity, o.remaining_quantity, o.avg_fill_price, o.status,
		o.client_order_id, o.created_at, o.updated_at
	FROM spot_orders o
	JOIN spot_markets sm ON sm.id = o.market_id
	JOIN assets ab ON ab.id = sm.base_asset_id
	JOIN assets aq ON aq.id = sm.quote_asset_id
	WHERE o.user_id = ?
	AND o.status IN (` + statusIn + `)
	AND (? = 0 OR o.market_id = ?)
	ORDER BY o.created_at DESC
	LIMIT ?`
	rows, err := db.QueryContext(ctx, q, userID, marketID, marketID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUserSpotOrderRows(rows)
}

func scanUserSpotOrderRows(rows *sql.Rows) ([]UserSpotOrderRow, error) {
	out := make([]UserSpotOrderRow, 0)
	for rows.Next() {
		var r UserSpotOrderRow
		if err := rows.Scan(
			&r.ID,
			&r.MarketID,
			&r.Symbol,
			&r.BaseSymbol,
			&r.QuoteSymbol,
			&r.Side,
			&r.OrderType,
			&r.AmountInputMode,
			&r.Price,
			&r.Quantity,
			&r.MaxQuoteAmount,
			&r.FilledQuoteAmount,
			&r.FilledQuantity,
			&r.RemainingQuantity,
			&r.AvgFillPrice,
			&r.Status,
			&r.ClientOrderID,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (m *defaultSpotOrderModel) ListOpenByUser(ctx context.Context, userID uint64, marketID int, limit int) ([]UserSpotOrderRow, error) {
	return m.listByUser(ctx, userID, marketID, limit,
		"'"+enum.SOS_Pending.String()+"','"+enum.SOS_PartiallyFilled.String()+"'")
}

func (m *defaultSpotOrderModel) ListHistoryByUser(ctx context.Context, userID uint64, marketID int, limit int) ([]UserSpotOrderRow, error) {
	return m.listByUser(ctx, userID, marketID, limit,
		"'"+enum.SOS_Filled.String()+"','"+enum.SOS_Canceled.String()+"','"+enum.SOS_Rejected.String()+"'")
}
