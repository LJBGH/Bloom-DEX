package enum

// SpotMarketStatus 交易对状态，与 entity.SpotMarket.Status 对应。
type SpotMarketStatus uint

const (
	SMS_Active   SpotMarketStatus = iota // 交易中
	SMS_Paused                           // 暂停
	SMS_Delisted                         // 已下架
)

func (s SpotMarketStatus) String() string {
	switch s {
	case SMS_Active:
		return "ACTIVE"
	case SMS_Paused:
		return "PAUSED"
	case SMS_Delisted:
		return "DELISTED"
	default:
		return ""
	}
}

func (s SpotMarketStatus) Desc() string {
	switch s {
	case SMS_Active:
		return "交易中"
	case SMS_Paused:
		return "暂停"
	case SMS_Delisted:
		return "已下架"
	default:
		return "未知"
	}
}
