package enum

// AssetActiveState 对应 assets.is_active（DB: 0/1）。
type AssetActiveState uint

const (
	AAS_Inactive AssetActiveState = iota // 停用
	AAS_Active                           // 启用
)

func (s AssetActiveState) String() string {
	switch s {
	case AAS_Inactive:
		return "0"
	case AAS_Active:
		return "1"
	default:
		return ""
	}
}

func (s AssetActiveState) Desc() string {
	switch s {
	case AAS_Inactive:
		return "停用"
	case AAS_Active:
		return "启用"
	default:
		return "未知"
	}
}
