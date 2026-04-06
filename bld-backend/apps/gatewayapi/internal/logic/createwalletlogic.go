// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

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

type CreateWalletLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateWalletLogic {
	return &CreateWalletLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateWalletLogic) CreateWallet(req *types.CreateWalletReq) (resp *types.CreateWalletResp, err error) {
	if req.UserId == 0 {
		return nil, errors.New("user_id required")
	}

	base := strings.TrimRight(l.svcCtx.Config.WalletCoreRestUrl, "/")
	url := fmt.Sprintf("%s/v1/wallet/create", base)

	payload := struct {
		UserId uint64 `json:"user_id"`
		Chain  string `json:"chain"`
	}{
		UserId: req.UserId,
		Chain:  "EVM",
	}

	var out types.CreateWalletResp
	err = postJSON(l.ctx, url, &payload, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
