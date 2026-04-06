// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"bld-backend/apps/ordersapi/internal/svc"
	"bld-backend/apps/ordersapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TradeapiLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTradeapiLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TradeapiLogic {
	return &TradeapiLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TradeapiLogic) Tradeapi() (resp *types.HealthzResp, err error) {
	// todo: add your logic here and delete this line

	return &types.HealthzResp{Status: "ok"}, nil
}
