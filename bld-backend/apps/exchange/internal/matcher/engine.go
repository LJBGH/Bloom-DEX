package matcher

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"

	walletpb "bld-backend/api/wallet"
	"bld-backend/apps/exchange/internal/book"
	"bld-backend/apps/exchange/internal/mq"
	"bld-backend/apps/exchange/internal/store"
	ratutil "bld-backend/apps/exchange/internal/util/rat"
	"bld-backend/core/enum"
	"bld-backend/core/model"

	"github.com/zeromicro/go-zero/core/logx"
)

// Engine 按 market_id 维护订单簿并执行撮合（单互斥锁，进程内单实例）。
type Engine struct {
	mu         sync.Mutex
	store      *store.SpotStore
	books      map[int]*book.OrderBook
	depth      mq.DepthPublisher
	depthTopic string
	tradeTopic string
	depthParts int
	seq        map[int]int64

	wallet walletpb.WalletClient

	depthDisabledOnce sync.Once
}

func New(st *store.SpotStore, depth mq.DepthPublisher, depthTopic, tradeTopic string, depthParts int, wallet walletpb.WalletClient) *Engine {
	if depthParts <= 0 {
		depthParts = 1
	}
	return &Engine{
		store:      st,
		books:      make(map[int]*book.OrderBook),
		depth:      depth,
		depthTopic: depthTopic,
		tradeTopic: tradeTopic,
		depthParts: depthParts,
		seq:        make(map[int]int64),
		wallet:     wallet,
	}
}

// Recover 启动时按订单 id 升序重放未完结限价单，等价于在线消费 Kafka，避免仅 RestoreLimit 导致「可成交买卖同时挂簿却不撮」。
// 须在 Kafka consumer 启动前完成（worker 已保证）。全程持有 e.mu，调用 handleOrderMessage（不再二次加锁）。
func (e *Engine) Recover(ctx context.Context) error {
	ids, err := e.store.ListActiveMarketIDs(ctx)
	if err != nil {
		return err
	}
	for _, mid := range ids {
		rows, err := e.store.ListOpenLimitOrders(ctx, mid)
		if err != nil {
			return fmt.Errorf("list open orders market %d: %w", mid, err)
		}
		e.mu.Lock()
		e.books[mid] = book.NewOrderBook(mid)
		e.seq[mid] = 0
		replayed := 0
		for i := range rows {
			r := &rows[i]
			if !r.Price.Valid || r.Price.String == "" {
				continue
			}
			if _, errP := ratutil.MustPositive(r.Price.String); errP != nil {
				continue
			}
			rem, err := ratutil.Parse(r.RemainingQuantity)
			if err != nil || rem.Sign() == 0 {
				continue
			}
			msg := &model.SpotOrderKafkaMsg{
				OrderID:  r.ID,
				MarketID: mid,
				Status:   enum.SOS_Pending.String(),
			}
			if err := e.handleOrderMessage(ctx, msg); err != nil {
				e.mu.Unlock()
				return fmt.Errorf("recover replay order_id=%d market_id=%d: %w", r.ID, mid, err)
			}
			replayed++
		}
		e.publishDepthUnsafe(ctx, mid)
		e.mu.Unlock()
		logx.Infof("exchange recover market_id=%d replayed_open_limits=%d (from %d rows)", mid, replayed, len(rows))
	}
	return nil
}

// getBook 获取市场订单簿。
func (e *Engine) getBook(mid int) *book.OrderBook {
	b := e.books[mid]
	if b == nil {
		b = book.NewOrderBook(mid)
		e.books[mid] = b
	}
	return b
}

// HandleKafkaMessage 消费一笔新订单（以 DB 状态为准）。须独占 e.mu；Recover 阶段由 Recover 持锁调用 handleOrderMessage。
func (e *Engine) HandleKafkaMessage(ctx context.Context, msg *model.SpotOrderKafkaMsg) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.handleOrderMessage(ctx, msg)
}

// handleOrderMessage 单笔撮合（调用方须已持有 e.mu）。
func (e *Engine) handleOrderMessage(ctx context.Context, msg *model.SpotOrderKafkaMsg) error {
	if msg.OrderID == 0 || msg.MarketID <= 0 {
		return nil
	}
	if msg.Status != enum.SOS_Pending.String() {
		logx.Infof("exchange skip order_id=%d: kafka status=%q (need %s)", msg.OrderID, msg.Status, enum.SOS_Pending.String())
		return nil
	}

	ord, err := e.store.GetOrder(ctx, msg.OrderID)
	if err != nil {
		return err
	}
	if ord == nil {
		return fmt.Errorf("order %d not in DB yet", msg.OrderID)
	}
	if ord.Status != enum.SOS_Pending.String() && ord.Status != enum.SOS_PartiallyFilled.String() {
		logx.Infof("exchange skip order_id=%d: db status=%q", ord.ID, ord.Status)
		return nil
	}

	rem, err := ratutil.Parse(ord.RemainingQuantity)
	if err != nil {
		return err
	}
	if rem.Sign() == 0 {
		logx.Infof("exchange skip order_id=%d: remaining_quantity is 0", ord.ID)
		return nil
	}

	mkt, err := e.store.GetMarket(ctx, ord.MarketID)
	if err != nil {
		return err
	}
	if mkt == nil {
		return errors.New("market not found")
	}

	ob := e.getBook(ord.MarketID)
	// Kafka 重复投递或 Recover 后 offset 仍指向已重放订单：簿上已有该单则跳过，避免二次撮合。
	if ob.HasRestingOrder(ord.ID) {
		logx.Infof("exchange skip order_id=%d: already resting on book (idempotent)", ord.ID)
		return nil
	}

	var trades []book.Trade

	// 匹配订单
	switch ord.OrderType {
	// 限价单
	case enum.Limit.String():
		// 解析价格
		if !ord.Price.Valid || ord.Price.String == "" {
			return errors.New("limit order missing price")
		}
		// 解析价格。
		limitPx, err := ratutil.MustPositive(ord.Price.String)
		if err != nil {
			return err
		}
		// 匹配限价单。
		if ord.Side == enum.Buy.String() {
			// 匹配限价买单。
			trades, rem = ob.MatchLimitBuy(limitPx, rem, ord.ID, ord.UserID)
		} else if ord.Side == enum.Sell.String() {
			// 匹配限价卖单。
			trades, rem = ob.MatchLimitSell(limitPx, rem, ord.ID, ord.UserID)
		} else {
			// 未知方向。
			return fmt.Errorf("unknown side %q", ord.Side)
		}
		// 如果剩余数量大于0。
		if rem.Sign() > 0 {
			if ob.HasRestingOrder(ord.ID) {
				return nil
			}
			rest := &book.Resting{
				OrderID:   ord.ID,
				UserID:    ord.UserID,
				CreatedAt: ord.CreatedAt.UnixMilli(),
				Price:     new(big.Rat).Set(limitPx),
				RemQty:    new(big.Rat).Set(rem),
			}
			if ord.Side == enum.Buy.String() {
				ob.AddBid(rest)
			} else {
				ob.AddAsk(rest)
			}
		}

	// 市价单
	case enum.Market.String():
		// 匹配订单
		if ord.Side == enum.Buy.String() {
			// 匹配市价买单。
			trades, rem = ob.MatchMarketBuy(rem, ord.ID, ord.UserID)
		} else if ord.Side == enum.Sell.String() {
			// 匹配市价卖单。
			trades, rem = ob.MatchMarketSell(rem, ord.ID, ord.UserID)
		} else {
			return fmt.Errorf("unknown side %q", ord.Side)
		}
	default:
		return fmt.Errorf("unsupported order_type %q", ord.OrderType)
	}

	// 如果成交数量为 0
	if len(trades) == 0 {
		// 如果订单类型为市价单且剩余数量大于 0
		if ord.OrderType == enum.Market.String() && rem.Sign() > 0 {
			return e.persistMarketReject(ctx, ord)
		}
		// 限价无成交仅挂簿：DB 未变，内存簿已更新
		e.publishDepthUnsafe(ctx, ord.MarketID)
		if ord.OrderType == enum.Limit.String() && rem.Sign() > 0 {
			logx.Infof("exchange resting order_id=%d market_id=%d side=%s rem=%s", ord.ID, ord.MarketID, ord.Side, ratutil.StringTrim(rem))
		}
		return nil
	}

	// 构建持久化数据
	tradeRows, updates, err := e.buildPersistence(ctx, mkt, trades, ord)
	if err != nil {
		return err
	}

	// 执行匹配交易
	tradeIDs, err := e.store.RunMatchTx(ctx, tradeRows, updates)
	if err != nil {
		// 如果执行匹配交易失败，则重新加载市场订单簿。
		if rerr := e.reloadMarketBook(ctx, ord.MarketID); rerr != nil {
			logx.Errorf("reload market %d after tx err: %v", ord.MarketID, rerr)
		}
		return err
	}

	// 应用钱包结算。
	e.applyWalletSettlements(ctx, mkt, ord, trades, tradeRows, tradeIDs)

	// 成交后若簿上 maker 已完全成交，Match 过程已从内存摘除；限价 taker 剩余已在簿上添加
	e.publishPublicTrades(ctx, ord, trades)
	// 发送盘口快照到 Kafka。
	e.publishDepthUnsafe(ctx, ord.MarketID)
	// 记录成交信息。
	logx.Infof("exchange matched order_id=%d trades=%d", ord.ID, len(trades))
	return nil
}

// persistMarketReject 持久化市场拒绝订单。
func (e *Engine) persistMarketReject(ctx context.Context, ord *store.SpotOrderRow) error {
	var avg sql.NullString
	if ord.AvgFillPrice.Valid {
		avg = ord.AvgFillPrice
	}
	up := store.OrderFillUpdate{
		OrderID:           ord.ID,
		FilledQuantity:    ord.FilledQuantity,
		RemainingQuantity: ord.RemainingQuantity,
		Status:            enum.SOS_Rejected.String(),
		AvgFillPrice:      avg,
	}
	_, err := e.store.RunMatchTx(ctx, nil, []store.OrderFillUpdate{up})
	if err != nil {
		_ = e.reloadMarketBook(ctx, ord.MarketID)
	}
	return err
}

// reloadMarketBook 重新加载市场订单簿。
func (e *Engine) reloadMarketBook(ctx context.Context, marketID int) error {
	delete(e.books, marketID)
	ob := e.getBook(marketID)
	rows, err := e.store.ListOpenLimitOrders(ctx, marketID)
	if err != nil {
		return err
	}
	for i := range rows {
		r := &rows[i]
		if !r.Price.Valid || r.Price.String == "" {
			continue
		}
		p, err := ratutil.MustPositive(r.Price.String)
		if err != nil {
			continue
		}
		rem, err := ratutil.Parse(r.RemainingQuantity)
		if err != nil || rem.Sign() == 0 {
			continue
		}
		ob.RestoreLimit(r.Side, &book.Resting{
			OrderID:   r.ID,
			UserID:    r.UserID,
			CreatedAt: r.CreatedAt.UnixMilli(),
			Price:     p,
			RemQty:    new(big.Rat).Set(rem),
		})
	}
	e.publishDepthUnsafe(ctx, marketID)
	return nil
}

// publishPublicTrades 发送公开成交到 Kafka（调用方须已持有 e.mu）；Side 为吃单方。
func (e *Engine) publishPublicTrades(ctx context.Context, taker *store.SpotOrderRow, trades []book.Trade) {
	if e.depth == nil || e.tradeTopic == "" || len(trades) == 0 {
		return
	}
	ts := time.Now().UnixMilli()
	for _, tr := range trades {
		msg := model.PublicTradeKafkaMsg{
			MarketID: taker.MarketID,
			Price:    ratutil.StringTrim(tr.Price),
			Quantity: ratutil.StringTrim(tr.Quantity),
			Side:     taker.Side,
			TsMs:     ts,
		}
		raw, err := json.Marshal(msg)
		if err != nil {
			logx.Errorf("public trade marshal market_id=%d: %v", taker.MarketID, err)
			continue
		}
		part := int32(taker.MarketID % e.depthParts)
		if err := e.depth.Publish(ctx, e.tradeTopic, part, strconv.Itoa(taker.MarketID), raw); err != nil {
			logx.Errorf("public trade publish market_id=%d: %v", taker.MarketID, err)
		}
	}
}

// publishDepthUnsafe 发送盘口快照到 Kafka（调用方须已持有 e.mu）。
func (e *Engine) publishDepthUnsafe(ctx context.Context, marketID int) {
	if e.depth == nil || e.depthTopic == "" {
		e.depthDisabledOnce.Do(func() {
			logx.Errorf("depth kafka disabled or DepthTopic empty: snapshots won't reach market-ws; check Kafka brokers and producer init")
		})
		return
	}
	ob := e.getBook(marketID)
	bLevels, aLevels := ob.SnapshotTop(50)
	e.seq[marketID]++
	seq := e.seq[marketID]
	// 构建市场深度消息（空盘口也要非 nil slice，JSON 才是 [] 而不是 null）
	msg := model.MarketDepthKafkaMsg{
		MarketID: marketID,
		Seq:      seq,
		TsMs:     time.Now().UnixMilli(),
		Bids:     make([]model.DepthPriceLevel, 0),
		Asks:     make([]model.DepthPriceLevel, 0),
	}
	for _, lv := range bLevels {
		msg.Bids = append(msg.Bids, model.DepthPriceLevel{Price: lv.Price, Quantity: lv.Qty})
	}
	for _, lv := range aLevels {
		msg.Asks = append(msg.Asks, model.DepthPriceLevel{Price: lv.Price, Quantity: lv.Qty})
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		logx.Errorf("depth marshal market_id=%d: %v", marketID, err)
		return
	}
	part := int32(marketID % e.depthParts)
	if err := e.depth.Publish(ctx, e.depthTopic, part, strconv.Itoa(marketID), raw); err != nil {
		logx.Errorf("depth publish market_id=%d: %v", marketID, err)
	}
}

// applyWalletSettlements 成交已落库后调用钱包 RPC；失败只打日志，避免阻塞 Kafka 消费。
func (e *Engine) applyWalletSettlements(ctx context.Context, mkt *store.MarketFees, taker *store.SpotOrderRow, trades []book.Trade, tradeRows []store.TradeRow, tradeIDs []uint64) {
	if e.wallet == nil || len(trades) == 0 {
		return
	}
	for i := range trades {
		if i >= len(tradeRows) || i >= len(tradeIDs) {
			logx.Errorf("wallet settlement mismatch i=%d rows=%d ids=%d", i, len(tradeRows), len(tradeIDs))
			return
		}
		tr := trades[i]
		row := tradeRows[i]
		// 调用钱包 RPC。
		_, err := e.wallet.ApplySpotTrade(ctx, &walletpb.ApplySpotTradeRequest{
			TradeId:      tradeIDs[i],
			MarketId:     int32(mkt.ID),
			BaseAssetId:  int32(mkt.BaseAssetID),
			QuoteAssetId: int32(mkt.QuoteAssetID),
			MakerOrderId: tr.MakerOrderID,
			TakerOrderId: tr.TakerOrderID,
			MakerUserId:  tr.MakerUserID,
			TakerUserId:  tr.TakerUserID,
			TakerSide:    taker.Side,
			Price:        row.Price,
			Quantity:     row.Quantity,
			MakerFee:     row.MakerFee,
			TakerFee:     row.TakerFee,
		})
		if err != nil {
			logx.Errorf("wallet ApplySpotTrade trade_id=%d: %v", tradeIDs[i], err)
		}
	}
}

// buildPersistence 构建持久化数据。
func (e *Engine) buildPersistence(ctx context.Context, mkt *store.MarketFees, trades []book.Trade, _ *store.SpotOrderRow) ([]store.TradeRow, []store.OrderFillUpdate, error) {
	byOrder := make(map[uint64][]book.Trade)
	for _, tr := range trades {
		byOrder[tr.TakerOrderID] = append(byOrder[tr.TakerOrderID], tr)
		byOrder[tr.MakerOrderID] = append(byOrder[tr.MakerOrderID], tr)
	}

	rowMap := make(map[uint64]*store.SpotOrderRow)
	for oid := range byOrder {
		r, err := e.store.GetOrder(ctx, oid)
		if err != nil {
			return nil, nil, err
		}
		if r == nil {
			return nil, nil, fmt.Errorf("order %d not found", oid)
		}
		rowMap[oid] = r
	}

	makerRate, _ := ratutil.Parse(mkt.MakerFeeRate)
	takerRate, _ := ratutil.Parse(mkt.TakerFeeRate)
	if makerRate == nil {
		makerRate = big.NewRat(0, 1)
	}
	if takerRate == nil {
		takerRate = big.NewRat(0, 1)
	}

	tradeRows := make([]store.TradeRow, 0, len(trades))
	for _, tr := range trades {
		notional := new(big.Rat).Mul(tr.Price, tr.Quantity)
		mkFee := new(big.Rat).Mul(makerRate, notional)
		tkFee := new(big.Rat).Mul(takerRate, notional)
		feeTotal := new(big.Rat).Add(mkFee, tkFee)
		tradeRows = append(tradeRows, store.TradeRow{
			MarketID:     mkt.ID,
			MakerOrderID: tr.MakerOrderID,
			TakerOrderID: tr.TakerOrderID,
			Price:        ratutil.StringTrim(tr.Price),
			Quantity:     ratutil.StringTrim(tr.Quantity),
			FeeAssetID:   sql.NullInt64{Int64: int64(mkt.QuoteAssetID), Valid: true},
			FeeAmount:    ratutil.StringTrim(feeTotal),
			MakerFee:     ratutil.StringTrim(mkFee),
			TakerFee:     ratutil.StringTrim(tkFee),
		})
	}

	var updates []store.OrderFillUpdate
	for oid, legs := range byOrder {
		r := rowMap[oid]
		u, err := orderUpdateAfterTrades(r, legs, oid)
		if err != nil {
			return nil, nil, err
		}
		updates = append(updates, u)
	}

	return tradeRows, updates, nil
}

// orderUpdateAfterTrades 订单成交后更新。
func orderUpdateAfterTrades(r *store.SpotOrderRow, legs []book.Trade, selfID uint64) (store.OrderFillUpdate, error) {
	var sumQty big.Rat
	for _, tr := range legs {
		var q *big.Rat
		if tr.TakerOrderID == selfID {
			q = tr.Quantity
		} else if tr.MakerOrderID == selfID {
			q = tr.Quantity
		} else {
			continue
		}
		sumQty.Add(&sumQty, q)
	}

	oldFilled, err := ratutil.Parse(r.FilledQuantity)
	if err != nil {
		return store.OrderFillUpdate{}, err
	}
	oldRem, err := ratutil.Parse(r.RemainingQuantity)
	if err != nil {
		return store.OrderFillUpdate{}, err
	}

	newFilled := new(big.Rat).Add(oldFilled, &sumQty)
	newRem := new(big.Rat).Sub(oldRem, &sumQty)
	if newRem.Sign() < 0 {
		return store.OrderFillUpdate{}, fmt.Errorf("order %d remaining would go negative", selfID)
	}

	if oldFilled.Sign() > 0 && !r.AvgFillPrice.Valid {
		return store.OrderFillUpdate{}, fmt.Errorf("order %d has filled>0 but avg_fill_price is null", selfID)
	}

	var avg *big.Rat
	if r.AvgFillPrice.Valid {
		avg, err = ratutil.Parse(r.AvgFillPrice.String)
		if err != nil {
			return store.OrderFillUpdate{}, err
		}
	} else {
		avg = big.NewRat(0, 1)
	}

	fb := new(big.Rat).Set(oldFilled)
	for _, tr := range legs {
		if tr.TakerOrderID != selfID && tr.MakerOrderID != selfID {
			continue
		}
		q := tr.Quantity
		p := tr.Price
		avg = store.AvgFill(fb, avg, q, p)
		fb.Add(fb, q)
	}

	st := enum.SOS_PartiallyFilled.String()
	if newRem.Sign() == 0 {
		st = enum.SOS_Filled.String()
	}

	var avgNS sql.NullString
	if newFilled.Sign() > 0 {
		avgNS = sql.NullString{String: ratutil.StringTrim(avg), Valid: true}
	}

	return store.OrderFillUpdate{
		OrderID:           selfID,
		FilledQuantity:    ratutil.StringTrim(newFilled),
		RemainingQuantity: ratutil.StringTrim(newRem),
		Status:            st,
		AvgFillPrice:      avgNS,
	}, nil
}
