package logic

import (
	"context"
	"fmt"
	"strings"

	"bld-backend/apps/ordersapi/internal/model"
	"bld-backend/apps/ordersapi/internal/svc"
	"bld-backend/apps/ordersapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSpotOrdersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSpotOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSpotOrdersLogic {
	return &ListSpotOrdersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListSpotOrdersLogic) List(req *types.ListSpotOrdersReq) (*types.ListSpotOrdersResp, error) {
	if req.UserId == 0 {
		return nil, fmt.Errorf("user_id required")
	}
	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	if scope != "open" && scope != "history" {
		return nil, fmt.Errorf("scope must be open or history")
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

	var rows []model.UserSpotOrderRow
	var err error
	if scope == "open" {
		rows, err = l.svcCtx.SpotOrderModel.ListOpenByUser(l.ctx, req.UserId, marketID, limit)
	} else {
		rows, err = l.svcCtx.SpotOrderModel.ListHistoryByUser(l.ctx, req.UserId, marketID, limit)
	}
	if err != nil {
		return nil, err
	}

	items := make([]types.SpotOrderListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, orderRowToItem(r))
	}
	return &types.ListSpotOrdersResp{Items: items}, nil
}

func orderRowToItem(r model.UserSpotOrderRow) types.SpotOrderListItem {
	it := types.SpotOrderListItem{
		OrderId:           r.ID,
		MarketId:          r.MarketID,
		Symbol:            r.Symbol,
		BaseSymbol:        r.BaseSymbol,
		QuoteSymbol:       r.QuoteSymbol,
		Side:              r.Side,
		OrderType:         r.OrderType,
		AmountInputMode:   r.AmountInputMode,
		Quantity:          r.Quantity,
		FilledQuantity:    r.FilledQuantity,
		RemainingQuantity: r.RemainingQuantity,
		Status:            r.Status,
		CreatedAt:         r.CreatedAt.UnixMilli(),
		UpdatedAt:         r.UpdatedAt.UnixMilli(),
	}
	if r.Price.Valid {
		s := r.Price.String
		it.Price = &s
	}
	if r.AvgFillPrice.Valid {
		s := r.AvgFillPrice.String
		it.AvgFillPrice = &s
	}
	if r.ClientOrderID.Valid {
		s := r.ClientOrderID.String
		it.ClientOrderId = &s
	}
	return it
}
