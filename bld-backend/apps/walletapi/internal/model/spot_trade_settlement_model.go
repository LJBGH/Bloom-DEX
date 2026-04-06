package model

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SpotTradeSettlementModel interface {
	TryInsertTx(ctx context.Context, s sqlx.Session, tradeID uint64) (inserted bool, err error)
}

type defaultSpotTradeSettlementModel struct {
	conn sqlx.SqlConn
}

func NewSpotTradeSettlementModel(conn sqlx.SqlConn) SpotTradeSettlementModel {
	return &defaultSpotTradeSettlementModel{conn: conn}
}

func (m *defaultSpotTradeSettlementModel) TryInsertTx(ctx context.Context, s sqlx.Session, tradeID uint64) (bool, error) {
	if tradeID == 0 {
		return false, errors.New("trade_id required")
	}
	res, err := s.ExecCtx(ctx, "INSERT INTO spot_trade_settlements(trade_id) VALUES(?)", tradeID)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return false, nil
		}
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
