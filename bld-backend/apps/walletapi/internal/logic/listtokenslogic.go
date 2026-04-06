package logic

import (
	"context"

	"bld-backend/apps/walletapi/internal/svc"
	"bld-backend/apps/walletapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTokensLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTokensLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTokensLogic {
	return &ListTokensLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListTokensLogic) ListTokens(networkID int) (*types.TokenListResp, error) {
	rows, err := l.svcCtx.AssetModel.ListActiveByNetwork(l.ctx, networkID)
	if err != nil {
		return nil, err
	}
	out := make([]types.TokenItem, 0, len(rows))
	for _, a := range rows {
		contract := ""
		if a.ContractAddress.Valid {
			contract = a.ContractAddress.String
		}
		out = append(out, types.TokenItem{
			AssetId:          a.ID,
			Symbol:           a.Symbol,
			Decimals:         uint32(a.Decimals),
			ContractAddress:  contract,
			NetworkId:        a.NetworkID,
		})
	}
	return &types.TokenListResp{NetworkId: networkID, Items: out}, nil
}

