package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	walletpb "bld-backend/api/wallet"
	"bld-backend/apps/walletapi/internal/svc"
	"bld-backend/core/enum"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ApplySpotTradeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewApplySpotTradeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplySpotTradeLogic {
	return &ApplySpotTradeLogic{ctx: ctx, svcCtx: svcCtx}
}

// Apply 现货成交同步结算：扣减双方冻结、增加可用、手续费、写 spot_fund_flows；trade_id 幂等。
func (l *ApplySpotTradeLogic) Apply(in *walletpb.ApplySpotTradeRequest) error {
	if in == nil {
		return errors.New("empty request")
	}
	if in.TradeId == 0 {
		return errors.New("trade_id required")
	}
	if in.MarketId <= 0 || in.BaseAssetId <= 0 || in.QuoteAssetId <= 0 {
		return errors.New("market_id and asset ids required")
	}
	if in.MakerOrderId == 0 || in.TakerOrderId == 0 || in.MakerUserId == 0 || in.TakerUserId == 0 {
		return errors.New("order and user ids required")
	}
	side := strings.ToUpper(strings.TrimSpace(in.TakerSide))
	if side != enum.Buy.String() && side != enum.Sell.String() {
		return errors.New("taker_side must be BUY or SELL")
	}
	price := strings.TrimSpace(in.Price)
	qty := strings.TrimSpace(in.Quantity)
	if !ratIsPositive(price) || !ratIsPositive(qty) {
		return errors.New("price and quantity must be > 0")
	}
	mf := strings.TrimSpace(in.MakerFee)
	tf := strings.TrimSpace(in.TakerFee)
	if mf == "" {
		mf = "0"
	}
	if tf == "" {
		tf = "0"
	}
	if !ratNonNegString(mf) || !ratNonNegString(tf) {
		return errors.New("fees must be >= 0")
	}

	N, err := ratMulStr(price, qty)
	if err != nil {
		return err
	}
	// 成交金额是否大于手续费金额。
	nRat, _ := parseRat(N)
	// 报价币手续费金额。
	fmRat, _ := parseRat(mf)
	// 成交金额是否大于报价币手续费金额。
	if nRat.Cmp(fmRat) < 0 {
		return errors.New("maker_fee exceeds notional")
	}

	base := int(in.BaseAssetId)
	quote := int(in.QuoteAssetId)
	takerUID := in.TakerUserId
	makerUID := in.MakerUserId
	takerOrd := in.TakerOrderId
	makerOrd := in.MakerOrderId
	mid := in.MarketId
	tid := in.TradeId
	tt := enum.Spot.String()

	// 确保钱包余额表存在对应用户和资产的行。
	pairs := []struct {
		uid uint64
		aid int
	}{
		{takerUID, base}, {takerUID, quote}, {makerUID, base}, {makerUID, quote},
	}

	for _, p := range pairs {
		// 确保钱包余额表存在对应用户和资产的行。
		if err := l.svcCtx.WalletBalanceModel.EnsureRow(l.ctx, p.uid, p.aid); err != nil {
			return err
		}
	}

	// 在事务中执行函数。
	return l.svcCtx.WalletBalanceModel.RunInTransaction(l.ctx, func(ctx context.Context, s sqlx.Session) error {
		// 尝试插入 spot_trade_settlements 表。
		inserted, err := l.svcCtx.SpotTradeSettlementModel.TryInsertTx(ctx, s, tid)
		if err != nil {
			return err
		}
		if !inserted {
			return nil
		}
		// 结算吃单方买方。
		if side == enum.Buy.String() {
			return l.settleTakerBuy(ctx, s, tt, takerUID, makerUID, takerOrd, makerOrd, base, quote, mid, tid, N, qty, mf, tf)
		}
		return l.settleTakerSell(ctx, s, tt, takerUID, makerUID, takerOrd, makerOrd, base, quote, mid, tid, N, qty, mf, tf)
	})
}

// fundFlow 写 spot_fund_flows。
func (l *ApplySpotTradeLogic) fundFlow(ctx context.Context, s sqlx.Session, userID uint64, assetID int, marketID int32, orderID uint64, tradeID uint64, flowType, reason, availD, frozenD string) error {
	var oid sql.NullInt64
	if orderID > 0 {
		oid = sql.NullInt64{Int64: int64(orderID), Valid: true}
	}
	return l.svcCtx.SpotFundFlowModel.InsertTx(ctx, s, userID, assetID, marketID, oid, tradeID, flowType, reason, availD, frozenD)
}

// feePositive 手续费是否大于0。
func feePositive(f string) bool {
	r, err := parseRat(f)
	if err != nil {
		return false
	}
	return r.Sign() > 0
}

// settleTakerBuy 吃单方买方结算。
func (l *ApplySpotTradeLogic) settleTakerBuy(ctx context.Context, s sqlx.Session, tt string, takerUID, makerUID, takerOrd, makerOrd uint64, base, quote int, marketID int32, tradeID uint64, notional, qty, makerFee, takerFee string) error {
	// 扣减吃单方冻结。
	if err := l.svcCtx.WalletBalanceModel.SubtractFrozenTx(ctx, s, takerUID, quote, notional); err != nil {
		return err
	}
	// 减少吃单方活跃冻结。
	if err := l.svcCtx.AssetFreezeModel.ReduceFrozenForActiveOrderTx(ctx, s, takerUID, quote, takerOrd, tt, notional); err != nil {
		return err
	}
	// 写成交资金流水。
	if err := l.fundFlow(ctx, s, takerUID, quote, marketID, takerOrd, tradeID, enum.TradeExecuted.String(), "", "0", "-"+notional); err != nil {
		return err
	}

	// 增加吃单方可用。
	if err := l.svcCtx.WalletBalanceModel.AddAvailableTx(ctx, s, takerUID, base, qty); err != nil {
		return err
	}
	// 写成交资金流水。
	if err := l.fundFlow(ctx, s, takerUID, base, marketID, takerOrd, tradeID, enum.TradeExecuted.String(), "", qty, "0"); err != nil {
		return err
	}

	// 手续费是否大于0。
	if feePositive(takerFee) {
		// 扣减吃单方可用。
		if err := l.svcCtx.WalletBalanceModel.DebitAvailableTx(ctx, s, takerUID, quote, takerFee); err != nil {
			return err
		}
		if err := l.fundFlow(ctx, s, takerUID, quote, marketID, takerOrd, tradeID, enum.Fees.String(), "", "-"+takerFee, "0"); err != nil {
			return err
		}
	}

	// 扣减maker方冻结。
	if err := l.svcCtx.WalletBalanceModel.SubtractFrozenTx(ctx, s, makerUID, base, qty); err != nil {
		return err
	}
	// 减少maker方活跃冻结。
	if err := l.svcCtx.AssetFreezeModel.ReduceFrozenForActiveOrderTx(ctx, s, makerUID, base, makerOrd, tt, qty); err != nil {
		return err
	}
	// 写成交资金流水。
	if err := l.fundFlow(ctx, s, makerUID, base, marketID, makerOrd, tradeID, enum.TradeExecuted.String(), "", "0", "-"+qty); err != nil {
		return err
	}

	// 增加maker方可用。
	if err := l.svcCtx.WalletBalanceModel.AddAvailableTx(ctx, s, makerUID, quote, notional); err != nil {
		return err
	}
	// 写成交资金流水。
	if err := l.fundFlow(ctx, s, makerUID, quote, marketID, makerOrd, tradeID, enum.TradeExecuted.String(), "", notional, "0"); err != nil {
		return err
	}
	// 手续费是否大于0。
	if feePositive(makerFee) {
		// 扣减maker方可用。
		if err := l.svcCtx.WalletBalanceModel.DebitAvailableTx(ctx, s, makerUID, quote, makerFee); err != nil {
			return err
		}
		if err := l.fundFlow(ctx, s, makerUID, quote, marketID, makerOrd, tradeID, enum.Fees.String(), "", "-"+makerFee, "0"); err != nil {
			return err
		}
	}
	return nil
}

// settleTakerSell 吃单方卖方结算。
func (l *ApplySpotTradeLogic) settleTakerSell(ctx context.Context, s sqlx.Session, tt string, takerUID, makerUID, takerOrd, makerOrd uint64, base, quote int, marketID int32, tradeID uint64, notional, qty, makerFee, takerFee string) error {
	if err := l.svcCtx.WalletBalanceModel.SubtractFrozenTx(ctx, s, takerUID, base, qty); err != nil {
		return err
	}
	if err := l.svcCtx.AssetFreezeModel.ReduceFrozenForActiveOrderTx(ctx, s, takerUID, base, takerOrd, tt, qty); err != nil {
		return err
	}
	if err := l.fundFlow(ctx, s, takerUID, base, marketID, takerOrd, tradeID, enum.TradeExecuted.String(), "", "0", "-"+qty); err != nil {
		return err
	}

	if err := l.svcCtx.WalletBalanceModel.AddAvailableTx(ctx, s, takerUID, quote, notional); err != nil {
		return err
	}
	if err := l.fundFlow(ctx, s, takerUID, quote, marketID, takerOrd, tradeID, enum.TradeExecuted.String(), "", notional, "0"); err != nil {
		return err
	}
	if feePositive(takerFee) {
		if err := l.svcCtx.WalletBalanceModel.DebitAvailableTx(ctx, s, takerUID, quote, takerFee); err != nil {
			return err
		}
		if err := l.fundFlow(ctx, s, takerUID, quote, marketID, takerOrd, tradeID, enum.Fees.String(), "", "-"+takerFee, "0"); err != nil {
			return err
		}
	}

	if err := l.svcCtx.WalletBalanceModel.SubtractFrozenTx(ctx, s, makerUID, quote, notional); err != nil {
		return err
	}
	if err := l.svcCtx.AssetFreezeModel.ReduceFrozenForActiveOrderTx(ctx, s, makerUID, quote, makerOrd, tt, notional); err != nil {
		return err
	}
	if err := l.fundFlow(ctx, s, makerUID, quote, marketID, makerOrd, tradeID, enum.TradeExecuted.String(), "", "0", "-"+notional); err != nil {
		return err
	}

	if err := l.svcCtx.WalletBalanceModel.AddAvailableTx(ctx, s, makerUID, base, qty); err != nil {
		return err
	}
	if err := l.fundFlow(ctx, s, makerUID, base, marketID, makerOrd, tradeID, enum.TradeExecuted.String(), "", qty, "0"); err != nil {
		return err
	}
	if feePositive(makerFee) {
		if err := l.svcCtx.WalletBalanceModel.DebitAvailableTx(ctx, s, makerUID, quote, makerFee); err != nil {
			return err
		}
		if err := l.fundFlow(ctx, s, makerUID, quote, marketID, makerOrd, tradeID, enum.Fees.String(), "", "-"+makerFee, "0"); err != nil {
			return err
		}
	}
	return nil
}
