package model

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type Asset struct {
	ID              int            `db:"id"`
	Symbol          string         `db:"symbol"`
	Decimals        int            `db:"decimals"`
	IsAggregate     int            `db:"is_aggregate"`
	NetworkID       int            `db:"network_id"`
	ContractAddress sql.NullString `db:"contract_address"`
}

// 资产模型
type AssetModel interface {
	FindBySymbol(ctx context.Context, symbol string) (*Asset, error)
	FindBySymbolNetwork(ctx context.Context, symbol string, networkId int) (*Asset, error)
	FindByID(ctx context.Context, id int) (*Asset, error)
	ListActiveByNetwork(ctx context.Context, networkId int) ([]Asset, error)
	ShouldAggregateSymbol(ctx context.Context, symbol string) (bool, error)
}

// 资产模型实现
type defaultAssetModel struct {
	conn sqlx.SqlConn
}

// 创建资产模型
func NewAssetModel(conn sqlx.SqlConn) AssetModel {
	return &defaultAssetModel{conn: conn}
}

// 根据符号查找资产
func (m *defaultAssetModel) FindBySymbol(ctx context.Context, symbol string) (*Asset, error) {
	var a Asset
	err := m.conn.QueryRowCtx(ctx, &a,
		"SELECT id,symbol,decimals,COALESCE(is_aggregate,0) AS is_aggregate,network_id,contract_address FROM assets WHERE symbol=? AND is_active=1 ORDER BY id ASC LIMIT 1",
		symbol,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func (m *defaultAssetModel) FindBySymbolNetwork(ctx context.Context, symbol string, networkId int) (*Asset, error) {
	var a Asset
	err := m.conn.QueryRowCtx(ctx, &a,
		"SELECT id,symbol,decimals,COALESCE(is_aggregate,0) AS is_aggregate,network_id,contract_address FROM assets WHERE symbol=? AND network_id=? AND is_active=1 ORDER BY id ASC LIMIT 1",
		symbol, networkId,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// 根据ID查找资产
func (m *defaultAssetModel) FindByID(ctx context.Context, id int) (*Asset, error) {
	var a Asset
	err := m.conn.QueryRowCtx(ctx, &a,
		"SELECT id,symbol,decimals,COALESCE(is_aggregate,0) AS is_aggregate,network_id,contract_address FROM assets WHERE id=? AND is_active=1 ORDER BY id ASC LIMIT 1",
		id,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func (m *defaultAssetModel) ListActiveByNetwork(ctx context.Context, networkId int) ([]Asset, error) {
	db, err := m.conn.RawDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, "SELECT id,symbol,decimals,COALESCE(is_aggregate,0) AS is_aggregate,network_id,contract_address FROM assets WHERE network_id=? AND is_active=1 ORDER BY id ASC", networkId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Asset
	for rows.Next() {
		var a Asset
		if err := rows.Scan(&a.ID, &a.Symbol, &a.Decimals, &a.IsAggregate, &a.NetworkID, &a.ContractAddress); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func (m *defaultAssetModel) ShouldAggregateSymbol(ctx context.Context, symbol string) (bool, error) {
	var v int
	err := m.conn.QueryRowCtx(ctx, &v,
		"SELECT 1 FROM assets WHERE symbol=? AND is_active=1 AND COALESCE(is_aggregate,0)=1 ORDER BY id ASC LIMIT 1",
		symbol,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return v == 1, nil
}
