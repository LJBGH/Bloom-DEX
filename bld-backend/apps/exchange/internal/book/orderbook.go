package book

import (
	"math/big"

	"bld-backend/apps/exchange/internal/util/rat"
)

// Resting 挂单（限价剩余量）。
type Resting struct {
	OrderID   uint64   // 订单 ID
	UserID    uint64   // 用户 ID
	CreatedAt int64    // 挂单时间（UnixMilli），同价位时间优先
	Price     *big.Rat // 价格
	RemQty    *big.Rat // 剩余数量
}

// level 挂单档位。
type level struct {
	price  *big.Rat   // 价格
	orders []*Resting // 挂单列表
}

// OrderBook 单笔交易对市场内的买卖盘（价优 + 时间优先）。
type OrderBook struct {
	MarketID int      // 交易对 ID
	bids     []*level // 买盘：价格从高到低
	asks     []*level // 卖盘：价格从低到高
}

func NewOrderBook(marketID int) *OrderBook {
	return &OrderBook{MarketID: marketID}
}

// AddBid 买方挂单（价高优先，同价 FIFO）。
func (ob *OrderBook) AddBid(o *Resting) {
	// 插入买单。
	ob.bids = insertBidLevel(ob.bids, o)
}

// AddAsk 卖方挂单（价低优先，同价 FIFO）。
func (ob *OrderBook) AddAsk(o *Resting) {
	// 插入卖单。
	ob.asks = insertAskLevel(ob.asks, o)
}

// 买单插入买方挂单（价高优先，同价 FIFO）。
func insertBidLevel(levels []*level, o *Resting) []*level {
	// 遍历买单档位。
	for i, lv := range levels {
		// 比较价格，相等则插入挂单。
		c := o.Price.Cmp(lv.price)
		if c == 0 {
			lv.orders = insertByTimePriority(lv.orders, o)
			return levels
		}
		// 如果价格更高，则插入新的挂单档位。
		if c > 0 {
			// 创建新的挂单档位。
			nl := &level{price: new(big.Rat).Set(o.Price), orders: []*Resting{o}}
			return append(levels[:i], append([]*level{nl}, levels[i:]...)...)
		}
	}
	// 如果价格更低，则插入新的挂单档位。
	nl := &level{price: new(big.Rat).Set(o.Price), orders: []*Resting{o}}
	return append(levels, nl)
}

// 卖单插入卖方挂单（价低优先，同价 FIFO）。
func insertAskLevel(levels []*level, o *Resting) []*level {
	// 遍历卖单档位。
	for i, lv := range levels {
		// 比较价格，相等则插入挂单。
		c := o.Price.Cmp(lv.price)
		if c == 0 {
			lv.orders = insertByTimePriority(lv.orders, o)
			return levels
		}
		// 如果价格更低，则插入新的挂单档位。
		if c < 0 {
			// 创建新的挂单档位。
			nl := &level{price: new(big.Rat).Set(o.Price), orders: []*Resting{o}}
			return append(levels[:i], append([]*level{nl}, levels[i:]...)...)
		}
	}
	// 如果价格更高，则插入新的挂单档位。
	nl := &level{price: new(big.Rat).Set(o.Price), orders: []*Resting{o}}
	return append(levels, nl)
}

// insertByTimePriority 同价位按 CreatedAt 升序（更早优先）；相同时间按 OrderID 升序兜底。
func insertByTimePriority(orders []*Resting, o *Resting) []*Resting {
	for i, ex := range orders {
		if o.CreatedAt < ex.CreatedAt || (o.CreatedAt == ex.CreatedAt && o.OrderID < ex.OrderID) {
			return append(orders[:i], append([]*Resting{o}, orders[i:]...)...)
		}
	}
	return append(orders, o)
}

// Trade 一笔撮合结果（价为 maker 价）。
type Trade struct {
	MakerOrderID uint64   //  买单订单 ID
	TakerOrderID uint64   //  卖单订单 ID
	MakerUserID  uint64   //  买单用户 ID
	TakerUserID  uint64   //  卖单用户 ID
	Price        *big.Rat //  价格
	Quantity     *big.Rat //  数量
}

// MatchLimitBuy 限价买入（taker 为买方）：
// 只与卖盘（asks，从低到高）撮合，且仅当 卖单挂单价 <= 买方限价 时才可成交
// 返回成交列表与剩余买方 base 数量
// 输入：
// - limitPrice：买方限价
// - takerRem：买方剩余数量
// - takerOrderID：买方订单 ID
// - takerUserID：买方用户 ID
// 输出：
// - trades：成交列表
// - rem：剩余买方 base 数量
func (ob *OrderBook) MatchLimitBuy(limitPrice *big.Rat, takerRem *big.Rat, takerOrderID, takerUserID uint64) ([]Trade, *big.Rat) {

	rem := new(big.Rat).Set(takerRem)
	var trades []Trade
	// 遍历卖盘档位。
	for rem.Sign() > 0 && len(ob.asks) > 0 {
		lv := ob.asks[0]
		// askPrice > limitPrice → 卖得太贵，更优卖档已吃完，停止吃单
		if lv.price.Cmp(limitPrice) > 0 {
			break
		}
		// 消耗卖盘档位。
		remBefore := new(big.Rat).Set(rem)
		trades, rem = ob.consumeLevelAsks(lv, rem, takerOrderID, takerUserID, trades)
		// 本档全是自成交等导致一口都吃不到时，不得死循环；应挂出剩余买量。
		if rem.Cmp(remBefore) == 0 {
			break
		}
		ob.trimEmptyFront(&ob.asks)
	}
	return trades, rem
}

// MatchLimitSell 限价卖出（taker 为卖方）：
// 只与买盘（bids，从高到低）撮合，且仅当 买单挂单价 >= 卖方限价 时才可成交
// 返回成交列表与剩余卖方 base 数量
// 输入：
// - limitPrice：卖方限价
// - takerRem：卖方剩余数量
// - takerOrderID：卖方订单 ID
// - takerUserID：卖方用户 ID
// 输出：
// - trades：成交列表
// - rem：剩余卖方 base 数量
func (ob *OrderBook) MatchLimitSell(limitPrice *big.Rat, takerRem *big.Rat, takerOrderID, takerUserID uint64) ([]Trade, *big.Rat) {
	rem := new(big.Rat).Set(takerRem)
	var trades []Trade
	for rem.Sign() > 0 && len(ob.bids) > 0 {
		lv := ob.bids[0]
		// bidPrice < limitPrice → 买价太低，更优买档已吃完，停止吃单
		if lv.price.Cmp(limitPrice) < 0 {
			break
		}
		remBefore := new(big.Rat).Set(rem)
		trades, rem = ob.consumeLevelBids(lv, rem, takerOrderID, takerUserID, trades)
		// 如果剩余数量没有变化，则停止吃单。
		if rem.Cmp(remBefore) == 0 {
			break
		}
		// 删除空挂单档位。
		ob.trimEmptyFront(&ob.bids)
	}
	return trades, rem
}

// MatchMarketBuy 市价买入：按卖一价起连续吃单。
func (ob *OrderBook) MatchMarketBuy(takerRem *big.Rat, takerOrderID, takerUserID uint64) ([]Trade, *big.Rat) {
	rem := new(big.Rat).Set(takerRem)
	var trades []Trade
	for rem.Sign() > 0 && len(ob.asks) > 0 {
		lv := ob.asks[0]
		remBefore := new(big.Rat).Set(rem)
		trades, rem = ob.consumeLevelAsks(lv, rem, takerOrderID, takerUserID, trades)
		if rem.Cmp(remBefore) == 0 {
			break
		}
		ob.trimEmptyFront(&ob.asks)
	}
	return trades, rem
}

// MatchMarketSell 市价卖出：按买一价起连续吃单。
func (ob *OrderBook) MatchMarketSell(takerRem *big.Rat, takerOrderID, takerUserID uint64) ([]Trade, *big.Rat) {
	rem := new(big.Rat).Set(takerRem)
	var trades []Trade
	for rem.Sign() > 0 && len(ob.bids) > 0 {
		lv := ob.bids[0]
		remBefore := new(big.Rat).Set(rem)
		trades, rem = ob.consumeLevelBids(lv, rem, takerOrderID, takerUserID, trades)
		if rem.Cmp(remBefore) == 0 {
			break
		}
		ob.trimEmptyFront(&ob.bids)
	}
	return trades, rem
}

// consumeLevelAsks 消耗卖方挂单（价低优先，同价 FIFO）。
// 输入：
// - lv：卖盘档位
// - rem：剩余数量
// - takerOID：买方订单 ID
// - takerUID：买方用户 ID
// - trades：成交列表
// 输出：
// - trades：成交列表
// - rem：剩余数量
func (ob *OrderBook) consumeLevelAsks(lv *level, rem *big.Rat, takerOID, takerUID uint64, trades []Trade) ([]Trade, *big.Rat) {
	for rem.Sign() > 0 {
		j := -1
		for i, mk := range lv.orders {
			if mk.UserID != takerUID {
				j = i
				break
			}
		}
		if j < 0 {
			break
		}
		mk := lv.orders[j]
		q := rat.Min(rem, mk.RemQty)
		trades = append(trades, Trade{
			MakerOrderID: mk.OrderID,
			TakerOrderID: takerOID,
			MakerUserID:  mk.UserID,
			TakerUserID:  takerUID,
			Price:        new(big.Rat).Set(lv.price),
			Quantity:     new(big.Rat).Set(q),
		})
		mk.RemQty.Sub(mk.RemQty, q)
		rem.Sub(rem, q)
		if mk.RemQty.Sign() == 0 {
			lv.orders = append(lv.orders[:j], lv.orders[j+1:]...)
		}
	}
	return trades, rem
}

// consumeLevelBids 消耗买方挂单（价高优先，同价 FIFO）。
// 输入：
// - lv：买盘档位
// - rem：剩余数量
// - takerOID：买方订单 ID
// - takerUID：买方用户 ID
// - trades：成交列表
// 输出：
// - trades：成交列表
// - rem：剩余数量
func (ob *OrderBook) consumeLevelBids(lv *level, rem *big.Rat, takerOID, takerUID uint64, trades []Trade) ([]Trade, *big.Rat) {
	for rem.Sign() > 0 {
		j := -1
		for i, mk := range lv.orders {
			if mk.UserID != takerUID {
				j = i
				break
			}
		}
		if j < 0 {
			break
		}
		mk := lv.orders[j]
		q := rat.Min(rem, mk.RemQty)
		trades = append(trades, Trade{
			MakerOrderID: mk.OrderID,
			TakerOrderID: takerOID,
			MakerUserID:  mk.UserID,
			TakerUserID:  takerUID,
			Price:        new(big.Rat).Set(lv.price),
			Quantity:     new(big.Rat).Set(q),
		})
		mk.RemQty.Sub(mk.RemQty, q)
		rem.Sub(rem, q)
		if mk.RemQty.Sign() == 0 {
			lv.orders = append(lv.orders[:j], lv.orders[j+1:]...)
		}
	}
	return trades, rem
}

// trimEmptyFront 删除前端空挂单。
func (ob *OrderBook) trimEmptyFront(side *[]*level) {
	for len(*side) > 0 {
		lv := (*side)[0]
		if len(lv.orders) == 0 {
			*side = (*side)[1:]
			continue
		}
		break
	}
}

// RestoreLimit 从数据库恢复限价挂单（不做撮合）。
func (ob *OrderBook) RestoreLimit(side string, o *Resting) {
	if side == "BUY" {
		ob.AddBid(o)
		return
	}
	ob.AddAsk(o)
}

// DepthSnapshotLevel 盘口聚合一档（供 Kafka / 推送）。
type DepthSnapshotLevel struct {
	Price string // 价格
	Qty   string // 数量
}

// SnapshotTop 取买盘/卖盘各最多 limit 档（买盘价高在前，卖盘价低在前）；
// 每档数量为同价挂单 remaining 之和。
func (ob *OrderBook) SnapshotTop(limit int) (bids, asks []DepthSnapshotLevel) {
	if limit <= 0 {
		limit = 50
	}
	for _, lv := range ob.bids {
		if len(bids) >= limit {
			break
		}
		var sum big.Rat
		for _, o := range lv.orders {
			sum.Add(&sum, o.RemQty)
		}
		if sum.Sign() == 0 {
			continue
		}
		bids = append(bids, DepthSnapshotLevel{
			Price: rat.StringTrim(lv.price),
			Qty:   rat.StringTrim(&sum),
		})
	}
	for _, lv := range ob.asks {
		if len(asks) >= limit {
			break
		}
		var sum big.Rat
		for _, o := range lv.orders {
			sum.Add(&sum, o.RemQty)
		}
		if sum.Sign() == 0 {
			continue
		}
		asks = append(asks, DepthSnapshotLevel{
			Price: rat.StringTrim(lv.price),
			Qty:   rat.StringTrim(&sum),
		})
	}
	return bids, asks
}

// HasRestingOrder 是否已在簿（防 Kafka 重复投递重复挂单）。
func (ob *OrderBook) HasRestingOrder(orderID uint64) bool {
	for _, lv := range ob.bids {
		for _, r := range lv.orders {
			if r.OrderID == orderID {
				return true
			}
		}
	}
	for _, lv := range ob.asks {
		for _, r := range lv.orders {
			if r.OrderID == orderID {
				return true
			}
		}
	}
	return false
}
