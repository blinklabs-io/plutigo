use std::array::TryFromSliceError;

use crate::{
    binder::Eval,
    bls::BlsError,
    constant::{Constant, Integer},
    data::PlutusData,
    term::Term,
    typ::Type,
};

use super::{value::Value, ExBudget};

#[derive(thiserror::Error, Debug)]
pub enum MachineError<'a, V>
where
    V: Eval<'a>,
{
    #[error("Explicit error term")]
    ExplicitErrorTerm,
    #[error("Non-function application")]
    NonFunctionApplication(&'a Value<'a, V>, &'a Value<'a, V>),
    #[error("Non-constant value")]
    NotAConstant(&'a Value<'a, V>),
    #[error("Open term evaluated")]
    OpenTermEvaluated(&'a Term<'a, V>),
    #[error("Out of budget")]
    OutOfExError(ExBudget),
    #[error("Unexpected builtin term argument")]
    UnexpectedBuiltinTermArgument(&'a Term<'a, V>),
    #[error("Non-polymorphic instantiation")]
    NonPolymorphicInstantiation(&'a Value<'a, V>),
    #[error("Builtin term argument expected")]
    BuiltinTermArgumentExpected(&'a Term<'a, V>),
    #[error("Non-constructor scrutinized")]
    NonConstrScrutinized(&'a Value<'a, V>),
    #[error("Non-integer index")]
    MissingCaseBranch(&'a [&'a Term<'a, V>], &'a Value<'a, V>),
    #[error(transparent)]
    Runtime(RuntimeError<'a>),
}

#[derive(thiserror::Error, Debug)]
pub enum RuntimeError<'a> {
    #[error("Byte string out of bounds")]
    ByteStringOutOfBounds(&'a [u8], &'a Integer),
    #[error("Type mismatch")]
    TypeMismatch(Type<'a>, &'a Constant<'a>),
    #[error("Expected pair")]
    ExpectedPair(&'a Constant<'a>),
    #[error("Expected list")]
    ExpectedList(&'a Constant<'a>),
    #[error("Not data")]
    NotData(&'a Constant<'a>),
    #[error("Malformed data")]
    MalFormedData(&'a PlutusData<'a>),
    #[error("Empty list")]
    EmptyList(&'a [&'a Constant<'a>]),
    #[error("Unexpected Ed25519 public key length")]
    UnexpectedEd25519PublicKeyLength(TryFromSliceError),
    #[error("Unexpected Ed25519 signature length")]
    UnexpectedEd25519SignatureLength(TryFromSliceError),
    #[error("Division by zero")]
    DivisionByZero(&'a Integer, &'a Integer),
    #[error("MkCons type mismatch")]
    MkConsTypeMismatch(&'a Constant<'a>),
    #[error("Byte string cons not a byte")]
    ByteStringConsNotAByte(&'a Integer),
    #[error(transparent)]
    Secp256k1(#[from] secp256k1::Error),
    #[error(transparent)]
    DecodeUtf8(#[from] std::str::Utf8Error),
    #[error(transparent)]
    Bls(#[from] BlsError),
    #[error("Bls Error: Hash to curve dst too big")]
    HashToCurveDstTooBig,
    #[error(
        "bytes size beyond limit when converting from integer\n{:>13} {0}\n{:>13} {1}",
        "Size",
        "Maximum"
    )]
    IntegerToByteStringSizeTooBig(&'a Integer, i64),
    #[error(
        "bytes size below limit when converting from integer\n{:>13} {0}\n{:>13} {1}",
        "Size",
        "Minimum"
    )]
    IntegerToByteStringSizeTooSmall(&'a Integer, usize),
    #[error("integerToByteString encountered negative input\n{:>13} {0}", "Input")]
    IntegerToByteStringNegativeInput(&'a Integer),
    #[error("integerToByteString encountered negative size\n{:>13} {0}", "Size")]
    IntegerToByteStringNegativeSize(&'a Integer),
}

impl<'a, V> MachineError<'a, V>
where
    V: Eval<'a>,
{
    pub fn runtime(runtime_error: RuntimeError<'a>) -> Self {
        MachineError::Runtime(runtime_error)
    }

    pub fn type_mismatch(expected: Type<'a>, constant: &'a Constant<'a>) -> Self {
        MachineError::runtime(RuntimeError::TypeMismatch(expected, constant))
    }

    pub fn mk_cons_type_mismatch(constant: &'a Constant<'a>) -> Self {
        MachineError::runtime(RuntimeError::MkConsTypeMismatch(constant))
    }

    pub fn expected_pair(constant: &'a Constant<'a>) -> Self {
        MachineError::runtime(RuntimeError::ExpectedPair(constant))
    }

    pub fn expected_list(constant: &'a Constant<'a>) -> Self {
        MachineError::runtime(RuntimeError::ExpectedList(constant))
    }

    pub fn empty_list(constant: &'a [&'a Constant<'a>]) -> Self {
        MachineError::runtime(RuntimeError::EmptyList(constant))
    }

    pub fn byte_string_out_of_bounds(byte_string: &'a [u8], index: &'a Integer) -> Self {
        MachineError::runtime(RuntimeError::ByteStringOutOfBounds(byte_string, index))
    }

    pub fn not_data(constant: &'a Constant<'a>) -> Self {
        MachineError::runtime(RuntimeError::NotData(constant))
    }

    pub fn malformed_data(plutus_data: &'a PlutusData<'a>) -> Self {
        MachineError::runtime(RuntimeError::MalFormedData(plutus_data))
    }

    pub fn unexpected_ed25519_public_key_length(length: TryFromSliceError) -> Self {
        MachineError::runtime(RuntimeError::UnexpectedEd25519PublicKeyLength(length))
    }

    pub fn unexpected_ed25519_signature_length(length: TryFromSliceError) -> Self {
        MachineError::runtime(RuntimeError::UnexpectedEd25519SignatureLength(length))
    }

    pub fn division_by_zero(numerator: &'a Integer, denominator: &'a Integer) -> Self {
        MachineError::runtime(RuntimeError::DivisionByZero(numerator, denominator))
    }

    pub fn byte_string_cons_not_a_byte(byte: &'a Integer) -> Self {
        MachineError::runtime(RuntimeError::ByteStringConsNotAByte(byte))
    }

    pub fn secp256k1(error: secp256k1::Error) -> Self {
        MachineError::runtime(RuntimeError::Secp256k1(error))
    }

    pub fn decode_utf8(error: std::str::Utf8Error) -> Self {
        MachineError::runtime(RuntimeError::DecodeUtf8(error))
    }

    pub fn bls(error: BlsError) -> Self {
        MachineError::runtime(RuntimeError::Bls(error))
    }

    pub fn hash_to_curve_dst_too_big() -> Self {
        MachineError::runtime(RuntimeError::HashToCurveDstTooBig)
    }

    pub fn integer_to_byte_string_size_too_big(integer: &'a Integer, maximum: i64) -> Self {
        MachineError::runtime(RuntimeError::IntegerToByteStringSizeTooBig(
            integer, maximum,
        ))
    }

    pub fn integer_to_byte_string_size_too_small(integer: &'a Integer, minimum: usize) -> Self {
        MachineError::runtime(RuntimeError::IntegerToByteStringSizeTooSmall(
            integer, minimum,
        ))
    }

    pub fn integer_to_byte_string_negative_input(integer: &'a Integer) -> Self {
        MachineError::runtime(RuntimeError::IntegerToByteStringNegativeInput(integer))
    }

    pub fn integer_to_byte_string_negative_size(integer: &'a Integer) -> Self {
        MachineError::runtime(RuntimeError::IntegerToByteStringNegativeSize(integer))
    }
}
