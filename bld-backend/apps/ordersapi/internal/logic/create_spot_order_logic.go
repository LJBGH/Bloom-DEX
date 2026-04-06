package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	walletpb "bld-backend/api/wallet"
	"bld-backend/core/enum"
	"bld-backend/core/model"
	"bld-backend/apps/ordersapi/internal/svc"
	"bld-backend/apps/ordersapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/status"
)

type CreateSpotOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSpotOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSpotOrderLogic {
	return &CreateSpotOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateSpotOrder 创建现货订单
func (l *CreateSpotOrderLogic) CreateSpotOrder(req *types.CreateSpotOrderReq) (*types.CreateSpotOrderResp, error) {
	if req.UserId == 0 {
		return nil, errors.New("user_id required")
	}
	if req.MarketId <= 0 {
		return nil, errors.New("market_id required")
	}

	// 方向
	side := strings.ToUpper(strings.TrimSpace(req.Side))
	if side != enum.Buy.String() && side != enum.Sell.String() {
		return nil, errors.New("side must be BUY or SELL")
	}

	// 订单类型
	orderType := strings.ToUpper(strings.TrimSpace(req.OrderType))
	if orderType != enum.Limit.String() && orderType != enum.Market.String() {
		return nil, errors.New("order_type must be LIMIT or MARKET")
	}

	// 解析下单维度
	amountInputMode, err := resolveAmountInputMode(orderType, req.AmountInputMode)
	if err != nil {
		return nil, err
	}

	// 规范化价格、数量、市价买单成交额
	price, quantity, maxQuote, perr := normalizeSpotOrderParams(req, side, orderType)
	if perr != nil {
		return nil, perr
	}

	// 检查交易对是否存在并处于活跃状态
	mkt, err := l.svcCtx.SpotMarketModel.GetByID(l.ctx, req.MarketId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("market not found")
		}
		return nil, err
	}
	if strings.ToUpper(strings.TrimSpace(mkt.Status)) != enum.SMS_Active.String() {
		return nil, errors.New("market not active")
	}

	// 校验余额，并计算钱包 RPC 冻结金额（不落库、不进 Kafka）
	var walletFreezeQuote, walletFreezeBase *string
	switch side {
	case enum.Buy.String():
		if err := l.validateSpotBuyWallet(mkt, req.UserId, orderType, price, quantity, maxQuote); err != nil {
			return nil, err
		}
		w, werr := spotBuyWalletFreezeAmount(orderType, price, quantity, maxQuote)
		if werr != nil {
			return nil, werr
		}
		walletFreezeQuote = w
	case enum.Sell.String():
		if err := l.validateSpotSellWallet(mkt, req.UserId, quantity); err != nil {
			return nil, err
		}
		walletFreezeBase = spotSellWalletFreezeAmount(quantity)
	default:
		return nil, errors.New("invalid side")
	}

	// 生成订单 ID
	if l.svcCtx.IDGen == nil {
		return nil, errors.New("id generator not initialized")
	}
	orderID := l.svcCtx.IDGen.Next()

	if l.svcCtx.Wallet == nil {
		return nil, errors.New("wallet service unavailable")
	}

	if err := l.walletFreezeSpotOrder(req.UserId, mkt, side, orderID, walletFreezeQuote, walletFreezeBase); err != nil {
		return nil, err
	}

	id, err := l.svcCtx.SpotOrderModel.Create(l.ctx, orderID, req.UserId, req.MarketId, side, orderType, amountInputMode, price, quantity, req.ClientOrderId)
	if err != nil {
		_ = l.walletUnfreezeSpotOrder(req.UserId, mkt, side, orderID, walletFreezeQuote, walletFreezeBase)
		return nil, err
	}

	if l.svcCtx.KafkaProducer == nil {
		return nil, errors.New("kafka producer not initialized")
	}
	if l.svcCtx.Config.Kafka.Partitions <= 0 {
		return nil, errors.New("kafka partitions must be > 0")
	}

	// 根据代币对 ID 计算分区，并设置 key
	partition := int32(req.MarketId) % l.svcCtx.Config.Kafka.Partitions
	key := strconv.Itoa(req.MarketId)

	msg := &model.SpotOrderKafkaMsg{
		OrderID:         id,
		UserID:          req.UserId,
		MarketID:        req.MarketId,
		Side:            side,
		OrderType:       orderType,
		AmountInputMode: amountInputMode,
		Price:           price,
		Quantity:        quantity,
		ClientOrderID:   req.ClientOrderId,
		Status:          enum.SOS_Pending.String(),
	}
	if err := l.svcCtx.KafkaProducer.Publish(l.ctx, l.svcCtx.Config.Kafka.Topic, partition, key, msg); err != nil {
		return nil, err
	}

	return &types.CreateSpotOrderResp{
		OrderId: id,
		Status:  enum.SOS_Pending.String(),
	}, nil
}

// parseMarketBuyBaseQty 市价买单 base：空或 0 视为未填数量；正数返回规范化字符串。
func parseMarketBuyBaseQty(raw string) (canonical string, hasPositive bool, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "0", false, nil
	}
	r := new(big.Rat)
	if _, ok := r.SetString(s); !ok {
		return "", false, errors.New("invalid quantity for MARKET BUY")
	}
	if r.Sign() < 0 {
		return "", false, errors.New("quantity must be >= 0 for MARKET BUY")
	}
	if r.Sign() == 0 {
		return "0", false, nil
	}
	return ratToDecimalString(r), true, nil
}

// normalizeMarketBuyParams 市价买：max_quote_amount（成交额）与 quantity（买入量）至少其一；仅数量时需 reference_price 估算报价冻结。
func normalizeMarketBuyParams(req *types.CreateSpotOrderReq) (price *string, quantity string, maxQuote *string, err error) {
	mqStr := ""
	if req.MaxQuoteAmount != nil {
		mqStr = strings.TrimSpace(*req.MaxQuoteAmount)
	}
	hasMQ := mqStr != "" && isPositiveDecimal(mqStr)

	refStr := ""
	if req.ReferencePrice != nil {
		refStr = strings.TrimSpace(*req.ReferencePrice)
	}
	hasRef := refStr != "" && isPositiveDecimal(refStr)

	qCanon, qtyPos, err := parseMarketBuyBaseQty(req.Quantity)
	if err != nil {
		return nil, "", nil, err
	}
	if !hasMQ && !qtyPos {
		return nil, "", nil, errors.New("MARKET BUY requires either max_quote_amount > 0 or quantity > 0")
	}
	if hasMQ {
		mq := mqStr
		return nil, qCanon, &mq, nil
	}
	if !hasRef {
		return nil, "", nil, errors.New("MARKET BUY with quantity requires max_quote_amount or reference_price")
	}
	qtyRat, err := parseDecimalRat(qCanon)
	if err != nil {
		return nil, "", nil, fmt.Errorf("quantity: %w", err)
	}
	refRat, err := parseDecimalRat(refStr)
	if err != nil {
		return nil, "", nil, fmt.Errorf("reference_price: %w", err)
	}
	buf := big.NewRat(1005, 1000)
	need := new(big.Rat).Mul(qtyRat, refRat)
	need.Mul(need, buf)
	s := ratToDecimalString(need)
	return nil, qCanon, &s, nil
}

// resolveAmountInputMode 限价单固定 QUANTITY；市价单必须由前端传 QUANTITY（按数量）或 TURNOVER（按成交额）。
func resolveAmountInputMode(orderType string, raw *string) (string, error) {
	if orderType == enum.Limit.String() {
		return enum.Quantity.String(), nil
	}
	if raw == nil {
		return "", errors.New("amount_input_mode is required for MARKET orders (QUANTITY or TURNOVER)")
	}
	s := strings.ToUpper(strings.TrimSpace(*raw))
	if s != enum.Quantity.String() && s != enum.Turnover.String() {
		return "", errors.New("amount_input_mode must be QUANTITY or TURNOVER")
	}
	return s, nil
}

// normalizeSpotOrderParams 按方向与订单类型解析价格、数量、市价买单成交额。
func normalizeSpotOrderParams(req *types.CreateSpotOrderReq, side, orderType string) (price *string, quantity string, maxQuote *string, err error) {
	q := strings.TrimSpace(req.Quantity)
	switch orderType {
	case enum.Limit.String():
		if req.MaxQuoteAmount != nil && strings.TrimSpace(*req.MaxQuoteAmount) != "" {
			return nil, "", nil, errors.New("max_quote_amount is only for MARKET BUY; omit for LIMIT orders (turnover is price×quantity, computed server-side)")
		}
		if req.Price == nil || strings.TrimSpace(*req.Price) == "" {
			return nil, "", nil, errors.New("price required for LIMIT order")
		}
		p := strings.TrimSpace(*req.Price)
		if !isPositiveDecimal(p) {
			return nil, "", nil, errors.New("price must be > 0")
		}
		if q == "" {
			return nil, "", nil, errors.New("quantity required for LIMIT order")
		}
		if !isPositiveDecimal(q) {
			return nil, "", nil, errors.New("quantity must be > 0")
		}
		return &p, q, nil, nil
	case enum.Market.String():
		// 市价单不使用 price（即使客户端误传，此处也返回 nil）
		if side == enum.Buy.String() {
			return normalizeMarketBuyParams(req)
		}
		if q == "" {
			return nil, "", nil, errors.New("quantity required for MARKET SELL order")
		}
		if !isPositiveDecimal(q) {
			return nil, "", nil, errors.New("quantity must be > 0")
		}
		return nil, q, nil, nil
	default:
		return nil, "", nil, errors.New("invalid order_type")
	}
}

// walletAssetsSnapshot 获取钱包资产快照
func (l *CreateSpotOrderLogic) walletAssetsSnapshot(userID uint64) (*walletpb.GetAssetsResponse, error) {
	if l.svcCtx.Wallet == nil {
		return nil, errors.New("wallet service unavailable")
	}
	resp, err := l.svcCtx.Wallet.GetAssets(l.ctx, &walletpb.GetAssetsRequest{
		UserId:  userID,
		AssetId: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("wallet GetAssets: %w", err)
	}
	return resp, nil
}

// validateSpotBuyWallet 买入：限价校验报价币可用 ≥ price×数量；市价校验报价币可用 ≥ max_quote_amount。
func (l *CreateSpotOrderLogic) validateSpotBuyWallet(mkt *model.SpotMarket, userID uint64, orderType string, price *string, quantity string, maxQuote *string) error {
	resp, err := l.walletAssetsSnapshot(userID)
	if err != nil {
		return err
	}
	available, err := sumAvailableBySymbol(resp, mkt.QuoteSymbol)
	if err != nil {
		return fmt.Errorf("wallet available for %s: %w", mkt.QuoteSymbol, err)
	}
	switch orderType {
	case enum.Limit.String():
		if price == nil {
			return errors.New("price required")
		}
		p, err := parseDecimalRat(*price)
		if err != nil {
			return err
		}
		qty, err := parseDecimalRat(quantity)
		if err != nil {
			return err
		}
		need := new(big.Rat).Mul(p, qty)
		if available.Cmp(need) < 0 {
			return errors.New("insufficient quote balance")
		}
	case enum.Market.String():
		if maxQuote == nil {
			return errors.New("max_quote_amount required for MARKET BUY")
		}
		budget, err := parseDecimalRat(*maxQuote)
		if err != nil {
			return err
		}
		if available.Cmp(budget) < 0 {
			return errors.New("insufficient quote balance")
		}
	default:
		return errors.New("invalid order_type")
	}
	return nil
}

// validateSpotSellWallet 卖出：校验基础币可用 ≥ 卖出数量（限价/市价相同）。
func (l *CreateSpotOrderLogic) validateSpotSellWallet(mkt *model.SpotMarket, userID uint64, quantity string) error {
	resp, err := l.walletAssetsSnapshot(userID)
	if err != nil {
		return err
	}
	available, err := sumAvailableBySymbol(resp, mkt.BaseSymbol)
	if err != nil {
		return fmt.Errorf("wallet available for %s: %w", mkt.BaseSymbol, err)
	}
	qty, err := parseDecimalRat(quantity)
	if err != nil {
		return err
	}
	if available.Cmp(qty) < 0 {
		return errors.New("insufficient base balance")
	}
	return nil
}

// spotBuyWalletFreezeAmount 买单在钱包侧应冻结的报价币数量：限价=price×quantity；市价=max_quote_amount。
func spotBuyWalletFreezeAmount(orderType string, price *string, quantity string, maxQuote *string) (*string, error) {
	switch orderType {
	case enum.Limit.String():
		if price == nil {
			return nil, errors.New("price required")
		}
		p, err := parseDecimalRat(*price)
		if err != nil {
			return nil, err
		}
		qty, err := parseDecimalRat(quantity)
		if err != nil {
			return nil, err
		}
		need := new(big.Rat).Mul(p, qty)
		s := ratToDecimalString(need)
		return &s, nil
	case enum.Market.String():
		if maxQuote == nil {
			return nil, errors.New("max_quote_amount required for MARKET BUY")
		}
		s := strings.TrimSpace(*maxQuote)
		return &s, nil
	default:
		return nil, errors.New("invalid order_type")
	}
}

// spotSellWalletFreezeAmount 卖单在钱包侧应冻结的基础币数量（与 quantity 一致，规范化小数格式）。
func spotSellWalletFreezeAmount(quantity string) *string {
	qty, err := parseDecimalRat(quantity)
	if err != nil {
		s := strings.TrimSpace(quantity)
		return &s
	}
	s := ratToDecimalString(qty)
	return &s
}

// ratToDecimalString 将 big.Rat 转换为十进制字符串
func ratToDecimalString(r *big.Rat) string {
	if r == nil {
		return "0"
	}
	s := r.FloatString(18)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

// sumAvailableBySymbol 与 ListAssets 展示一致：同名资产（多 asset_id）累加可用余额。
func sumAvailableBySymbol(resp *walletpb.GetAssetsResponse, symbol string) (*big.Rat, error) {
	want := strings.TrimSpace(symbol)
	if want == "" {
		return big.NewRat(0, 1), nil
	}
	var sum *big.Rat
	for _, it := range resp.GetItems() {
		if !strings.EqualFold(strings.TrimSpace(it.GetSymbol()), want) {
			continue
		}
		v, err := parseDecimalRat(it.GetAvailableBalance())
		if err != nil {
			return nil, err
		}
		if sum == nil {
			sum = new(big.Rat).Set(v)
		} else {
			sum.Add(sum, v)
		}
	}
	if sum == nil {
		return big.NewRat(0, 1), nil
	}
	return sum, nil
}

// parseDecimalRat 解析十进制字符串为 big.Rat
func parseDecimalRat(s string) (*big.Rat, error) {
	r := new(big.Rat)
	if _, ok := r.SetString(strings.TrimSpace(s)); !ok {
		return nil, fmt.Errorf("invalid decimal: %q", s)
	}
	return r, nil
}

// isPositiveDecimal 判断十进制字符串是否为正数
func isPositiveDecimal(s string) bool {
	r := new(big.Rat)
	if _, ok := r.SetString(strings.TrimSpace(s)); !ok {
		return false
	}
	return r.Sign() > 0
}

// walletFreezeSpotOrder 冻结资产
func (l *CreateSpotOrderLogic) walletFreezeSpotOrder(userID uint64, mkt *model.SpotMarket, side string, orderID uint64, frozenQuote, frozenBase *string) error {
	switch side {
	case enum.Buy.String():
		if frozenQuote == nil || strings.TrimSpace(*frozenQuote) == "" {
			return errors.New("missing frozen quote amount")
		}
		_, err := l.svcCtx.Wallet.FreezeForOrder(l.ctx, &walletpb.FreezeForOrderRequest{
			UserId:      userID,
			AssetId:     int32(mkt.QuoteAssetID),
			OrderId:     orderID,
			Amount:      strings.TrimSpace(*frozenQuote),
			TradingType: enum.Spot.String(),
		})
		return grpcWalletErr(err)
	case enum.Sell.String():
		if frozenBase == nil || strings.TrimSpace(*frozenBase) == "" {
			return errors.New("missing frozen base amount")
		}
		_, err := l.svcCtx.Wallet.FreezeForOrder(l.ctx, &walletpb.FreezeForOrderRequest{
			UserId:      userID,
			AssetId:     int32(mkt.BaseAssetID),
			OrderId:     orderID,
			Amount:      strings.TrimSpace(*frozenBase),
			TradingType: enum.Spot.String(),
		})
		return grpcWalletErr(err)
	default:
		return errors.New("invalid side")
	}
}

// walletUnfreezeSpotOrder 解冻资产
func (l *CreateSpotOrderLogic) walletUnfreezeSpotOrder(userID uint64, mkt *model.SpotMarket, side string, orderID uint64, frozenQuote, frozenBase *string) error {
	switch side {
	case enum.Buy.String():
		if frozenQuote == nil || strings.TrimSpace(*frozenQuote) == "" {
			return nil
		}
		_, err := l.svcCtx.Wallet.UnfreezeForOrder(l.ctx, &walletpb.UnfreezeForOrderRequest{
			UserId:      userID,
			AssetId:     int32(mkt.QuoteAssetID),
			OrderId:     orderID,
			TradingType: enum.Spot.String(),
		})
		return grpcWalletErr(err)
	case enum.Sell.String():
		if frozenBase == nil || strings.TrimSpace(*frozenBase) == "" {
			return nil
		}
		_, err := l.svcCtx.Wallet.UnfreezeForOrder(l.ctx, &walletpb.UnfreezeForOrderRequest{
			UserId:      userID,
			AssetId:     int32(mkt.BaseAssetID),
			OrderId:     orderID,
			TradingType: enum.Spot.String(),
		})
		return grpcWalletErr(err)
	default:
		return nil
	}
}

// grpcWalletErr 将 gRPC 错误转换为标准错误
func grpcWalletErr(err error) error {
	if err == nil {
		return nil
	}
	if st, ok := status.FromError(err); ok {
		return errors.New(st.Message())
	}
	return err
}
