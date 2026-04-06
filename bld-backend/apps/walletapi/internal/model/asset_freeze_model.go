package model

import (
	"context"
	"database/sql"
	"errors"
	"math/big"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type AssetFreeze struct {
	ID           uint64 `db:"id"`
	UserID       uint64 `db:"user_id"`
	AssetID      int    `db:"asset_id"`
	OrderID      uint64 `db:"order_id"`
	TradingType  string `db:"trading_type"`
	FrozenAmount string `db:"frozen_amount"`
	IsFrozen     int    `db:"is_frozen"`
}

type AssetFreezeModel interface {
	HasActiveFreezeTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, orderID uint64, tradingType string) (bool, error)
	InsertActiveTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, orderID uint64, tradingType, amount string) (uint64, error)
	FindActiveTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, orderID uint64, tradingType string) (*AssetFreeze, error)
	MarkUnfrozenTx(ctx context.Context, s sqlx.Session, id uint64) error
	// ReduceFrozenForActiveOrderTx 减少单笔订单活跃冻结量（部分成交）；与 wallet_balances 冻结扣减一致使用。
	ReduceFrozenForActiveOrderTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, orderID uint64, tradingType, reduceAmount string) error
}

type defaultAssetFreezeModel struct {
	conn sqlx.SqlConn
}

func NewAssetFreezeModel(conn sqlx.SqlConn) AssetFreezeModel {
	return &defaultAssetFreezeModel{conn: conn}
}

func (m *defaultAssetFreezeModel) HasActiveFreezeTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, orderID uint64, tradingType string) (bool, error) {
	var n int64
	err := s.QueryRowCtx(ctx, &n,
		"SELECT COUNT(*) FROM asset_freezes WHERE user_id=? AND asset_id=? AND order_id=? AND trading_type=? AND is_frozen=1",
		userID, assetID, orderID, tradingType,
	)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (m *defaultAssetFreezeModel) InsertActiveTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, orderID uint64, tradingType, amount string) (uint64, error) {
	res, err := s.ExecCtx(ctx,
		"INSERT INTO asset_freezes(user_id,asset_id,order_id,trading_type,frozen_amount,is_frozen) VALUES(?,?,?,?,?,1)",
		userID, assetID, orderID, tradingType, amount,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (m *defaultAssetFreezeModel) FindActiveTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, orderID uint64, tradingType string) (*AssetFreeze, error) {
	var row AssetFreeze
	err := s.QueryRowCtx(ctx, &row,
		"SELECT id,user_id,asset_id,order_id,trading_type,frozen_amount,is_frozen FROM asset_freezes WHERE user_id=? AND asset_id=? AND order_id=? AND trading_type=? AND is_frozen=1 ORDER BY id ASC LIMIT 1",
		userID, assetID, orderID, tradingType,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (m *defaultAssetFreezeModel) MarkUnfrozenTx(ctx context.Context, s sqlx.Session, id uint64) error {
	_, err := s.ExecCtx(ctx, "UPDATE asset_freezes SET is_frozen=0 WHERE id=? AND is_frozen=1", id)
	return err
}

func ratToDecimal18Freeze(r *big.Rat) string {
	f := new(big.Float).SetPrec(512).SetRat(r)
	return trimTrailingZerosFreeze(f.Text('f', 18))
}

func trimTrailingZerosFreeze(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return s[:i]
		}
		if s[i] != '0' {
			return s[:i+1]
		}
	}
	return s
}

func (m *defaultAssetFreezeModel) ReduceFrozenForActiveOrderTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, orderID uint64, tradingType, reduceAmount string) error {
	// 仅扫描用到的列；勿用完整 AssetFreeze，否则列数与字段数不一致会报 not matching destination to scan。
	var row struct {
		ID           uint64 `db:"id"`
		FrozenAmount string `db:"frozen_amount"`
	}
	err := s.QueryRowCtx(ctx, &row,
		"SELECT id,frozen_amount FROM asset_freezes WHERE user_id=? AND asset_id=? AND order_id=? AND trading_type=? AND is_frozen=1 ORDER BY id ASC LIMIT 1 FOR UPDATE",
		userID, assetID, orderID, tradingType,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("no active freeze row")
		}
		return err
	}
	cur := new(big.Rat)
	red := new(big.Rat)
	if _, ok := cur.SetString(row.FrozenAmount); !ok {
		return errors.New("invalid frozen_amount")
	}
	if _, ok := red.SetString(reduceAmount); !ok {
		return errors.New("invalid reduce amount")
	}
	if red.Sign() <= 0 {
		return errors.New("reduce amount must be > 0")
	}
	if cur.Cmp(red) < 0 {
		return errors.New("insufficient frozen on asset_freezes")
	}
	rem := new(big.Rat).Sub(cur, red)
	if rem.Sign() == 0 {
		_, err = s.ExecCtx(ctx, "UPDATE asset_freezes SET frozen_amount='0', is_frozen=0 WHERE id=?", row.ID)
		return err
	}
	_, err = s.ExecCtx(ctx, "UPDATE asset_freezes SET frozen_amount=? WHERE id=?", ratToDecimal18Freeze(rem), row.ID)
	return err
}
