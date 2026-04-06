package logic

import (
	"context"

	"bld-backend/apps/walletapi/internal/svc"
	"bld-backend/apps/walletapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListNetworksLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListNetworksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListNetworksLogic {
	return &ListNetworksLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListNetworksLogic) ListNetworks() (*types.NetworkListResp, error) {
	rows, err := l.svcCtx.NetworkModel.List(l.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]types.NetworkItem, 0, len(rows))
	for _, r := range rows {
		var rpc string
		if r.RpcURL != nil {
			rpc = *r.RpcURL
		}
		var chainID int64
		if r.ChainID != nil {
			chainID = *r.ChainID
		}
		out = append(out, types.NetworkItem{
			Id:           r.ID,
			Symbol:       r.Symbol,
			Name:         r.Name,
			RpcUrl:       rpc,
			ChainId:      chainID,
			CryptoType: r.CryptoType,
		})
	}
	return &types.NetworkListResp{Items: out}, nil
}

