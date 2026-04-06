package model

import (
	"context"
	"database/sql"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SpotFundFlowModel interface {
	InsertTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, marketID int32, orderID sql.NullInt64, tradeID uint64, flowType, reason, availDelta, frozenDelta string) error
}

type defaultSpotFundFlowModel struct {
	conn sqlx.SqlConn
}

func NewSpotFundFlowModel(conn sqlx.SqlConn) SpotFundFlowModel {
	return &defaultSpotFundFlowModel{conn: conn}
}

func (m *defaultSpotFundFlowModel) InsertTx(ctx context.Context, s sqlx.Session, userID uint64, assetID int, marketID int32, orderID sql.NullInt64, tradeID uint64, flowType, reason, availDelta, frozenDelta string) error {
	var mid any
	if marketID > 0 {
		mid = marketID
	}
	var oid any
	if orderID.Valid {
		oid = orderID.Int64
	}
	var tid any
	if tradeID > 0 {
		tid = tradeID
	}
	var rsn any = reason
	if reason == "" {
		rsn = nil
	}
	_, err := s.ExecCtx(ctx, `
INSERT INTO spot_fund_flows (user_id, asset_id, market_id, order_id, trade_id, flow_type, reason, available_delta, frozen_delta)
VALUES (?,?,?,?,?,?,?,?,?)`,
		userID, assetID, mid, oid, tid, flowType, rsn, availDelta, frozenDelta,
	)
	return err
}
