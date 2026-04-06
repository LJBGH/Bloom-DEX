package logic

import (
	"context"
	"errors"

	"bld-backend/apps/walletapi/internal/svc"
	"bld-backend/apps/walletapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPrivateKeyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPrivateKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPrivateKeyLogic {
	return &GetPrivateKeyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetPrivateKey 解密并返回托管私钥（生产环境应叠加鉴权、审计、二次验证）
func (l *GetPrivateKeyLogic) GetPrivateKey(in *types.GetPrivateKeyReq) (*types.GetPrivateKeyResp, error) {
	if in.UserId == 0 {
		return nil, errors.New("user_id required")
	}
	if in.NetworkId == 0 {
		return nil, errors.New("network_id required")
	}

	network, err := l.svcCtx.NetworkModel.FindByID(l.ctx, in.NetworkId)
	if err != nil {
		return nil, err
	}

	w, err := l.svcCtx.WalletModel.FindByUserNetwork(l.ctx, in.UserId, in.NetworkId)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, errors.New("wallet not found")
	}

	plain, err := l.svcCtx.MasterKey.DecryptFromBase64(w.PrivKeyEnc)
	if err != nil {
		return nil, err
	}

	return &types.GetPrivateKeyResp{
		NetworkId:  network.ID,
		NetworkSym: network.Symbol,
		CryptoType: network.CryptoType,
		Address:    w.Address,
		PrivateKey: string(plain),
	}, nil
}
