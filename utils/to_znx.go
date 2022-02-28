package utils

import "math/big"

// ToMgb number of MGB to Wei
func ToMgb(mgb uint64) *big.Int {
	return new(big.Int).Mul(new(big.Int).SetUint64(mgb), big.NewInt(1e18))
}
