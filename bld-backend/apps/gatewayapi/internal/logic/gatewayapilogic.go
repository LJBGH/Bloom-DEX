// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"bld-backend/apps/gatewayapi/internal/svc"
	"bld-backend/apps/gatewayapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GatewayapiLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 网关API逻辑
func NewGatewayapiLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GatewayapiLogic {
	return &GatewayapiLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 网关API逻辑
func (l *GatewayapiLogic) Gatewayapi() (resp *types.HealthzResp, err error) {
	// todo: add your logic here and delete this line

	return &types.HealthzResp{Status: "ok"}, nil
}
