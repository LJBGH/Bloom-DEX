package logic

import (
	"errors"
	"math/big"
	"strings"
)

func parseRat(s string) (*big.Rat, error) {
	r := new(big.Rat)
	if _, ok := r.SetString(strings.TrimSpace(s)); !ok {
		return nil, errors.New("invalid decimal")
	}
	return r, nil
}

func ratToDecimal18(r *big.Rat) string {
	if r == nil {
		return "0"
	}
	f := new(big.Float).SetPrec(512).SetRat(r)
	out := f.Text('f', 18)
	return trimDecimal(out)
}

func trimDecimal(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func ratSubStr(a, b string) (string, error) {
	ra, err := parseRat(a)
	if err != nil {
		return "", err
	}
	rb, err := parseRat(b)
	if err != nil {
		return "", err
	}
	return ratToDecimal18(new(big.Rat).Sub(ra, rb)), nil
}

func ratMulStr(a, b string) (string, error) {
	ra, err := parseRat(a)
	if err != nil {
		return "", err
	}
	rb, err := parseRat(b)
	if err != nil {
		return "", err
	}
	return ratToDecimal18(new(big.Rat).Mul(ra, rb)), nil
}

func ratNonNegString(s string) bool {
	r, err := parseRat(s)
	if err != nil {
		return false
	}
	return r.Sign() >= 0
}

func ratIsPositive(s string) bool {
	r, err := parseRat(s)
	if err != nil {
		return false
	}
	return r.Sign() > 0
}
