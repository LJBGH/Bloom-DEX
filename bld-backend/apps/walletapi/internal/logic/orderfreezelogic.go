package logic

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"bld-backend/apps/walletapi/internal/svc"
	"bld-backend/core/enum"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type OrderFreezeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOrderFreezeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrderFreezeLogic {
	return &OrderFreezeLogic{ctx: ctx, svcCtx: svcCtx}
}

func normalizeTradingType(s string) (string, error) {
	t := strings.ToUpper(strings.TrimSpace(s))
	if t != enum.Spot.String() && t != enum.Contract.String() {
		return "", errors.New("trading_type must be SPOT or CONTRACT")
	}
	return t, nil
}

func isPositiveDecimalString(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	r := new(big.Rat)
	if _, ok := r.SetString(s); !ok {
		return false
	}
	return r.Sign() > 0
}

// FreezeForOrder 可用 -> 冻结，并写 asset_freezes。
func (l *OrderFreezeLogic) FreezeForOrder(userID uint64, assetID int, orderID uint64, amount, tradingType string) (uint64, error) {
	if userID == 0 {
		return 0, errors.New("user_id required")
	}
	if assetID <= 0 {
		return 0, errors.New("asset_id required")
	}
	if orderID == 0 {
		return 0, errors.New("order_id required")
	}
	tt, err := normalizeTradingType(tradingType)
	if err != nil {
		return 0, err
	}
	amt := strings.TrimSpace(amount)
	if !isPositiveDecimalString(amt) {
		return 0, errors.New("amount must be > 0")
	}

	if err := l.svcCtx.WalletBalanceModel.EnsureRow(l.ctx, userID, assetID); err != nil {
		return 0, err
	}

	var freezeID uint64
	err = l.svcCtx.WalletBalanceModel.RunInTransaction(l.ctx, func(ctx context.Context, s sqlx.Session) error {
		dup, err := l.svcCtx.AssetFreezeModel.HasActiveFreezeTx(ctx, s, userID, assetID, orderID, tt)
		if err != nil {
			return err
		}
		if dup {
			return errors.New("active freeze already exists for this order and asset")
		}
		if err := l.svcCtx.WalletBalanceModel.MoveAvailableToFrozenTx(ctx, s, userID, assetID, amt); err != nil {
			return err
		}
		id, err := l.svcCtx.AssetFreezeModel.InsertActiveTx(ctx, s, userID, assetID, orderID, tt, amt)
		if err != nil {
			return err
		}
		freezeID = id
		return nil
	})
	if err != nil {
		return 0, err
	}
	return freezeID, nil
}

// UnfreezeForOrder 解冻并退回可用；若无活跃冻结则视为成功（幂等）。
func (l *OrderFreezeLogic) UnfreezeForOrder(userID uint64, assetID int, orderID uint64, tradingType string) error {
	if userID == 0 {
		return errors.New("user_id required")
	}
	if assetID <= 0 {
		return errors.New("asset_id required")
	}
	if orderID == 0 {
		return errors.New("order_id required")
	}
	tt, err := normalizeTradingType(tradingType)
	if err != nil {
		return err
	}

	return l.svcCtx.WalletBalanceModel.RunInTransaction(l.ctx, func(ctx context.Context, s sqlx.Session) error {
		row, err := l.svcCtx.AssetFreezeModel.FindActiveTx(ctx, s, userID, assetID, orderID, tt)
		if err != nil {
			return err
		}
		if row == nil {
			return nil
		}
		amt := strings.TrimSpace(row.FrozenAmount)
		if err := l.svcCtx.WalletBalanceModel.MoveFrozenToAvailableTx(ctx, s, userID, assetID, amt); err != nil {
			return err
		}
		return l.svcCtx.AssetFreezeModel.MarkUnfrozenTx(ctx, s, row.ID)
	})
}
