package logic

import (
	"context"
	"errors"
	"strings"

	"bld-backend/apps/walletapi/internal/model"
	"bld-backend/apps/walletapi/internal/svc"
	"bld-backend/apps/walletapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetDepositAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// 获取充值地址逻辑
func NewGetDepositAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDepositAddressLogic {
	return &GetDepositAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取充值地址逻辑
func (l *GetDepositAddressLogic) GetDepositAddress(in *types.GetDepositAddressReq) (*types.GetDepositAddressResp, error) {
	userID := in.UserId
	if userID == 0 {
		return nil, errors.New("user_id required")
	}

	symbol := strings.TrimSpace(in.Symbol)
	if symbol == "" {
		return nil, errors.New("symbol required")
	}

	var network *model.Network
	var err error
	if in.NetworkId == 0 {
		network, err = l.svcCtx.NetworkModel.FindBySymbol(l.ctx, "LOCALHOST")
	} else {
		network, err = l.svcCtx.NetworkModel.FindByID(l.ctx, in.NetworkId)
	}
	if err != nil {
		return nil, err
	}
	networkId := network.ID

	asset, err := l.svcCtx.AssetModel.FindBySymbolNetwork(l.ctx, symbol, networkId)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, errors.New("asset not found")
	}

	w, err := l.svcCtx.WalletModel.FindByUserNetwork(l.ctx, userID, networkId)
	if err != nil {
		return nil, err
	}
	if w == nil {
		_, _, err = insertNewCustodyWallet(l.ctx, l.svcCtx, userID, network)
		if err != nil {
			return nil, err
		}
		w, err = l.svcCtx.WalletModel.FindByUserNetwork(l.ctx, userID, networkId)
		if err != nil {
			return nil, err
		}
		if w == nil {
			return nil, errors.New("create wallet failed")
		}
	}

	contract := ""
	if asset.ContractAddress.Valid {
		contract = asset.ContractAddress.String
	}

	return &types.GetDepositAddressResp{
		WalletId:        w.ID,
		Chain:           network.CryptoType,
		Address:         w.Address,
		Decimals:        uint32(asset.Decimals),
		ContractAddress: contract,
	}, nil
}
