// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package syn

import "math/big"

var bigOne = big.NewInt(1)

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
	temp := new(big.Int).And(n, bigOne)

	// (n >> 1) ^ (-temp)
	result := new(big.Int).Rsh(n, 1)
	negTemp := new(big.Int).Neg(temp)
	result.Xor(result, negTemp)

	return result
}
