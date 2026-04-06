package model

import (
	"context"
	"database/sql"
	"errors"
	"math/big"

	"bld-backend/core/util/amount"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type WalletBalance struct {
	ID               uint64 `db:"id"`
	UserID           uint64 `db:"user_id"`
	AssetID          int    `db:"asset_id"`
	AvailableBalance string `db:"available_balance"`
	FrozenBalance    string `db:"frozen_balance"`
}

type WalletBalanceWithAsset struct {
	AssetID          int    `db:"asset_id"`
	Symbol           string `db:"symbol"`
	Decimals         int    `db:"decimals"`
	IsAggregate      int    `db:"is_aggregate"`
	AvailableBalance string `db:"available_balance"`
	FrozenBalance    string `db:"frozen_balance"`
}

type WalletBalanceModel interface {
	EnsureRow(ctx context.Context, userID uint64, assetID int) error
	GetAvailable(ctx context.Context, userID uint64, assetID int) (string, error)
	GetAvailableBySymbol(ctx context.Context, userID uint64, symbol string) (string, error)
	DebitAvailable(ctx context.Context, userID uint64, assetID int, amount string) error
	DebitAvailableBySymbol(ctx context.Context, userID uint64, symbol, amount string) error
	CreditAvailable(ctx context.Context, userID uint64, assetID int, amount string) error
	// ListByUser returns all asset balances for a user.
	ListByUser(ctx context.Context, userID uint64) ([]WalletBalance, error)
	// ListWithAssetByUser returns balances joined with asset metadata.
	ListWithAssetByUser(ctx context.Context, userID uint64) ([]WalletBalanceWithAsset, error)
	// ListWithAssetByUserAndAssetID returns a single asset balance joined with metadata.
	ListWithAssetByUserAndAssetID(ctx context.Context, userID uint64, assetID int) ([]WalletBalanceWithAsset, error)

	// RunInTransaction runs fn inside a DB transaction (same connection as balance rows).
	RunInTransaction(ctx context.Context, fn func(context.Context, sqlx.Session) error) error
	// MoveAvailableToFrozenTx moves amount from available to frozen for one balance row.
	MoveAvailableToFrozenTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, amount string) error
	// MoveFrozenToAvailableTx moves amount from frozen back to available.
	MoveFrozenToAvailableTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, amount string) error
	// SubtractFrozenTx 仅减少冻结（成交消耗），不增加可用。
	SubtractFrozenTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, amount string) error
	// AddAvailableTx 增加可用（须在事务内；无行则插入）。
	AddAvailableTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, amount string) error
	// DebitAvailableTx 减少可用（手续费等）。
	DebitAvailableTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, amount string) error
}

type defaultWalletBalanceModel struct {
	conn sqlx.SqlConn
}

func NewWalletBalanceModel(conn sqlx.SqlConn) WalletBalanceModel {
	return &defaultWalletBalanceModel{conn: conn}
}

// EnsureRow 确保钱包余额表存在对应用户和资产的行。
func (m *defaultWalletBalanceModel) EnsureRow(ctx context.Context, userID uint64, assetID int) error {
	var id uint64
	err := m.conn.QueryRowCtx(ctx, &id,
		"SELECT id FROM wallet_balances WHERE user_id=? AND asset_id=? ORDER BY id ASC LIMIT 1",
		userID, assetID,
	)
	if err == nil && id != 0 {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = m.conn.ExecCtx(ctx,
		"INSERT INTO wallet_balances(user_id,asset_id,available_balance,frozen_balance) VALUES(?,?,0,0)",
		userID, assetID,
	)
	return err
}

// GetAvailable 获取可用余额。
func (m *defaultWalletBalanceModel) GetAvailable(ctx context.Context, userID uint64, assetID int) (string, error) {
	var w WalletBalance
	err := m.conn.QueryRowCtx(ctx, &w,
		"SELECT id,user_id,asset_id,available_balance,frozen_balance FROM wallet_balances WHERE user_id=? AND asset_id=? ORDER BY id ASC LIMIT 1",
		userID, assetID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "0", nil
		}
		return "", err
	}
	return w.AvailableBalance, nil
}

// GetAvailableBySymbol 获取可用余额（按符号）。
func (m *defaultWalletBalanceModel) GetAvailableBySymbol(ctx context.Context, userID uint64, symbol string) (string, error) {
	type row struct {
		AvailableBalance string `db:"available_balance"`
		Decimals         int    `db:"decimals"`
	}
	var rows []row
	err := m.conn.QueryRowsCtx(ctx, &rows, `
SELECT wb.available_balance, a.decimals
FROM wallet_balances wb
JOIN assets a ON a.id = wb.asset_id
WHERE wb.user_id=? AND a.symbol=? AND a.is_active=1
`, userID, symbol)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "0", nil
	}
	maxDecimals := 0
	for _, r := range rows {
		if r.Decimals > maxDecimals {
			maxDecimals = r.Decimals
		}
	}
	total := big.NewInt(0)
	for _, r := range rows {
		v, err := amount.DecimalToWei(r.AvailableBalance, r.Decimals)
		if err != nil {
			return "", err
		}
		total.Add(total, scaleWei(v, r.Decimals, maxDecimals))
	}
	return amount.WeiToDecimal(total, maxDecimals), nil
}

// DebitAvailable 减少可用（手续费等）。
func (m *defaultWalletBalanceModel) DebitAvailable(ctx context.Context, userID uint64, assetID int, amount string) error {
	// 简化实现：不做链路级的余额校验（最终以 UPDATE 行是否成功为准）。
	res, err := m.conn.ExecCtx(ctx,
		"UPDATE wallet_balances SET available_balance = available_balance - ? WHERE user_id=? AND asset_id=? AND available_balance >= ? ORDER BY id ASC LIMIT 1",
		amount, userID, assetID, amount,
	)
	if err != nil {
		return err
	}
	ra, _ := res.RowsAffected()
	if ra == 0 {
		return errors.New("insufficient balance")
	}
	return nil
}

// DebitAvailableBySymbol 减少可用（按符号）。
func (m *defaultWalletBalanceModel) DebitAvailableBySymbol(ctx context.Context, userID uint64, symbol, amountStr string) error {
	type row struct {
		ID               uint64 `db:"id"`
		AvailableBalance string `db:"available_balance"`
		Decimals         int    `db:"decimals"`
	}
	var rows []row
	err := m.conn.QueryRowsCtx(ctx, &rows, `
SELECT wb.id, wb.available_balance, a.decimals
FROM wallet_balances wb
JOIN assets a ON a.id = wb.asset_id
WHERE wb.user_id=? AND a.symbol=? AND a.is_active=1
ORDER BY wb.id ASC
`, userID, symbol)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return errors.New("insufficient balance")
	}
	maxDecimals := 0
	for _, r := range rows {
		if r.Decimals > maxDecimals {
			maxDecimals = r.Decimals
		}
	}
	remaining, err := amount.DecimalToWei(amountStr, maxDecimals)
	if err != nil {
		return err
	}
	if remaining.Sign() <= 0 {
		return errors.New("invalid amount")
	}
	type debitItem struct {
		id       uint64
		decimals int
		debitWei *big.Int
		debitStr string
	}
	plan := make([]debitItem, 0, len(rows))
	for _, r := range rows {
		if remaining.Sign() <= 0 {
			break
		}
		availWei, err := amount.DecimalToWei(r.AvailableBalance, r.Decimals)
		if err != nil {
			return err
		}
		availScaled := scaleWei(availWei, r.Decimals, maxDecimals)
		if availScaled.Sign() <= 0 {
			continue
		}
		useScaled := minBig(remaining, availScaled)
		debitWei := scaleWei(useScaled, maxDecimals, r.Decimals)
		if debitWei.Sign() <= 0 {
			continue
		}
		debitStr := amount.WeiToDecimal(debitWei, r.Decimals)
		plan = append(plan, debitItem{id: r.ID, decimals: r.Decimals, debitWei: debitWei, debitStr: debitStr})
		remaining.Sub(remaining, useScaled)
	}
	if remaining.Sign() > 0 {
		return errors.New("insufficient balance")
	}
	return m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		for _, p := range plan {
			res, err := session.ExecCtx(ctx,
				"UPDATE wallet_balances SET available_balance = available_balance - ? WHERE id=? AND available_balance >= ?",
				p.debitStr, p.id, p.debitStr,
			)
			if err != nil {
				return err
			}
			ra, _ := res.RowsAffected()
			if ra == 0 {
				return errors.New("insufficient balance")
			}
		}
		return nil
	})
}

// CreditAvailable 增加可用（充值等）。
func (m *defaultWalletBalanceModel) CreditAvailable(ctx context.Context, userID uint64, assetID int, amount string) error {
	// try update first
	res, err := m.conn.ExecCtx(ctx,
		"UPDATE wallet_balances SET available_balance = available_balance + ? WHERE user_id=? AND asset_id=? ORDER BY id ASC LIMIT 1",
		amount, userID, assetID,
	)
	if err != nil {
		return err
	}
	ra, _ := res.RowsAffected()
	if ra > 0 {
		return nil
	}
	// no row -> create it then update
	if err := m.EnsureRow(ctx, userID, assetID); err != nil {
		return err
	}
	_, err = m.conn.ExecCtx(ctx,
		"UPDATE wallet_balances SET available_balance = available_balance + ? WHERE user_id=? AND asset_id=? ORDER BY id ASC LIMIT 1",
		amount, userID, assetID,
	)
	return err
}

// ListByUser 按用户列出余额。
func (m *defaultWalletBalanceModel) ListByUser(ctx context.Context, userID uint64) ([]WalletBalance, error) {
	var list []WalletBalance
	err := m.conn.QueryRowsCtx(ctx, &list,
		"SELECT id,user_id,asset_id,available_balance,frozen_balance FROM wallet_balances WHERE user_id=? ORDER BY id ASC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// ListWithAssetByUser 按用户列出余额（带资产信息）。
func (m *defaultWalletBalanceModel) ListWithAssetByUser(ctx context.Context, userID uint64) ([]WalletBalanceWithAsset, error) {
	var list []WalletBalanceWithAsset
	err := m.conn.QueryRowsCtx(ctx, &list, `
SELECT
  wb.asset_id,
  a.symbol,
  a.decimals,
  0 AS is_aggregate,
  wb.available_balance,
  wb.frozen_balance
FROM wallet_balances wb
JOIN assets a ON a.id = wb.asset_id
WHERE wb.user_id=? AND a.is_active=1 AND COALESCE(a.is_aggregate,0)=0

UNION ALL

SELECT
  MIN(wb.asset_id) AS asset_id,
  a.symbol,
  MAX(a.decimals) AS decimals,
  1 AS is_aggregate,
  SUM(wb.available_balance) AS available_balance,
  SUM(wb.frozen_balance) AS frozen_balance
FROM wallet_balances wb
JOIN assets a ON a.id = wb.asset_id
WHERE wb.user_id=? AND a.is_active=1 AND COALESCE(a.is_aggregate,0)=1
GROUP BY a.symbol
`, userID, userID)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// RunInTransaction 在事务中执行函数。
func (m *defaultWalletBalanceModel) RunInTransaction(ctx context.Context, fn func(context.Context, sqlx.Session) error) error {
	return m.conn.TransactCtx(ctx, fn)
}

// MoveAvailableToFrozenTx 将可用余额划入冻结。
func (m *defaultWalletBalanceModel) MoveAvailableToFrozenTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, amount string) error {
	res, err := s.ExecCtx(ctx,
		"UPDATE wallet_balances SET available_balance = available_balance - ?, frozen_balance = frozen_balance + ? WHERE user_id=? AND asset_id=? AND available_balance >= ? ORDER BY id ASC LIMIT 1",
		amount, amount, userID, assetID, amount,
	)
	if err != nil {
		return err
	}
	ra, _ := res.RowsAffected()
	if ra == 0 {
		return errors.New("insufficient balance")
	}
	return nil
}

// MoveFrozenToAvailableTx 将冻结余额划入可用。
func (m *defaultWalletBalanceModel) MoveFrozenToAvailableTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, amount string) error {
	res, err := s.ExecCtx(ctx,
		"UPDATE wallet_balances SET frozen_balance = frozen_balance - ?, available_balance = available_balance + ? WHERE user_id=? AND asset_id=? AND frozen_balance >= ? ORDER BY id ASC LIMIT 1",
		amount, amount, userID, assetID, amount,
	)
	if err != nil {
		return err
	}
	ra, _ := res.RowsAffected()
	if ra == 0 {
		return errors.New("insufficient frozen balance")
	}
	return nil
}

// SubtractFrozenTx 仅减少冻结（成交消耗），不增加可用。
func (m *defaultWalletBalanceModel) SubtractFrozenTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, amount string) error {
	res, err := s.ExecCtx(ctx,
		"UPDATE wallet_balances SET frozen_balance = frozen_balance - ? WHERE user_id=? AND asset_id=? AND frozen_balance >= ? ORDER BY id ASC LIMIT 1",
		amount, userID, assetID, amount,
	)
	if err != nil {
		return err
	}
	ra, _ := res.RowsAffected()
	if ra == 0 {
		return errors.New("insufficient frozen balance")
	}
	return nil
}

// AddAvailableTx 增加可用（须在事务内；无行则插入）。
func (m *defaultWalletBalanceModel) AddAvailableTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, amount string) error {
	res, err := s.ExecCtx(ctx,
		"UPDATE wallet_balances SET available_balance = available_balance + ? WHERE user_id=? AND asset_id=? ORDER BY id ASC LIMIT 1",
		amount, userID, assetID,
	)
	if err != nil {
		return err
	}
	ra, _ := res.RowsAffected()
	if ra > 0 {
		return nil
	}
	_, err = s.ExecCtx(ctx,
		"INSERT INTO wallet_balances(user_id,asset_id,available_balance,frozen_balance) VALUES(?,?,?,0)",
		userID, assetID, amount,
	)
	return err
}

// DebitAvailableTx 减少可用（手续费等）。
func (m *defaultWalletBalanceModel) DebitAvailableTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, amount string) error {
	res, err := s.ExecCtx(ctx,
		"UPDATE wallet_balances SET available_balance = available_balance - ? WHERE user_id=? AND asset_id=? AND available_balance >= ? ORDER BY id ASC LIMIT 1",
		amount, userID, assetID, amount,
	)
	if err != nil {
		return err
	}
	ra, _ := res.RowsAffected()
	if ra == 0 {
		return errors.New("insufficient available balance")
	}
	return nil
}

// ListWithAssetByUserAndAssetID 按用户和资产ID列出余额（带资产信息）。
func (m *defaultWalletBalanceModel) ListWithAssetByUserAndAssetID(ctx context.Context, userID uint64, assetID int) ([]WalletBalanceWithAsset, error) {
	var list []WalletBalanceWithAsset
	err := m.conn.QueryRowsCtx(ctx, &list, `
SELECT
  wb.asset_id,
  a.symbol,
  a.decimals,
  0 AS is_aggregate,
  wb.available_balance,
  wb.frozen_balance
FROM wallet_balances wb
JOIN assets a ON a.id = wb.asset_id
WHERE wb.user_id=? AND wb.asset_id=? AND a.is_active=1
ORDER BY wb.id ASC
`, userID, assetID)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func scaleWei(v *big.Int, fromDecimals, toDecimals int) *big.Int {
	out := new(big.Int).Set(v)
	if fromDecimals == toDecimals {
		return out
	}
	diff := fromDecimals - toDecimals
	if diff > 0 {
		div := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(diff)), nil)
		return out.Div(out, div)
	}
	mul := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-diff)), nil)
	return out.Mul(out, mul)
}

func minBig(a, b *big.Int) *big.Int {
	if a.Cmp(b) <= 0 {
		return new(big.Int).Set(a)
	}
	return new(big.Int).Set(b)
}
