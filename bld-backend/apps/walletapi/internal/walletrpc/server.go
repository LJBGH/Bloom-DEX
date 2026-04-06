package walletrpc

import (
	"context"

	walletpb "bld-backend/api/wallet"
	"bld-backend/apps/walletapi/internal/logic"
	"bld-backend/apps/walletapi/internal/svc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server 实现 walletpb.Wallet，与 REST GET /v1/assets 共用 ListAssets 逻辑。
type Server struct {
	walletpb.UnimplementedWalletServer
	SvcCtx *svc.ServiceContext
}

func NewServer(svcCtx *svc.ServiceContext) *Server {
	return &Server{SvcCtx: svcCtx}
}

// GetAssets 获取资产余额
func (s *Server) GetAssets(ctx context.Context, in *walletpb.GetAssetsRequest) (*walletpb.GetAssetsResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	if in.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}

	l := logic.NewListAssetsLogic(ctx, s.SvcCtx)
	resp, err := l.ListAssets(in.UserId, int(in.AssetId))
	if err != nil {
		switch err.Error() {
		case "user_id required":
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	}

	out := &walletpb.GetAssetsResponse{UserId: resp.UserId}
	for _, it := range resp.Items {
		out.Items = append(out.Items, &walletpb.AssetItem{
			Symbol:           it.Symbol,
			AssetId:          int32(it.AssetId),
			AvailableBalance: it.AvailableBalance,
			FrozenBalance:    it.FrozenBalance,
		})
	}
	return out, nil
}

// FreezeForOrder 冻结资产
func (s *Server) FreezeForOrder(ctx context.Context, in *walletpb.FreezeForOrderRequest) (*walletpb.FreezeForOrderResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	l := logic.NewOrderFreezeLogic(ctx, s.SvcCtx)
	fid, err := l.FreezeForOrder(in.UserId, int(in.AssetId), in.OrderId, in.Amount, in.TradingType)
	if err != nil {
		switch err.Error() {
		case "user_id required", "asset_id required", "order_id required", "amount must be > 0":
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case "trading_type must be SPOT or CONTRACT":
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case "active freeze already exists for this order and asset":
			return nil, status.Error(codes.AlreadyExists, err.Error())
		case "insufficient balance":
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	}
	return &walletpb.FreezeForOrderResponse{FreezeId: fid}, nil
}

// UnfreezeForOrder 解冻资产
func (s *Server) UnfreezeForOrder(ctx context.Context, in *walletpb.UnfreezeForOrderRequest) (*walletpb.UnfreezeForOrderResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	l := logic.NewOrderFreezeLogic(ctx, s.SvcCtx)
	err := l.UnfreezeForOrder(in.UserId, int(in.AssetId), in.OrderId, in.TradingType)
	if err != nil {
		switch err.Error() {
		case "user_id required", "asset_id required", "order_id required":
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case "trading_type must be SPOT or CONTRACT":
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case "insufficient frozen balance":
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	}
	return &walletpb.UnfreezeForOrderResponse{}, nil
}

// ApplySpotTrade 现货成交后同步结算钱包与 spot_fund_flows（trade_id 幂等）。
func (s *Server) ApplySpotTrade(ctx context.Context, in *walletpb.ApplySpotTradeRequest) (*walletpb.ApplySpotTradeResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	l := logic.NewApplySpotTradeLogic(ctx, s.SvcCtx)
	if err := l.Apply(in); err != nil {
		switch err.Error() {
		case "empty request", "trade_id required", "market_id and asset ids required", "order and user ids required", "taker_side must be BUY or SELL", "price and quantity must be > 0", "fees must be >= 0", "maker_fee exceeds notional":
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case "insufficient frozen balance", "insufficient available balance", "no active freeze row", "insufficient frozen on asset_freezes", "reduce amount must be > 0", "invalid frozen_amount", "invalid reduce amount":
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	}
	return &walletpb.ApplySpotTradeResponse{}, nil
}
