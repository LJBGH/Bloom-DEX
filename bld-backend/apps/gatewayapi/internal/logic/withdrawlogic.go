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

type WithdrawLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewWithdrawLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WithdrawLogic {
	return &WithdrawLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *WithdrawLogic) Withdraw(req *types.WithdrawReq) (*types.WithdrawResp, error) {
	if req.UserId == 0 {
		return nil, errors.New("user_id required")
	}
	symbol := strings.TrimSpace(req.Symbol)
	if symbol == "" {
		return nil, errors.New("symbol required")
	}
	dest := strings.TrimSpace(req.DestAddress)
	if dest == "" {
		return nil, errors.New("dest_address required")
	}
	amountStr := strings.TrimSpace(req.Amount)
	if amountStr == "" {
		return nil, errors.New("amount required")
	}

	base := strings.TrimRight(l.svcCtx.Config.WalletCoreRestUrl, "/")
	url := fmt.Sprintf("%s/v1/withdraw", base)

	payload := struct {
		UserId      uint64 `json:"user_id"`
		Symbol      string `json:"symbol"`
		DestAddress string `json:"dest_address"`
		Amount      string `json:"amount"`
		Chain       string `json:"chain"`
	}{
		UserId:      req.UserId,
		Symbol:      symbol,
		DestAddress: dest,
		Amount:      amountStr,
		Chain:       "EVM",
	}

	out := new(types.WithdrawResp)
	err := postJSON(l.ctx, url, &payload, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

