package logic

import (
	"context"
	"fmt"

	"bld-backend/apps/ordersapi/internal/svc"
	"bld-backend/apps/ordersapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSpotTradesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSpotTradesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSpotTradesLogic {
	return &ListSpotTradesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListSpotTradesLogic) List(req *types.ListSpotTradesReq) (*types.ListSpotTradesResp, error) {
	if req.UserId == 0 {
		return nil, fmt.Errorf("user_id required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	marketID := req.MarketId
	if marketID < 0 {
		marketID = 0
	}

	rows, err := l.svcCtx.SpotTradeModel.ListForUser(l.ctx, req.UserId, marketID, limit)
	if err != nil {
		return nil, err
	}
	items := make([]types.SpotTradeListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, types.SpotTradeListItem{
			TradeId:   r.ID,
			MarketId:  r.MarketID,
			Symbol:    r.Symbol,
			Side:      r.Side,
			Role:      r.Role,
			Price:     r.Price,
			Quantity:  r.Quantity,
			FeeAmount: r.FeeAmount,
			CreatedAt: r.CreatedAt.UnixMilli(),
		})
	}
	return &types.ListSpotTradesResp{Items: items}, nil
}
