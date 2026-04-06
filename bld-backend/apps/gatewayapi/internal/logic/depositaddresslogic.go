package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"bld-backend/apps/gatewayapi/internal/svc"
	"bld-backend/apps/gatewayapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DepositAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDepositAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DepositAddressLogic {
	return &DepositAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DepositAddressLogic) GetDepositAddress(req *types.GetDepositAddressReq) (*types.GetDepositAddressResp, error) {
	if req.UserId == 0 {
		return nil, errors.New("user_id required")
	}
	symbol := strings.TrimSpace(req.Symbol)
	if symbol == "" {
		return nil, errors.New("symbol required")
	}

	base := strings.TrimRight(l.svcCtx.Config.WalletCoreRestUrl, "/")
	url := fmt.Sprintf("%s/v1/deposit/address", base)

	payload := struct {
		UserId uint64 `json:"user_id"`
		Symbol string `json:"symbol"`
	}{
		UserId: req.UserId,
		Symbol: symbol,
	}

	var out types.GetDepositAddressResp
	err := postJSON(l.ctx, url, &payload, &out)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

