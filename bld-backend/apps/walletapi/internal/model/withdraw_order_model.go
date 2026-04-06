package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type WithdrawOrder struct {
	ID          uint64 `db:"id"`
	UserID      uint64 `db:"user_id"`
	AssetID     int    `db:"asset_id"`
	DestAddress string `db:"dest_address"`
	Amount      string `db:"amount"`
	Status      string `db:"status"`
	TxHash      *string `db:"tx_hash"`
}

type WithdrawOrderModel interface {
	Create(ctx context.Context, userID uint64, assetID int, destAddress, amount, status string) (uint64, error)
	SetTxHash(ctx context.Context, id uint64, txHash string) error
}

type defaultWithdrawOrderModel struct {
	conn sqlx.SqlConn
}

func NewWithdrawOrderModel(conn sqlx.SqlConn) WithdrawOrderModel {
	return &defaultWithdrawOrderModel{conn: conn}
}

func (m *defaultWithdrawOrderModel) Create(ctx context.Context, userID uint64, assetID int, destAddress, amount, status string) (uint64, error) {
	res, err := m.conn.ExecCtx(ctx,
		"INSERT INTO withdraw_orders(user_id,asset_id,dest_address,amount,status) VALUES(?,?,?,?,?)",
		userID, assetID, destAddress, amount, status,
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

func (m *defaultWithdrawOrderModel) SetTxHash(ctx context.Context, id uint64, txHash string) error {
	_, err := m.conn.ExecCtx(ctx,
		"UPDATE withdraw_orders SET tx_hash=? WHERE id=?",
		txHash, id,
	)
	return err
}

