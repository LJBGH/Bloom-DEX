package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type Network struct {
	ID           int    `db:"id"`
	Symbol       string `db:"symbol"`
	Name         string `db:"name"`
	RpcURL       *string `db:"rpc_url"`
	ChainID      *int64  `db:"chain_id"`
	CryptoType string `db:"crypto_type"`
}

type NetworkModel interface {
	List(ctx context.Context) ([]Network, error)
	FindByID(ctx context.Context, id int) (*Network, error)
	FindBySymbol(ctx context.Context, symbol string) (*Network, error)
}

type defaultNetworkModel struct {
	conn sqlx.SqlConn
}

func NewNetworkModel(conn sqlx.SqlConn) NetworkModel {
	return &defaultNetworkModel{conn: conn}
}

func (m *defaultNetworkModel) List(ctx context.Context) ([]Network, error) {
	var out []Network
	err := m.conn.QueryRowsCtx(ctx, &out, "SELECT id,symbol,name,rpc_url,chain_id,crypto_type FROM networks ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (m *defaultNetworkModel) FindByID(ctx context.Context, id int) (*Network, error) {
	var n Network
	err := m.conn.QueryRowCtx(ctx, &n, "SELECT id,symbol,name,rpc_url,chain_id,crypto_type FROM networks WHERE id=? LIMIT 1", id)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (m *defaultNetworkModel) FindBySymbol(ctx context.Context, symbol string) (*Network, error) {
	var n Network
	err := m.conn.QueryRowCtx(ctx, &n, "SELECT id,symbol,name,rpc_url,chain_id,crypto_type FROM networks WHERE symbol=? ORDER BY id ASC LIMIT 1", symbol)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

