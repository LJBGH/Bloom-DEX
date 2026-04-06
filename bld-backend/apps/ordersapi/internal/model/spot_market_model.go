package model

import (
	"context"
	"database/sql"

	coremodel "bld-backend/core/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SpotMarketModel interface {
	List(ctx context.Context, status string) ([]coremodel.SpotMarket, error)
	// GetByID 返回交易对（不限制 status）；不存在时 err 为 sql.ErrNoRows。
	GetByID(ctx context.Context, marketID int) (*coremodel.SpotMarket, error)
}

type defaultSpotMarketModel struct {
	conn sqlx.SqlConn
}

func NewSpotMarketModel(conn sqlx.SqlConn) SpotMarketModel {
	return &defaultSpotMarketModel{conn: conn}
}

func (m *defaultSpotMarketModel) List(ctx context.Context, status string) ([]coremodel.SpotMarket, error) {
	db, err := m.conn.RawDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(
		ctx,
		`SELECT
			sm.id,
			sm.symbol,
			sm.status,
			sm.base_asset_id,
			sm.quote_asset_id,
			ab.symbol AS base_symbol,
			aq.symbol AS quote_symbol,
			sm.maker_fee_rate,
			sm.taker_fee_rate,
			sm.min_price,
			sm.min_quantity
		FROM spot_markets sm
		JOIN assets ab ON ab.id = sm.base_asset_id
		JOIN assets aq ON aq.id = sm.quote_asset_id
		WHERE sm.status = ?
		ORDER BY sm.id ASC`,
		status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]coremodel.SpotMarket, 0)
	for rows.Next() {
		var mkt coremodel.SpotMarket
		if err := rows.Scan(
			&mkt.ID,
			&mkt.Symbol,
			&mkt.Status,
			&mkt.BaseAssetID,
			&mkt.QuoteAssetID,
			&mkt.BaseSymbol,
			&mkt.QuoteSymbol,
			&mkt.MakerFeeRate,
			&mkt.TakerFeeRate,
			&mkt.MinPrice,
			&mkt.MinQuantity,
		); err != nil {
			return nil, err
		}
		out = append(out, mkt)
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return out, nil
}

func (m *defaultSpotMarketModel) GetByID(ctx context.Context, marketID int) (*coremodel.SpotMarket, error) {
	var mkt coremodel.SpotMarket
	err := m.conn.QueryRowCtx(
		ctx,
		&mkt,
		`SELECT
			sm.id,
			sm.symbol,
			sm.status,
			sm.base_asset_id,
			sm.quote_asset_id,
			ab.symbol AS base_symbol,
			aq.symbol AS quote_symbol,
			sm.maker_fee_rate,
			sm.taker_fee_rate,
			sm.min_price,
			sm.min_quantity
		FROM spot_markets sm
		JOIN assets ab ON ab.id = sm.base_asset_id
		JOIN assets aq ON aq.id = sm.quote_asset_id
		WHERE sm.id = ?
		LIMIT 1`,
		marketID,
	)
	if err != nil {
		return nil, err
	}
	return &mkt, nil
}
