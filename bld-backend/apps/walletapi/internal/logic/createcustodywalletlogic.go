package logic

import (
	"context"
	"errors"

	"bld-backend/apps/walletapi/internal/model"
	"bld-backend/apps/walletapi/internal/svc"
	"bld-backend/apps/walletapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateCustodyWalletLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCustodyWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCustodyWalletLogic {
	return &CreateCustodyWalletLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// walletResp 返回钱包信息
func (l *CreateCustodyWalletLogic) walletResp(w *model.CustodyWallet, n *model.Network) *types.CreateCustodyWalletResp {
	return &types.CreateCustodyWalletResp{
		WalletId:   w.ID,
		UserId:     w.UserID,
		NetworkId:  n.ID,
		NetworkSym: n.Symbol,
		Chain:      n.CryptoType,
		Address:    w.Address,
	}
}

// CreateCustodyWallet 创建托管钱包：network_id 须来自 GET /v1/networks；同 crypto_type 已存在其他链钱包时复用密文与地址。
func (l *CreateCustodyWalletLogic) CreateCustodyWallet(in *types.CreateCustodyWalletReq) (*types.CreateCustodyWalletResp, error) {
	userId := in.UserId
	if userId == 0 {
		return nil, errors.New("user_id required")
	}
	if in.NetworkId == 0 {
		return nil, errors.New("network_id required")
	}

	network, err := l.svcCtx.NetworkModel.FindByID(l.ctx, in.NetworkId)
	if err != nil {
		return nil, err
	}
	networkId := network.ID

	existing, err := l.svcCtx.WalletModel.FindByUserNetwork(l.ctx, userId, networkId)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return l.walletResp(existing, network), nil
	}

	sameType, err := l.svcCtx.WalletModel.FindByUserAndCryptoType(l.ctx, userId, network.CryptoType)
	if err != nil {
		return nil, err
	}
	if sameType != nil && sameType.NetworkID != networkId {
		wid, err := l.svcCtx.WalletModel.InsertWithNetwork(l.ctx, userId, networkId, sameType.Address, sameType.PrivKeyEnc)
		if err != nil {
			if errors.Is(err, model.ErrDuplicateWallet) {
				return l.afterDuplicateInsert(userId, networkId, network)
			}
			return nil, err
		}
		return &types.CreateCustodyWalletResp{
			WalletId:   wid,
			UserId:     userId,
			NetworkId:  networkId,
			NetworkSym: network.Symbol,
			Chain:      network.CryptoType,
			Address:    sameType.Address,
		}, nil
	}

	wid, addr, err := insertNewCustodyWallet(l.ctx, l.svcCtx, userId, network)
	if err != nil {
		if errors.Is(err, model.ErrDuplicateWallet) {
			return l.afterDuplicateInsert(userId, networkId, network)
		}
		return nil, err
	}

	return &types.CreateCustodyWalletResp{
		WalletId:   wid,
		UserId:     userId,
		NetworkId:  networkId,
		NetworkSym: network.Symbol,
		Chain:      network.CryptoType,
		Address:    addr,
	}, nil
}

// afterDuplicateInsert 在插入重复钱包时，返回已有的钱包信息
func (l *CreateCustodyWalletLogic) afterDuplicateInsert(userId uint64, networkId int, network *model.Network) (*types.CreateCustodyWalletResp, error) {
	existing, err := l.svcCtx.WalletModel.FindByUserNetwork(l.ctx, userId, networkId)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return l.walletResp(existing, network), nil
	}
	return nil, errors.New("duplicate wallet")
}
