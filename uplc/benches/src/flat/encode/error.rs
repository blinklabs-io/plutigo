use std::convert::Infallible;

use thiserror::Error;

#[derive(Error, Debug)]
pub enum FlatEncodeError {
    #[error("Overflow detected, cannot fit {byte} in {num_bits} bits.")]
    Overflow { byte: u8, num_bits: usize },
    #[error("Buffer is not byte aligned")]
    BufferNotByteAligned,
    #[error("Cannot encode BLS12-381 constants")]
    BlsElementNotSupported,
    #[error(transparent)]
    EncodeCbor(#[from] minicbor::encode::Error<Infallible>),
}
