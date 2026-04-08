package kline

import (
	"context"
	"database/sql"
	"math/big"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Store persists pre-aggregated klines to MySQL.
type Store struct {
	conn sqlx.SqlConn
}

func NewStore(conn sqlx.SqlConn) *Store {
	return &Store{conn: conn}
}

type Row struct {
	MarketID    int    `json:"market_id"`
	Interval    string `json:"interval"`
	OpenTimeMs  int64  `json:"open_time_ms"`
	Open        string `json:"open"`
	High        string `json:"high"`
	Low         string `json:"low"`
	Close       string `json:"close"`
	Volume      string `json:"volume"`
	Turnover    string `json:"turnover"`
	TradesCount int    `json:"trades_count"`
}

// 获取K线
func (s *Store) Get(ctx context.Context, marketID int, interval string, openTimeMs int64) (*Row, error) {
	var r Row
	err := s.conn.QueryRowCtx(
		ctx,
		&r,
		"SELECT market_id, `interval`, open_time_ms, `open`, `high`, `low`, `close`, volume, turnover, trades_count "+
			"FROM spot_klines "+
			"WHERE market_id = ? AND `interval` = ? AND open_time_ms = ? "+
			"ORDER BY updated_at DESC, id DESC "+
			"LIMIT 1",
		marketID,
		interval,
		openTimeMs,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

// 将一笔交易应用到 (market_id, open_time_ms) 的 1m K线。
func (s *Store) Upsert(ctx context.Context, marketID int, interval string, openTimeMs int64, price, qty string) error {
	interval = strings.TrimSpace(interval)
	p := strings.TrimSpace(price)
	q := strings.TrimSpace(qty)
	if interval == "" || p == "" || q == "" {
		return nil
	}
	// turnover = price * qty
	pr := new(big.Rat)
	qr := new(big.Rat)
	if _, ok := pr.SetString(p); !ok {
		return nil
	}
	if _, ok := qr.SetString(q); !ok {
		return nil
	}
	turn := new(big.Rat).Mul(pr, qr)
	turnStr := ratToDecimal18(turn)

	// NOTE: 现在只有单主键 id，没有联合唯一键可用于 ON DUPLICATE KEY。
	// 这里用“先 UPDATE，若未更新则 INSERT”的方式保证在单消费者场景下正确聚合。
	return s.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		res, err := session.ExecCtx(
			ctx,
			"UPDATE spot_klines "+
				"SET `high` = GREATEST(`high`, ?), "+
				"`low` = LEAST(`low`, ?), "+
				"`close` = ?, "+
				"volume = volume + ?, "+
				"turnover = turnover + ?, "+
				"trades_count = trades_count + 1, "+
				"updated_at = NOW() "+
				"WHERE id = ( "+
				"  SELECT id FROM ( "+
				"    SELECT id "+
				"    FROM spot_klines "+
				"    WHERE market_id = ? AND `interval` = ? AND open_time_ms = ? "+
				"    ORDER BY updated_at DESC, id DESC "+
				"    LIMIT 1 "+
				"  ) t "+
				")",
			p, p, p, q, turnStr,
			marketID, interval, openTimeMs,
		)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n > 0 {
			return nil
		}

		_, err = session.ExecCtx(
			ctx,
			"INSERT INTO spot_klines (market_id, `interval`, open_time_ms, `open`, `high`, `low`, `close`, volume, turnover, trades_count) "+
				"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)",
			marketID, interval, openTimeMs,
			p, p, p, p,
			q, turnStr,
		)
		return err
	})
}

// 获取K线列表
func (s *Store) List(ctx context.Context, marketID int, interval string, fromMs, toMs int64, limit int) ([]Row, error) {
	if limit <= 0 {
		limit = 500
	}
	if limit > 2000 {
		limit = 2000
	}
	var rows []Row
	err := s.conn.QueryRowsCtx(
		ctx,
		&rows,
		"SELECT market_id, `interval`, open_time_ms, `open`, `high`, `low`, `close`, volume, turnover, trades_count "+
			"FROM spot_klines "+
			"WHERE market_id = ? "+
			"  AND `interval` = ? "+
			"  AND open_time_ms >= ? "+
			"  AND (? = 0 OR open_time_ms <= ?) "+
			"ORDER BY open_time_ms ASC "+
			"LIMIT ?",
		marketID, interval, fromMs, toMs, toMs, limit,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return []Row{}, nil
		}
		return nil, err
	}
	return rows, nil
}

// 获取K线总数
func (s *Store) Count(ctx context.Context) (int64, error) {
	var n int64
	if err := s.conn.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM spot_klines"); err != nil {
		return 0, err
	}
	return n, nil
}

// 从 spot_trades 中聚合最新的交易到 1m K线。
func (s *Store) BootstrapFromSpotTrades(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = 5000
	}
	type tradeRow struct {
		MarketID  int
		Price     string
		Quantity  string
		CreatedAt time.Time
	}
	var trades []tradeRow
	if err := s.conn.QueryRowsCtx(ctx, &trades,
		"SELECT market_id, price, quantity, created_at FROM spot_trades ORDER BY id DESC LIMIT ?",
		limit,
	); err != nil {
		return err
	}
	// Process oldest -> newest so open is consistent.
	for i := len(trades) - 1; i >= 0; i-- {
		tr := trades[i]
		openTimeMs := (tr.CreatedAt.UnixMilli() / 60000) * 60000
		if err := s.Upsert(ctx, tr.MarketID, "1m", openTimeMs, tr.Price, tr.Quantity); err != nil {
			return err
		}
	}
	return nil
}

// 将 big.Rat 转换为字符串，保留 18 位小数。
func ratToDecimal18(r *big.Rat) string {
	if r == nil {
		return "0"
	}
	s := r.FloatString(18)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}
