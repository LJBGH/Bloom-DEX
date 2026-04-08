package enum

// SpotSide 现货买卖方向，与 entity.SpotOrder.Side（DB: BUY/SELL）对应。
type SpotSide uint

const (
	Buy  SpotSide = iota // 买入
	Sell                 // 卖出
)

func (s SpotSide) String() string {
	switch s {
	case Buy:
		return "BUY"
	case Sell:
		return "SELL"
	default:
		return ""
	}
}

func (s SpotSide) Desc() string {
	switch s {
	case Buy:
		return "买入"
	case Sell:
		return "卖出"
	default:
		return "未知"
	}
}

// SpotOrderType 订单类型，与 entity.SpotOrder.OrderType（LIMIT/MARKET）对应。
type SpotOrderType uint

const (
	Limit  SpotOrderType = iota // 限价
	Market                      // 市价
)

func (t SpotOrderType) String() string {
	switch t {
	case Limit:
		return "LIMIT"
	case Market:
		return "MARKET"
	default:
		return ""
	}
}

func (t SpotOrderType) Desc() string {
	switch t {
	case Limit:
		return "限价"
	case Market:
		return "市价"
	default:
		return "未知"
	}
}

// SpotAmountInputMode 市价单数量维度，与 entity.SpotOrder.AmountInputMode 对应。
type SpotAmountInputMode uint

const (
	Quantity SpotAmountInputMode = iota // 按基础币数量
	Turnover                            // 按报价币成交额
)

func (m SpotAmountInputMode) String() string {
	switch m {
	case Quantity:
		return "QUANTITY"
	case Turnover:
		return "TURNOVER"
	default:
		return ""
	}
}

func (m SpotAmountInputMode) Desc() string {
	switch m {
	case Quantity:
		return "按数量"
	case Turnover:
		return "按成交额"
	default:
		return "未知"
	}
}

// SpotOrderStatus 订单状态，与 entity.SpotOrder.Status 对应。
type SpotOrderStatus uint

const (
	SOS_Pending         SpotOrderStatus = iota // 待成交
	SOS_PartiallyFilled                        // 部分成交
	SOS_Filled                                 // 全部成交
	SOS_Canceled                               // 已撤销
	SOS_Rejected                               // 已拒绝
)

func (s SpotOrderStatus) String() string {
	switch s {
	case SOS_Pending:
		return "PENDING"
	case SOS_PartiallyFilled:
		return "PARTIALLY_FILLED"
	case SOS_Filled:
		return "FILLED"
	case SOS_Canceled:
		return "CANCELED"
	case SOS_Rejected:
		return "REJECTED"
	default:
		return ""
	}
}

func (s SpotOrderStatus) Desc() string {
	switch s {
	case SOS_Pending:
		return "待成交"
	case SOS_PartiallyFilled:
		return "部分成交"
	case SOS_Filled:
		return "全部成交"
	case SOS_Canceled:
		return "已取消"
	case SOS_Rejected:
		return "已拒绝"
	default:
		return "未知"
	}
}
