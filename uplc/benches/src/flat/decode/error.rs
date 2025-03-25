use thiserror::Error;

#[derive(Error, Debug)]
pub enum FlatDecodeError {
    #[error("Reached end of buffer")]
    EndOfBuffer,
    #[error("Buffer is not byte aligned")]
    BufferNotByteAligned,
    #[error("Incorrect value of num_bits, must be less than 9")]
    IncorrectNumBits,
    #[error("Not enough data available, required {0} bytes")]
    NotEnoughBytes(usize),
    #[error("Not enough data available, required {0} bits")]
    NotEnoughBits(usize),
    #[error(transparent)]
    DecodeUtf8(#[from] std::str::Utf8Error),
    #[error(transparent)]
    DecodeCbor(#[from] minicbor::decode::Error),
    #[error("Decoding u32 to char {0}")]
    DecodeChar(u32),
    #[error("{0}")]
    Message(String),
    #[error("Default Function not found: {0}")]
    DefaultFunctionNotFound(u8),
    #[error("Unknown term constructor tag: {0}")]
    UnknownTermConstructor(u8),
    #[error("Unknown constant constructor tag: {0:#?}")]
    UnknownConstantConstructor(Vec<u8>),
}
