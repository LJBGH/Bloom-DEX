package enum

// WithdrawStatus 提现订单状态，与 entity.WithdrawOrder.Status 对应。
type WithdrawStatus uint

const (
	Sent      WithdrawStatus = iota // 已提交链上/队列
	Pending                         // 处理中
	Confirmed                       // 已确认
	Failed                          // 失败
	Canceled                        // 已取消
)

func (s WithdrawStatus) String() string {
	switch s {
	case Sent:
		return "SENT"
	case Pending:
		return "PENDING"
	case Confirmed:
		return "CONFIRMED"
	case Failed:
		return "FAILED"
	case Canceled:
		return "CANCELED"
	default:
		return ""
	}
}

func (s WithdrawStatus) Desc() string {
	switch s {
	case Sent:
		return "已发送"
	case Pending:
		return "处理中"
	case Confirmed:
		return "已确认"
	case Failed:
		return "失败"
	case Canceled:
		return "已取消"
	default:
		return "未知"
	}
}
