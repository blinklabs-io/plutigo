// #[cfg(feature = "num-bigint")]
// use num_bigint::{BigInt, BigUint, ToBigInt};

use crate::constant::Integer;

pub trait ZigZag {
    type Zag;

    fn zigzag(self) -> Self::Zag;
    fn unzigzag(self) -> Self::Zag;
}

impl ZigZag for &Integer {
    type Zag = Integer;

    fn zigzag(self) -> Self::Zag {
        if *self >= 0 {
            // For non-negative numbers, just multiply by 2 (left shift by 1)
            self.clone() << 1
        } else {
            // For negative numbers: -(2 * n) - 1
            // First multiply by 2
            let double: Integer = self.clone() << 1;

            // Then negate and subtract 1
            -double - 1
        }
    }

    fn unzigzag(self) -> Self::Zag {
        let temp: Integer = self.clone() & 1;

        (self.clone() >> 1) ^ -(temp)
    }
}
