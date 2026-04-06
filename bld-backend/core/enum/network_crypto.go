package enum

// CryptoType 链签名体系，与 entity.Network.CryptoType 对应。
type CryptoType uint

const (
	EVM     CryptoType = iota // 以太坊系
	Bitcoin                   // 比特币
	Solana                    // Solana
)

func (c CryptoType) String() string {
	switch c {
	case EVM:
		return "EVM"
	case Bitcoin:
		return "BITCOIN"
	case Solana:
		return "SOLANA"
	default:
		return ""
	}
}

func (c CryptoType) Desc() string {
	switch c {
	case EVM:
		return "以太坊系"
	case Bitcoin:
		return "比特币"
	case Solana:
		return "Solana"
	default:
		return "未知"
	}
}
