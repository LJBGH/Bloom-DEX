package enum

// TradingType 冻结/订单业务线，与 entity.AssetFreeze.TradingType 对应。
type TradingType uint

const (
	Spot     TradingType = iota // 现货
	Contract                    // 合约
)

func (t TradingType) String() string {
	switch t {
	case Spot:
		return "SPOT"
	case Contract:
		return "CONTRACT"
	default:
		return ""
	}
}

func (t TradingType) Desc() string {
	switch t {
	case Spot:
		return "现货"
	case Contract:
		return "合约"
	default:
		return "未知"
	}
}
