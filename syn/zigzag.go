package syn

import "math/big"

// zigzag encodes a signed big.Int into an unsigned big.Int using zigzag encoding.
func zigzag(n *big.Int) *big.Int {
	result := new(big.Int)

	if n.Sign() >= 0 {
		// For non-negative: multiply by 2 (left shift by 1)
		result.Lsh(n, 1)
	} else {
		// For negative: -(2 * n) - 1
		double := new(big.Int).Lsh(n, 1)
		result.Neg(double)
		result.Sub(result, big.NewInt(1))
	}

	return result
}

// unzigzag decodes an unsigned big.Int back to a signed big.Int using zigzag decoding.
func unzigzag(n *big.Int) *big.Int {
	// temp = n & 1
	temp := new(big.Int).And(n, big.NewInt(1))

	// (n >> 1) ^ (-temp)
	result := new(big.Int).Rsh(n, 1)
	negTemp := new(big.Int).Neg(temp)
	result.Xor(result, negTemp)

	return result
}
