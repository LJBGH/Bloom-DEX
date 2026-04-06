package amount

import (
	"errors"
	"math/big"
	"strings"
)

// DecimalToWei converts a decimal string (e.g. "1.23") to wei integer with given decimals.
func DecimalToWei(decimalStr string, decimals int) (*big.Int, error) {
	s := strings.TrimSpace(decimalStr)
	if s == "" {
		return nil, errors.New("empty amount")
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = strings.TrimPrefix(s, "-")
	}

	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}

	// Remove trailing spaces and keep only digits.
	intPart = strings.TrimSpace(intPart)
	fracPart = strings.TrimSpace(fracPart)

	if intPart == "" {
		intPart = "0"
	}
	if fracPart == "" {
		fracPart = "0"
	}

	// Pad / truncate fractional part to fit decimals.
	if len(fracPart) > decimals {
		fracPart = fracPart[:decimals]
	}
	for len(fracPart) < decimals {
		fracPart += "0"
	}

	base := new(big.Int)
	base.SetString(intPart, 10)

	weiFrac := new(big.Int)
	weiFrac.SetString(fracPart, 10)

	pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	wei := new(big.Int).Mul(base, pow)
	wei.Add(wei, weiFrac)
	if neg {
		wei.Neg(wei)
	}
	return wei, nil
}

// WeiToDecimal converts wei integer to decimal string with fixed decimals (no rounding).
func WeiToDecimal(wei *big.Int, decimals int) string {
	if wei == nil {
		return "0"
	}
	if decimals <= 0 {
		return wei.String()
	}
	neg := wei.Sign() < 0
	v := new(big.Int).Abs(wei)

	pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	intPart := new(big.Int).Div(v, pow)
	frac := new(big.Int).Mod(v, pow)

	fracStr := frac.String()
	// pad leading zeros
	if len(fracStr) < decimals {
		fracStr = strings.Repeat("0", decimals-len(fracStr)) + fracStr
	}
	out := intPart.String() + "." + fracStr
	if neg {
		out = "-" + out
	}
	// trim trailing zeros for nicer display? keep fixed decimals to avoid DB drift
	return out
}

