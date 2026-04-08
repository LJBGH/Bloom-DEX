package rat

import (
	"fmt"
	"math/big"
	"strings"
)

// Parse 解析非负十进制字符串；空串视为 0。
func Parse(s string) (*big.Rat, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewRat(0, 1), nil
	}
	r := new(big.Rat)
	if _, ok := r.SetString(s); !ok {
		return nil, fmt.Errorf("invalid decimal: %q", s)
	}
	if r.Sign() < 0 {
		return nil, fmt.Errorf("negative decimal: %q", s)
	}
	return r, nil
}

// 要求大于 0。
func MustPositive(s string) (*big.Rat, error) {
	r, err := Parse(s)
	if err != nil {
		return nil, err
	}
	if r.Sign() <= 0 {
		return nil, fmt.Errorf("must be positive: %q", s)
	}
	return r, nil
}

// StringTrim 将 Rat 转为十进制字符串（最多 18 位小数，去尾零）。
func StringTrim(r *big.Rat) string {
	if r == nil {
		return "0"
	}
	s := r.FloatString(18)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

// Min 返回 a、b 中较小者（均为非负时使用）。
func Min(a, b *big.Rat) *big.Rat {
	if a.Cmp(b) <= 0 {
		return new(big.Rat).Set(a)
	}
	return new(big.Rat).Set(b)
}
