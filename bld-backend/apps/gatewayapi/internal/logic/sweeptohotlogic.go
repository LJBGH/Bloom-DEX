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

type SweepToHotLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSweepToHotLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SweepToHotLogic {
	return &SweepToHotLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 扫热钱包逻辑
func (l *SweepToHotLogic) SweepToHot(req *types.SweepToHotReq) (*types.SweepToHotResp, error) {
	if req.UserId == 0 {
		return nil, errors.New("user_id required")
	}
	symbol := strings.TrimSpace(req.Symbol)
	if symbol == "" {
		return nil, errors.New("symbol required")
	}
	amountStr := strings.TrimSpace(req.Amount)
	if amountStr == "" {
		amountStr = "0"
	}

	base := strings.TrimRight(l.svcCtx.Config.WalletCoreRestUrl, "/")
	url := fmt.Sprintf("%s/v1/wallet/sweep/hot", base)

	payload := struct {
		UserId uint64 `json:"user_id"`
		Symbol string `json:"symbol"`
		Amount string `json:"amount"`
		Chain  string `json:"chain"`
	}{
		UserId: req.UserId,
		Symbol: symbol,
		Amount: amountStr,
		Chain:  "EVM",
	}

	out := new(types.SweepToHotResp)
	err := postJSON(l.ctx, url, &payload, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}
