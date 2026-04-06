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

type ListAssetsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAssetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAssetsLogic {
	return &ListAssetsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListAssets 按用户列出资产及余额。
// assetID 可选；>0 时仅返回该资产。
func (l *ListAssetsLogic) ListAssets(userID uint64, assetID int) (*types.AssetListResp, error) {
	if userID == 0 {
		return nil, errors.New("user_id required")
	}

	// 一次 JOIN 查询拿到余额 + 资产信息
	var (
		rows []model.WalletBalanceWithAsset
		err  error
	)
	if assetID > 0 {
		rows, err = l.svcCtx.WalletBalanceModel.ListWithAssetByUserAndAssetID(l.ctx, userID, assetID)
	} else {
		rows, err = l.svcCtx.WalletBalanceModel.ListWithAssetByUser(l.ctx, userID)
	}
	if err != nil {
		return nil, err
	}

	items := make([]types.AssetItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, types.AssetItem{
			Symbol:           r.Symbol,
			AssetId:          r.AssetID,
			AvailableBalance: formatByDecimals(r.AvailableBalance, r.Decimals),
			FrozenBalance:    formatByDecimals(r.FrozenBalance, r.Decimals),
		})
	}

	return &types.AssetListResp{
		UserId: userID,
		Items:  items,
	}, nil
}

func formatByDecimals(v string, decimals int) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return "0"
	}
	if decimals <= 0 {
		// drop fractional part if any
		if i := strings.IndexByte(s, '.'); i >= 0 {
			s = s[:i]
		}
		if s == "" || s == "-" {
			return "0"
		}
		return s
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = strings.TrimPrefix(s, "-")
	}
	intPart := s
	fracPart := ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart = s[:i]
		fracPart = s[i+1:]
	}
	if intPart == "" {
		intPart = "0"
	}
	// keep exactly `decimals` digits after dot:
	// - truncate if too long
	// - pad with zeros if too short
	if len(fracPart) > decimals {
		fracPart = fracPart[:decimals]
	} else if len(fracPart) < decimals {
		fracPart = fracPart + strings.Repeat("0", decimals-len(fracPart))
	}
	out := intPart + "." + fracPart
	if neg && out != "0" {
		out = "-" + out
	}
	return out
}

