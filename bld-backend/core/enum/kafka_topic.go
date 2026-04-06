package enum

// Kafka 等业务 topic 名常量。
const (
	TopicOrderInput  = "trade.order.create"  //订单创建主题
	TopicOrderUpdate = "trade.order.update"  //订单更新主题
	TopicDepthDelta  = "market.depth.delta"  //盘口快照主题
	TopicMarketTrade = "market.trade.tick"  //公开成交（market-ws → 前端）
	TopicTradeFill   = "trade.fill"          //交易填充主题
	TopicTicker      = "market.ticker"       //行情主题
	TopicDepositIn   = "wallet.deposit.in"   //充值主题
	TopicWithdrawOut = "wallet.withdraw.out" //提现主题
)
