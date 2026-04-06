package logic

import (
	"context"
	"strings"

	"bld-backend/apps/ordersapi/internal/svc"
	"bld-backend/apps/ordersapi/internal/types"
	"bld-backend/core/enum"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSpotMarketsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSpotMarketsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSpotMarketsLogic {
	return &ListSpotMarketsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListSpotMarkets 列出现货交易对
func (l *ListSpotMarketsLogic) List(status string) (*types.SpotMarketsResp, error) {
	s := strings.ToUpper(strings.TrimSpace(status))
	if s == "" {
		s = enum.SMS_Active.String()
	}

	items, err := l.svcCtx.SpotMarketModel.List(l.ctx, s)
	if err != nil {
		return nil, err
	}

	respItems := make([]types.SpotMarketItem, 0, len(items))
	for _, it := range items {
		respItems = append(respItems, types.SpotMarketItem{
			MarketID:     it.ID,
			Symbol:       it.Symbol,
			Status:       it.Status,
			BaseAssetID:  it.BaseAssetID,
			QuoteAssetID: it.QuoteAssetID,
			BaseSymbol:   it.BaseSymbol,
			QuoteSymbol:  it.QuoteSymbol,
			MakerFeeRate: it.MakerFeeRate,
			TakerFeeRate: it.TakerFeeRate,
			MinPrice:     it.MinPrice,
			MinQuantity:  it.MinQuantity,
		})
	}

	return &types.SpotMarketsResp{Items: respItems}, nil
}
