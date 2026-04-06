package enum

// SpotFundFlowType 现货资金流水类型，与 entity.SpotFundFlow.FlowType 对应。
type SpotFundFlowType uint

const (
	PlacedFreeze   SpotFundFlowType = iota // 下单冻结
	CancelUnfreeze                         // 撤单解冻
	TradeExecuted                          // 成交划转
	Fees                                   // 手续费
	Transfer                               // 转账
)

func (t SpotFundFlowType) String() string {
	switch t {
	case PlacedFreeze:
		return "PLACED_FREEZE"
	case CancelUnfreeze:
		return "CANCEL_UNFREEZE"
	case TradeExecuted:
		return "TRADE_EXECUTED"
	case Fees:
		return "FEES"
	case Transfer:
		return "TRANSFER"
	default:
		return ""
	}
}

func (t SpotFundFlowType) Desc() string {
	switch t {
	case PlacedFreeze:
		return "下单冻结"
	case CancelUnfreeze:
		return "撤单解冻"
	case TradeExecuted:
		return "成交"
	case Fees:
		return "手续费"
	case Transfer:
		return "转账"
	default:
		return "未知"
	}
}
