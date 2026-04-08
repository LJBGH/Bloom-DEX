package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	"bld-backend/core/enum"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// SpotOrderRow spot_orders 行（撮合用字段）。
type SpotOrderRow struct {
	ID                uint64         `db:"id"`
	UserID            uint64         `db:"user_id"`
	MarketID          int            `db:"market_id"`
	CreatedAt         time.Time      `db:"created_at"`
	Side              string         `db:"side"`
	OrderType         string         `db:"order_type"`
	Price             sql.NullString `db:"price"`
	Quantity          string         `db:"quantity"`
	FilledQuoteAmount string         `db:"filled_quote_amount"`
	FilledQuantity    string         `db:"filled_quantity"`
	RemainingQuantity string         `db:"remaining_quantity"`
	AvgFillPrice      sql.NullString `db:"avg_fill_price"`
	Status            string         `db:"status"`
}

// MarketFees 交易对手续费率（字符串 DECIMAL）。
type MarketFees struct {
	ID           int    `db:"id"`
	BaseAssetID  int    `db:"base_asset_id"`
	QuoteAssetID int    `db:"quote_asset_id"`
	MakerFeeRate string `db:"maker_fee_rate"`
	TakerFeeRate string `db:"taker_fee_rate"`
}

type marketIDRow struct {
	ID int `db:"id"`
}

// TradeRow 待写入 spot_trades。
type TradeRow struct {
	MarketID     int
	MakerOrderID uint64
	TakerOrderID uint64
	Price        string
	Quantity     string
	FeeAssetID   sql.NullInt64
	FeeAmount    string
	MakerFee     string
	TakerFee     string
}

// OrderFillUpdate 成交后对单笔订单的累计更新（引擎在内存中算好最终列值）。
type OrderFillUpdate struct {
	OrderID           uint64
	FilledQuoteDelta  string
	FilledQuantity    string
	RemainingQuantity string
	Status            string
	AvgFillPrice      sql.NullString
}

type SpotStore struct {
	conn sqlx.SqlConn
}

func NewSpotStore(conn sqlx.SqlConn) *SpotStore {
	return &SpotStore{conn: conn}
}

// GetOrder 获取订单。
func (s *SpotStore) GetOrder(ctx context.Context, orderID uint64) (*SpotOrderRow, error) {
	var r SpotOrderRow
	err := s.conn.QueryRowCtx(ctx, &r, `
SELECT id, user_id, market_id, created_at, side, order_type, price, quantity, filled_quote_amount, filled_quantity, remaining_quantity, avg_fill_price, status
FROM spot_orders WHERE id = ?`, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

// GetMarket 获取市场手续费。
func (s *SpotStore) GetMarket(ctx context.Context, marketID int) (*MarketFees, error) {
	var m MarketFees
	err := s.conn.QueryRowCtx(ctx, &m, `
SELECT id, base_asset_id, quote_asset_id, maker_fee_rate, taker_fee_rate FROM spot_markets WHERE id = ? LIMIT 1`, marketID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// CancelLimitOrder 将未完结限价单置为已撤销并清零剩余量；仅当仍为 PENDING/PARTIALLY_FILLED 时返回 true。
func (s *SpotStore) CancelLimitOrder(ctx context.Context, orderID uint64) (bool, error) {
	res, err := s.conn.ExecCtx(ctx, `
UPDATE spot_orders
SET status = ?, remaining_quantity = 0, cancelled_at = NOW(), updated_at = NOW()
WHERE id = ?
  AND order_type = ?
  AND status IN (?, ?)`,
		enum.SOS_Canceled.String(),
		orderID,
		enum.Limit.String(),
		enum.SOS_Pending.String(),
		enum.SOS_PartiallyFilled.String(),
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ListActiveMarketIDs 用于恢复订单簿。
func (s *SpotStore) ListActiveMarketIDs(ctx context.Context) ([]int, error) {
	var rows []marketIDRow
	err := s.conn.QueryRowsCtx(ctx, &rows, `SELECT id FROM spot_markets WHERE status = ? ORDER BY id ASC`, enum.SMS_Active.String())
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out, nil
}

// ListActiveMarkets 返回活跃市场及手续费信息（用于恢复阶段批量加载）。
func (s *SpotStore) ListActiveMarkets(ctx context.Context) ([]MarketFees, error) {
	var rows []MarketFees
	err := s.conn.QueryRowsCtx(ctx, &rows, `
SELECT id, base_asset_id, quote_asset_id, maker_fee_rate, taker_fee_rate
FROM spot_markets
WHERE status = ?
ORDER BY id ASC`, enum.SMS_Active.String())
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListOpenLimitOrders 某市场未完结限价单，按 id 时间序。
func (s *SpotStore) ListOpenLimitOrders(ctx context.Context, marketID int) ([]SpotOrderRow, error) {
	var rows []SpotOrderRow
	err := s.conn.QueryRowsCtx(ctx, &rows, `
SELECT id, user_id, market_id, created_at, side, order_type, price, quantity, filled_quote_amount, filled_quantity, remaining_quantity, avg_fill_price, status
FROM spot_orders
WHERE market_id = ?
  AND order_type = ?
  AND status IN (?, ?)
  AND remaining_quantity > 0
ORDER BY id ASC`, marketID, enum.Limit.String(), enum.SOS_Pending.String(), enum.SOS_PartiallyFilled.String())
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// 返回所有活跃市场的未完结限价单，按 market_id、id 升序。
func (s *SpotStore) ListOpenLimitOrdersForActiveMarkets(ctx context.Context) ([]SpotOrderRow, error) {
	var rows []SpotOrderRow
	err := s.conn.QueryRowsCtx(ctx, &rows, `
SELECT o.id, o.user_id, o.market_id, o.created_at, o.side, o.order_type, o.price, o.quantity, o.filled_quote_amount, o.filled_quantity, o.remaining_quantity, o.avg_fill_price, o.status
FROM spot_orders o
INNER JOIN spot_markets m ON m.id = o.market_id
WHERE m.status = ?
  AND o.order_type = ?
  AND o.status IN (?, ?)
  AND o.remaining_quantity > 0
ORDER BY o.market_id ASC, o.id ASC`, enum.SMS_Active.String(), enum.Limit.String(), enum.SOS_Pending.String(), enum.SOS_PartiallyFilled.String())
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// RunMatchTx 在同一事务中写入成交并更新订单；返回与 trades 顺序一致的 spot_trades.id。
func (s *SpotStore) RunMatchTx(ctx context.Context, trades []TradeRow, orders []OrderFillUpdate) ([]uint64, error) {
	var ids []uint64
	// 在事务中执行函数。
	err := s.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		// 初始化成交 ID 切片。
		ids = make([]uint64, 0, len(trades))
		// 遍历成交。
		for _, t := range trades {
			// 插入成交。
			q := `INSERT INTO spot_trades (market_id, maker_order_id, taker_order_id, price, quantity, fee_asset_id, fee_amount) VALUES (?,?,?,?,?,?,?)`
			res, err := session.ExecCtx(ctx, q, t.MarketID, t.MakerOrderID, t.TakerOrderID, t.Price, t.Quantity, t.FeeAssetID, t.FeeAmount)
			if err != nil {
				return fmt.Errorf("insert spot_trades: %w", err)
			}
			lid, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("spot_trades last id: %w", err)
			}
			ids = append(ids, uint64(lid))
		}
		for _, o := range orders {
			var avg any
			if o.AvgFillPrice.Valid {
				avg = o.AvgFillPrice.String
			} else {
				avg = nil
			}
			_, err := session.ExecCtx(ctx, `
UPDATE spot_orders SET filled_quote_amount = filled_quote_amount + ?, filled_quantity = ?, remaining_quantity = ?, status = ?, avg_fill_price = ?, updated_at = NOW()
WHERE id = ?`, o.FilledQuoteDelta, o.FilledQuantity, o.RemainingQuantity, o.Status, avg, o.OrderID)
			if err != nil {
				return fmt.Errorf("update spot_orders %d: %w", o.OrderID, err)
			}
		}
		return nil
	})
	return ids, err
}

// AvgFill 计算新的加权平均成交价（旧成交量、旧均价、本笔增量）。
func AvgFill(oldFilled, oldAvg *big.Rat, tradeQty, tradePrice *big.Rat) *big.Rat {
	if oldFilled.Sign() == 0 {
		return new(big.Rat).Set(tradePrice)
	}
	num := new(big.Rat).Mul(oldAvg, oldFilled)
	num.Add(num, new(big.Rat).Mul(tradePrice, tradeQty))
	den := new(big.Rat).Add(oldFilled, tradeQty)
	if den.Sign() == 0 {
		return new(big.Rat).Set(tradePrice)
	}
	return new(big.Rat).Quo(num, den)
}
