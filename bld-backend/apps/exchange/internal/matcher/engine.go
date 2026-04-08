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
	"bld-backend/apps/exchange/internal/wal"
	"bld-backend/core/enum"
	"bld-backend/core/model"

	"github.com/zeromicro/go-zero/core/logx"
)

// Engine 按 market_id 维护订单簿并执行撮合（单互斥锁，进程内单实例）。
type Engine struct {
	mu                sync.Mutex              // 互斥锁
	store             *store.SpotStore        // 现货存储
	books             map[int]*book.OrderBook // 订单簿
	depth             mq.DepthPublisher       // 深度发布器
	depthTopic        string                  // 深度主题
	tradeTopic        string                  // 交易主题
	depthParts        int                     // 深度分区
	seq               map[int]int64           // 序列号
	wallet            walletpb.WalletClient   // 钱包客户端
	wal               *wal.Writer             // 订单簿持久化写入器
	orders            map[uint64]*orderState  // 订单状态
	depthDisabledOnce sync.Once               // 深度禁用一次
}

// orderState 订单状态。
type orderState struct {
	OrderID   uint64   // 订单 ID
	UserID    uint64   // 用户 ID
	MarketID  int      // 市场 ID
	Side      string   // 方向
	OrderType string   // 订单类型
	CreatedAt int64    // 创建时间
	Filled    *big.Rat // 已成交数量
	Remaining *big.Rat // 剩余数量
	AvgFill   *big.Rat // 平均成交价格
}

// New 创建引擎。
// - st：现货存储
// - depth：深度发布器
// - depthTopic：深度主题
// - tradeTopic：交易主题
// - depthParts：深度分区
// - wallet：钱包客户端
// - walWriter：订单簿持久化写入器
// 返回：
// - *Engine：引擎
// - error：错误
func New(st *store.SpotStore, depth mq.DepthPublisher, depthTopic, tradeTopic string, depthParts int, wallet walletpb.WalletClient, walWriter *wal.Writer) *Engine {
	// 如果深度分区小于等于 0，则设置为 1
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
		wal:        walWriter,
		orders:     make(map[uint64]*orderState),
	}
}

// Recover 启动时按订单 id 升序重放未完结限价单，等价于在线消费 Kafka，
// 避免仅 RestoreLimit 导致「可成交买卖同时挂簿却不撮」（下单后无法匹配）。
// 须在 Kafka consumer 启动前完成（worker 已保证）。
// 全程持有 e.mu，调用 handleOrderMessage（不再二次加锁）。
func (e *Engine) Recover(ctx context.Context) error {
	markets, err := e.store.ListActiveMarkets(ctx)
	if err != nil {
		return err
	}
	// 获取所有活跃市场的未完结限价单。
	rows, err := e.store.ListOpenLimitOrdersForActiveMarkets(ctx)
	if err != nil {
		return err
	}
	// 按 market_id 分组。
	rowsByMarket := make(map[int][]store.SpotOrderRow, len(markets))
	for i := range rows {
		r := rows[i]
		rowsByMarket[r.MarketID] = append(rowsByMarket[r.MarketID], r)
	}

	for i := range markets {
		market := markets[i]
		mid := market.ID
		marketRows := rowsByMarket[mid]
		e.mu.Lock()
		e.books[mid] = book.NewOrderBook(mid)
		e.seq[mid] = 0
		replayed := 0

		// 遍历市场订单，恢复订单簿。
		for i := range marketRows {
			r := &marketRows[i]
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
				OrderID:        r.ID,
				UserID:         r.UserID,
				MarketID:       mid,
				CreatedAtMs:    r.CreatedAt.UnixMilli(),
				Side:           r.Side,
				OrderType:      r.OrderType,
				Price:          &r.Price.String,
				Quantity:       r.Quantity,
				FilledQuantity: r.FilledQuantity,
				RemainingQty:   r.RemainingQuantity,
				AvgFillPrice: func() *string {
					if r.AvgFillPrice.Valid {
						v := r.AvgFillPrice.String
						return &v
					}
					return nil
				}(),
				BaseAssetID:  market.BaseAssetID,
				QuoteAssetID: market.QuoteAssetID,
				MakerFeeRate: market.MakerFeeRate,
				TakerFeeRate: market.TakerFeeRate,
				// Keep DB truth when rebuilding from DB into in-memory book/WAL.
				Status: r.Status,
			}
			// 撮合订单。
			if err := e.handleOrderMessage(ctx, msg); err != nil {
				e.mu.Unlock()
				return fmt.Errorf("recover replay order_id=%d market_id=%d: %w", r.ID, mid, err)
			}
			replayed++
		}
		// 发送盘口快照到 Kafka。
		e.publishDepthUnsafe(ctx, mid)
		e.mu.Unlock()
		logx.Infof("exchange recover market_id=%d replayed_open_limits=%d (from %d rows)", mid, replayed, len(marketRows))
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

// 消费一笔新订单（以 DB 状态为准）。须独占 e.mu；Recover 阶段由 Recover 持锁调用 handleOrderMessage。
func (e *Engine) HandleKafkaMessage(ctx context.Context, msg *model.SpotOrderKafkaMsg) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.handleOrderMessage(ctx, msg)
}

// HandleWalReplayMessage 回放 WAL 事件时，以 DB 当前订单状态为准，避免重放已完结订单。
func (e *Engine) HandleWalReplayMessage(ctx context.Context, msg *model.SpotOrderKafkaMsg) error {
	if msg == nil || msg.OrderID == 0 {
		return nil
	}
	row, err := e.store.GetOrder(ctx, msg.OrderID)
	if err != nil {
		return err
	}
	if row == nil {
		logx.Infof("exchange skip wal replay order_id=%d: order not found in DB", msg.OrderID)
		return nil
	}
	if row.Status != enum.SOS_Pending.String() && row.Status != enum.SOS_PartiallyFilled.String() {
		logx.Infof("exchange skip wal replay order_id=%d: db status=%q", msg.OrderID, row.Status)
		return nil
	}
	rem, err := ratutil.Parse(row.RemainingQuantity)
	if err != nil {
		return err
	}
	if rem.Sign() == 0 {
		logx.Infof("exchange skip wal replay order_id=%d: db remaining is 0", msg.OrderID)
		return nil
	}

	// 标准化消息。
	norm := *msg
	norm.UserID = row.UserID
	norm.MarketID = row.MarketID
	norm.CreatedAtMs = row.CreatedAt.UnixMilli()
	norm.Side = row.Side
	norm.OrderType = row.OrderType
	if row.Price.Valid && row.Price.String != "" {
		px := row.Price.String
		norm.Price = &px
	} else {
		norm.Price = nil
	}
	norm.FilledQuantity = row.FilledQuantity
	norm.RemainingQty = row.RemainingQuantity
	if row.AvgFillPrice.Valid && row.AvgFillPrice.String != "" {
		avg := row.AvgFillPrice.String
		norm.AvgFillPrice = &avg
	} else {
		norm.AvgFillPrice = nil
	}
	norm.Status = row.Status
	return e.HandleKafkaMessage(ctx, &norm)
}

// 单笔撮合（调用方须已持有 e.mu）。
func (e *Engine) handleOrderMessage(ctx context.Context, msg *model.SpotOrderKafkaMsg) error {
	if msg.OrderID == 0 || msg.MarketID <= 0 {
		return nil
	}
	// 如果订单状态为取消。
	if msg.Status == enum.SOS_Canceled.String() {
		canceled, err := e.store.CancelLimitOrder(ctx, msg.OrderID)
		if err != nil {
			return fmt.Errorf("cancel limit order in db: %w", err)
		}
		ob := e.getBook(msg.MarketID)
		removed := ob.RemoveRestingOrder(msg.OrderID)
		// 如果订单被取消且钱包客户端不为空。
		if canceled && e.wallet != nil {
			mkt, err := e.store.GetMarket(ctx, msg.MarketID)
			if err != nil {
				return err
			}
			if mkt != nil {
				var assetID int32
				switch msg.Side {
				case enum.Sell.String():
					assetID = int32(mkt.BaseAssetID)
				case enum.Buy.String():
					assetID = int32(mkt.QuoteAssetID)
				}
				if assetID > 0 {
					if _, err := e.wallet.UnfreezeForOrder(ctx, &walletpb.UnfreezeForOrderRequest{
						UserId:      msg.UserID,
						AssetId:     assetID,
						OrderId:     msg.OrderID,
						TradingType: enum.Spot.String(),
					}); err != nil {
						logx.Errorf("wallet UnfreezeForOrder on cancel order_id=%d asset_id=%d: %v", msg.OrderID, assetID, err)
					}
				}
			}
		}
		if e.wal != nil && (canceled || removed) {
			txID := e.wal.NextTxID()
			cancelRaw, err := json.Marshal(map[string]any{
				"order_id": msg.OrderID,
				"status":   msg.Status,
			})
			if err != nil {
				return err
			}
			entries := []wal.BatchEntry{{Type: wal.RecordCancelOrder, TsMs: time.Now().UnixMilli(), Payload: cancelRaw}}
			if removed {
				removeRaw, _ := json.Marshal(map[string]any{"order_id": msg.OrderID})
				entries = append(entries, wal.BatchEntry{Type: wal.RecordRemoveOrder, TsMs: time.Now().UnixMilli(), Payload: removeRaw})
			}
			if _, err := e.wal.AppendBatch(txID, entries); err != nil {
				return fmt.Errorf("wal append cancel/remove: %w", err)
			}
		}
		if removed {
			e.publishDepthUnsafe(ctx, msg.MarketID)
		}
		delete(e.orders, msg.OrderID)
		return nil
	}
	// 如果订单状态不为待成交或部分成交。
	if msg.Status != enum.SOS_Pending.String() && msg.Status != enum.SOS_PartiallyFilled.String() {
		logx.Infof("exchange skip order_id=%d: kafka status=%q (need %s/%s)", msg.OrderID, msg.Status, enum.SOS_Pending.String(), enum.SOS_PartiallyFilled.String())
		return nil
	}

	// 将 Kafka 消息转换为订单状态。
	ord, err := e.materializeOrderState(msg)
	if err != nil {
		return err
	}
	// 如果订单类型为市价单且方向为买且成交额输入模式为成交额且最大成交额不为空且最大成交额不为空。
	isTurnoverMarketBuy := ord.OrderType == enum.Market.String() &&
		ord.Side == enum.Buy.String() &&
		msg.AmountInputMode == enum.Turnover.String() &&
		msg.MaxQuoteAmount != nil &&
		*msg.MaxQuoteAmount != ""
	// 解析剩余数量。
	rem, err := ratutil.Parse(ord.RemainingQuantity)
	if err != nil {
		return err
	}
	// 如果剩余数量为 0。
	if rem.Sign() == 0 && !isTurnoverMarketBuy {
		logx.Infof("exchange skip order_id=%d: remaining_quantity is 0", ord.ID)
		return nil
	}
	// 幂等保护：如果此前已经在本进程内完成撮合并把该单剩余量更新为 0，
	// 则即使 Kafka/WAL 里再次出现旧的 PENDING 消息，也不应把它重新挂簿。
	if st := e.orders[ord.ID]; st != nil && st.Remaining != nil && st.Remaining.Sign() == 0 {
		logx.Infof("exchange skip order_id=%d: already filled in memory (idempotent)", ord.ID)
		return nil
	}
	mkt := &store.MarketFees{
		ID:           msg.MarketID,
		BaseAssetID:  msg.BaseAssetID,
		QuoteAssetID: msg.QuoteAssetID,
		MakerFeeRate: msg.MakerFeeRate,
		TakerFeeRate: msg.TakerFeeRate,
	}

	ob := e.getBook(ord.MarketID)

	// Kafka 重复投递或 Recover 后 offset 仍指向已重放订单：簿上已有该单则跳过，避免二次撮合。
	if ob.HasRestingOrder(ord.ID) {
		logx.Infof("exchange skip order_id=%d: already resting on book (idempotent)", ord.ID)
		return nil
	}
	var (
		txID     uint64
		addEntry wal.BatchEntry
		hasWAL   bool
	)
	if e.wal != nil {
		hasWAL = true
		txID = e.wal.NextTxID()
		raw, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		addEntry = wal.BatchEntry{Type: wal.RecordAddOrder, TsMs: time.Now().UnixMilli(), Payload: raw}
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
			// 市价按成交额为 IOC 语义：无论是否吃满预算，都不挂簿。
			if isTurnoverMarketBuy {
				budget, err := ratutil.Parse(*msg.MaxQuoteAmount)
				if err != nil || budget.Sign() <= 0 {
					return fmt.Errorf("invalid max_quote_amount %q", *msg.MaxQuoteAmount)
				}
				var remBudget *big.Rat
				trades, remBudget = ob.MatchMarketBuyByQuote(budget, ord.ID, ord.UserID)
				// 市价按成交额为 IOC 语义：无论是否吃满预算，都不挂簿。
				rem = big.NewRat(0, 1)
				_ = remBudget
			} else {
				// 匹配市价买单。
				trades, rem = ob.MatchMarketBuy(rem, ord.ID, ord.UserID)
			}
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
			if hasWAL {
				if _, err := e.wal.AppendBatch(txID, []wal.BatchEntry{addEntry}); err != nil {
					return fmt.Errorf("wal append market reject intent: %w", err)
				}
			}
			if err := e.persistMarketReject(ctx, ord); err != nil {
				return err
			}
			e.releaseMarketResidualFreeze(ctx, mkt, ord)
			return nil
		}
		if hasWAL {
			if _, err := e.wal.AppendBatch(txID, []wal.BatchEntry{addEntry}); err != nil {
				return fmt.Errorf("wal append add_order intent: %w", err)
			}
		}
		// 限价无成交仅挂簿：DB 未变，内存簿已更新
		e.publishDepthUnsafe(ctx, ord.MarketID)
		if ord.OrderType == enum.Limit.String() && rem.Sign() > 0 {
			logx.Infof("exchange resting order_id=%d market_id=%d side=%s rem=%s", ord.ID, ord.MarketID, ord.Side, ratutil.StringTrim(rem))
		}
		return nil
	}

	// 构建持久化数据
	var tradeRows []store.TradeRow
	var updates []store.OrderFillUpdate
	if isTurnoverMarketBuy {
		tradeRows, updates, err = e.buildPersistenceTurnoverBuy(mkt, trades, ord)
	} else {
		tradeRows, updates, err = e.buildPersistence(mkt, trades, ord)
	}
	if err != nil {
		return err
	}
	if ord.OrderType == enum.Market.String() {
		e.finalizeMarketTakerUpdate(ord, updates, msg.AmountInputMode)
	}

	// 持久化成交和订单状态到 WAL
	if hasWAL {
		entries := make([]wal.BatchEntry, 0, 1+len(trades)+len(updates)*2)
		entries = append(entries, addEntry)
		for _, tr := range trades {
			raw, err := json.Marshal(map[string]any{
				"maker_order_id": tr.MakerOrderID,
				"taker_order_id": tr.TakerOrderID,
				"price":          ratutil.StringTrim(tr.Price),
				"quantity":       ratutil.StringTrim(tr.Quantity),
			})
			if err != nil {
				return err
			}
			entries = append(entries, wal.BatchEntry{Type: wal.RecordTrade, TsMs: time.Now().UnixMilli(), Payload: raw})
		}
		for _, up := range updates {
			r, parseErr := ratutil.Parse(up.RemainingQuantity)
			filled := parseErr == nil && r.Sign() == 0
			isTaker := up.OrderID == ord.ID
			if filled && !isTaker {
				rr, _ := json.Marshal(map[string]any{"order_id": up.OrderID})
				entries = append(entries, wal.BatchEntry{Type: wal.RecordRemoveOrder, TsMs: time.Now().UnixMilli(), Payload: rr})
				continue
			}
			raw, err := json.Marshal(map[string]any{
				"order_id":             up.OrderID,
				"filled_quantity":      up.FilledQuantity,
				"remaining_quantity":   up.RemainingQuantity,
				"status":               up.Status,
				"avg_fill_price_valid": up.AvgFillPrice.Valid,
				"avg_fill_price":       up.AvgFillPrice.String,
			})
			if err != nil {
				return err
			}
			entries = append(entries, wal.BatchEntry{Type: wal.RecordUpdateOrder, TsMs: time.Now().UnixMilli(), Payload: raw})
		}
		if _, err := e.wal.AppendBatch(txID, entries); err != nil {
			return fmt.Errorf("wal append intent batch: %w", err)
		}
	}

	// 成交落库。
	tradeIDs, err := e.store.RunMatchTx(ctx, tradeRows, updates)
	if err != nil {
		// 如果执行匹配交易失败，则重新加载市场订单簿。
		if rerr := e.reloadMarketBook(ctx, ord.MarketID); rerr != nil {
			logx.Errorf("reload market %d after tx err: %v", ord.MarketID, rerr)
		}
		return err
	}

	// 更新钱包余额。
	e.applyWalletSettlements(ctx, mkt, ord, trades, tradeRows, tradeIDs)
	// 市价单不会入簿，撮合完成后释放该订单剩余冻结（全部未成交或部分成交剩余）。
	if ord.OrderType == enum.Market.String() {
		e.releaseMarketResidualFreeze(ctx, mkt, ord)
	}

	// 发送公开成交到 Kafka。
	e.publishPublicTrades(ctx, ord, trades)

	// 发送盘口快照到 Kafka
	e.publishDepthUnsafe(ctx, ord.MarketID)

	// 记录成交信息。
	logx.Infof("exchange matched order_id=%d trades=%d", ord.ID, len(trades))
	return nil
}

// releaseMarketResidualFreeze releases remaining freeze for MARKET orders.
// Wallet side is idempotent by order+asset+trading_type.
func (e *Engine) releaseMarketResidualFreeze(ctx context.Context, mkt *store.MarketFees, ord *store.SpotOrderRow) {
	if e.wallet == nil || ord == nil || mkt == nil {
		return
	}
	var assetID int32
	switch ord.Side {
	case enum.Sell.String():
		assetID = int32(mkt.BaseAssetID)
	case enum.Buy.String():
		assetID = int32(mkt.QuoteAssetID)
	default:
		return
	}
	if _, err := e.wallet.UnfreezeForOrder(ctx, &walletpb.UnfreezeForOrderRequest{
		UserId:      ord.UserID,
		AssetId:     assetID,
		OrderId:     ord.ID,
		TradingType: enum.Spot.String(),
	}); err != nil {
		logx.Errorf("wallet UnfreezeForOrder order_id=%d asset_id=%d: %v", ord.ID, assetID, err)
	}
}

// 持久化市场拒绝订单。
func (e *Engine) persistMarketReject(ctx context.Context, ord *store.SpotOrderRow) error {
	var avg sql.NullString
	if ord.AvgFillPrice.Valid {
		avg = ord.AvgFillPrice
	}
	if st := e.orders[ord.ID]; st != nil {
		st.Remaining = big.NewRat(0, 1)
	}
	up := store.OrderFillUpdate{
		OrderID:           ord.ID,
		FilledQuoteDelta:  "0",
		FilledQuantity:    ord.FilledQuantity,
		RemainingQuantity: "0",
		Status:            enum.SOS_Canceled.String(),
		AvgFillPrice:      avg,
	}
	_, err := e.store.RunMatchTx(ctx, nil, []store.OrderFillUpdate{up})
	if err != nil {
		_ = e.reloadMarketBook(ctx, ord.MarketID)
	}
	return err
}

// finalizeMarketTakerUpdate enforces IOC semantics for MARKET taker:
// if not fully filled => CANCELED; TURNOVER mode always keeps remaining_quantity as 0.
func (e *Engine) finalizeMarketTakerUpdate(ord *store.SpotOrderRow, updates []store.OrderFillUpdate, amountInputMode string) {
	for i := range updates {
		if updates[i].OrderID != ord.ID {
			continue
		}
		rem, err := ratutil.Parse(updates[i].RemainingQuantity)
		if err != nil {
			continue
		}
		if rem.Sign() > 0 {
			updates[i].Status = enum.SOS_Canceled.String()
			updates[i].RemainingQuantity = "0"
			if st := e.orders[ord.ID]; st != nil {
				st.Remaining = big.NewRat(0, 1)
			}
			continue
		}
		if amountInputMode == enum.Turnover.String() {
			updates[i].RemainingQuantity = "0"
			if st := e.orders[ord.ID]; st != nil {
				st.Remaining = big.NewRat(0, 1)
			}
		}
	}
}

// 重新加载市场订单簿。
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
		e.orders[r.ID] = &orderState{
			OrderID:   r.ID,
			UserID:    r.UserID,
			MarketID:  r.MarketID,
			Side:      r.Side,
			OrderType: r.OrderType,
			CreatedAt: r.CreatedAt.UnixMilli(),
			Filled:    mustRat(r.FilledQuantity),
			Remaining: new(big.Rat).Set(rem),
			AvgFill:   mustRatNull(r.AvgFillPrice),
		}
	}
	// 发送盘口快照到 Kafka。
	e.publishDepthUnsafe(ctx, marketID)
	return nil
}

// 发送公开成交到 Kafka（调用方须已持有 e.mu）；Side 为吃单方。
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

// 发送盘口快照到 Kafka（调用方须已持有 e.mu）。
func (e *Engine) publishDepthUnsafe(ctx context.Context, marketID int) {
	if e.depth == nil || e.depthTopic == "" {
		e.depthDisabledOnce.Do(func() {
			logx.Errorf("depth kafka disabled or DepthTopic empty: snapshots won't reach market-ws; check Kafka brokers and producer init")
		})
		return
	}
	ob := e.getBook(marketID)
	// 获取市场深度。
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
	// 构建买盘消息。
	for _, lv := range bLevels {
		msg.Bids = append(msg.Bids, model.DepthPriceLevel{Price: lv.Price, Quantity: lv.Qty})
	}
	// 构建卖盘消息。
	for _, lv := range aLevels {
		msg.Asks = append(msg.Asks, model.DepthPriceLevel{Price: lv.Price, Quantity: lv.Qty})
	}
	// 序列化市场深度消息。
	raw, err := json.Marshal(msg)
	if err != nil {
		logx.Errorf("depth marshal market_id=%d: %v", marketID, err)
		return
	}
	// 发送市场深度消息到 Kafka。
	part := int32(marketID % e.depthParts)
	if err := e.depth.Publish(ctx, e.depthTopic, part, strconv.Itoa(marketID), raw); err != nil {
		logx.Errorf("depth publish market_id=%d: %v", marketID, err)
	}
}

// 成交已落库后调用钱包 RPC；失败只打日志，避免阻塞 Kafka 消费。
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

// 构建持久化数据。
func (e *Engine) buildPersistence(mkt *store.MarketFees, trades []book.Trade, _ *store.SpotOrderRow) ([]store.TradeRow, []store.OrderFillUpdate, error) {
	byOrder := make(map[uint64][]book.Trade)
	for _, tr := range trades {
		byOrder[tr.TakerOrderID] = append(byOrder[tr.TakerOrderID], tr)
		byOrder[tr.MakerOrderID] = append(byOrder[tr.MakerOrderID], tr)
	}

	makerRate, _ := ratutil.Parse(mkt.MakerFeeRate)
	takerRate, _ := ratutil.Parse(mkt.TakerFeeRate)
	if makerRate == nil {
		makerRate = big.NewRat(0, 1)
	}
	if takerRate == nil {
		takerRate = big.NewRat(0, 1)
	}

	// 构建持久化数据。
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

	// 更新订单状态。
	var updates []store.OrderFillUpdate
	for oid, legs := range byOrder {
		st := e.orders[oid]
		if st == nil {
			return nil, nil, fmt.Errorf("order state %d not found", oid)
		}
		u, err := orderUpdateAfterTradesState(st, legs, oid)
		if err != nil {
			return nil, nil, err
		}
		updates = append(updates, u)
	}

	return tradeRows, updates, nil
}

// buildPersistenceTurnoverBuy handles MARKET BUY in TURNOVER mode:
// taker order is IOC-like, so taker remaining is finalized to 0 after match.
func (e *Engine) buildPersistenceTurnoverBuy(mkt *store.MarketFees, trades []book.Trade, taker *store.SpotOrderRow) ([]store.TradeRow, []store.OrderFillUpdate, error) {
	tradeRows, _, err := e.buildPersistence(mkt, trades, taker)
	if err != nil {
		return nil, nil, err
	}
	byOrder := make(map[uint64][]book.Trade)
	for _, tr := range trades {
		byOrder[tr.MakerOrderID] = append(byOrder[tr.MakerOrderID], tr)
	}
	updates := make([]store.OrderFillUpdate, 0, len(byOrder)+1)
	for oid, legs := range byOrder {
		st := e.orders[oid]
		if st == nil {
			return nil, nil, fmt.Errorf("order state %d not found", oid)
		}
		u, err := orderUpdateAfterTradesState(st, legs, oid)
		if err != nil {
			return nil, nil, err
		}
		updates = append(updates, u)
	}
	takerState := e.orders[taker.ID]
	if takerState == nil {
		return nil, nil, fmt.Errorf("order state %d not found", taker.ID)
	}
	oldFilled := new(big.Rat).Set(takerState.Filled)
	newFilled := new(big.Rat).Set(oldFilled)
	avg := new(big.Rat).Set(takerState.AvgFill)
	fb := new(big.Rat).Set(oldFilled)
	var sumQuote big.Rat
	for _, tr := range trades {
		if tr.TakerOrderID != taker.ID {
			continue
		}
		newFilled.Add(newFilled, tr.Quantity)
		avg = store.AvgFill(fb, avg, tr.Quantity, tr.Price)
		fb.Add(fb, tr.Quantity)
		sumQuote.Add(&sumQuote, new(big.Rat).Mul(tr.Price, tr.Quantity))
	}
	takerState.Filled = newFilled
	takerState.Remaining = big.NewRat(0, 1)
	takerState.AvgFill = avg
	var avgNS sql.NullString
	if newFilled.Sign() > 0 {
		avgNS = sql.NullString{String: ratutil.StringTrim(avg), Valid: true}
	}
	updates = append(updates, store.OrderFillUpdate{
		OrderID:           taker.ID,
		FilledQuoteDelta:  ratutil.StringTrim(&sumQuote),
		FilledQuantity:    ratutil.StringTrim(newFilled),
		RemainingQuantity: "0",
		Status:            enum.SOS_Filled.String(),
		AvgFillPrice:      avgNS,
	})
	return tradeRows, updates, nil
}

// 订单成交后更新。
func orderUpdateAfterTradesState(state *orderState, legs []book.Trade, selfID uint64) (store.OrderFillUpdate, error) {
	var sumQty big.Rat
	var sumQuote big.Rat
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
		sumQuote.Add(&sumQuote, new(big.Rat).Mul(tr.Price, q))
	}

	oldFilled := new(big.Rat).Set(state.Filled)
	oldRem := new(big.Rat).Set(state.Remaining)

	newFilled := new(big.Rat).Add(oldFilled, &sumQty)
	newRem := new(big.Rat).Sub(oldRem, &sumQty)
	if newRem.Sign() < 0 {
		return store.OrderFillUpdate{}, fmt.Errorf("order %d remaining would go negative", selfID)
	}

	avg := new(big.Rat).Set(state.AvgFill)

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

	status := enum.SOS_PartiallyFilled.String()
	if newRem.Sign() == 0 {
		status = enum.SOS_Filled.String()
	}

	var avgNS sql.NullString
	if newFilled.Sign() > 0 {
		avgNS = sql.NullString{String: ratutil.StringTrim(avg), Valid: true}
	}
	state.Filled = newFilled
	state.Remaining = newRem
	state.AvgFill = avg

	return store.OrderFillUpdate{
		OrderID:           selfID,
		FilledQuoteDelta:  ratutil.StringTrim(&sumQuote),
		FilledQuantity:    ratutil.StringTrim(newFilled),
		RemainingQuantity: ratutil.StringTrim(newRem),
		Status:            status,
		AvgFillPrice:      avgNS,
	}, nil
}

// 反序列化订单状态。
func (e *Engine) materializeOrderState(msg *model.SpotOrderKafkaMsg) (*store.SpotOrderRow, error) {
	// 解析价格。
	var px sql.NullString
	if msg.Price != nil && *msg.Price != "" {
		px = sql.NullString{String: *msg.Price, Valid: true}
	}
	// 解析创建时间。
	createdAtMs := msg.CreatedAtMs
	if createdAtMs <= 0 {
		createdAtMs = time.Now().UnixMilli()
	}
	ord := &store.SpotOrderRow{
		ID:                msg.OrderID,
		UserID:            msg.UserID,
		MarketID:          msg.MarketID,
		CreatedAt:         time.UnixMilli(createdAtMs),
		Side:              msg.Side,
		OrderType:         msg.OrderType,
		Price:             px,
		Quantity:          msg.Quantity,
		FilledQuantity:    msg.FilledQuantity,
		RemainingQuantity: msg.RemainingQty,
		Status:            msg.Status,
	}
	if msg.AvgFillPrice != nil && *msg.AvgFillPrice != "" {
		ord.AvgFillPrice = sql.NullString{String: *msg.AvgFillPrice, Valid: true}
	}
	if e.orders[msg.OrderID] == nil {
		e.orders[msg.OrderID] = &orderState{
			OrderID:   msg.OrderID,
			UserID:    msg.UserID,
			MarketID:  msg.MarketID,
			Side:      msg.Side,
			OrderType: msg.OrderType,
			CreatedAt: createdAtMs,
			Filled:    mustRat(msg.FilledQuantity),
			Remaining: mustRat(msg.RemainingQty),
			AvgFill:   mustRatPtr(msg.AvgFillPrice),
		}
	}
	return ord, nil
}

// 将字符串转换为 big.Rat。
func mustRat(s string) *big.Rat {
	r, err := ratutil.Parse(s)
	if err != nil || r == nil {
		return big.NewRat(0, 1)
	}
	return r
}

// 将字符串指针转换为 big.Rat。
func mustRatPtr(s *string) *big.Rat {
	if s == nil || *s == "" {
		return big.NewRat(0, 1)
	}
	return mustRat(*s)
}

// 将 sql.NullString 转换为 big.Rat。
func mustRatNull(v sql.NullString) *big.Rat {
	if !v.Valid {
		return big.NewRat(0, 1)
	}
	return mustRat(v.String)
}
