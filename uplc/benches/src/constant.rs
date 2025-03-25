use bumpalo::Bump;

use crate::{binder::Eval, data::PlutusData, machine::MachineError, typ::Type};

#[derive(Debug, PartialEq)]
pub enum Constant<'a> {
    Integer(&'a Integer),
    ByteString(&'a [u8]),
    String(&'a str),
    Boolean(bool),
    Data(&'a PlutusData<'a>),
    ProtoList(&'a Type<'a>, &'a [&'a Constant<'a>]),
    ProtoPair(
        &'a Type<'a>,
        &'a Type<'a>,
        &'a Constant<'a>,
        &'a Constant<'a>,
    ),
    Unit,
    Bls12_381G1Element(&'a blst::blst_p1),
    Bls12_381G2Element(&'a blst::blst_p2),
    Bls12_381MlResult(&'a blst::blst_fp12),
}

pub type Integer = rug::Integer;

pub fn integer(arena: &Bump) -> &mut Integer {
    arena.alloc(Integer::new())
}

pub fn integer_from(arena: &Bump, i: i128) -> &mut Integer {
    arena.alloc(Integer::from(i))
}

impl<'a> Constant<'a> {
    pub fn integer(arena: &'a Bump, i: &'a Integer) -> &'a Constant<'a> {
        arena.alloc(Constant::Integer(i))
    }

    pub fn integer_from(arena: &'a Bump, i: i128) -> &'a Constant<'a> {
        arena.alloc(Constant::Integer(integer_from(arena, i)))
    }

    pub fn byte_string(arena: &'a Bump, bytes: &'a [u8]) -> &'a Constant<'a> {
        arena.alloc(Constant::ByteString(bytes))
    }

    pub fn string(arena: &'a Bump, s: &'a str) -> &'a Constant<'a> {
        arena.alloc(Constant::String(s))
    }

    pub fn bool(arena: &'a Bump, v: bool) -> &'a Constant<'a> {
        arena.alloc(Constant::Boolean(v))
    }

    pub fn data(arena: &'a Bump, d: &'a PlutusData<'a>) -> &'a Constant<'a> {
        arena.alloc(Constant::Data(d))
    }

    pub fn unit(arena: &'a Bump) -> &'a Constant<'a> {
        arena.alloc(Constant::Unit)
    }

    pub fn proto_list(
        arena: &'a Bump,
        inner: &'a Type<'a>,
        values: &'a [&'a Constant<'a>],
    ) -> &'a Constant<'a> {
        arena.alloc(Constant::ProtoList(inner, values))
    }

    pub fn proto_pair(
        arena: &'a Bump,
        first_type: &'a Type<'a>,
        second_type: &'a Type<'a>,
        first_value: &'a Constant<'a>,
        second_value: &'a Constant<'a>,
    ) -> &'a Constant<'a> {
        arena.alloc(Constant::ProtoPair(
            first_type,
            second_type,
            first_value,
            second_value,
        ))
    }

    pub fn g1(arena: &'a Bump, g1: &'a blst::blst_p1) -> &'a Constant<'a> {
        arena.alloc(Constant::Bls12_381G1Element(g1))
    }

    pub fn g2(arena: &'a Bump, g2: &'a blst::blst_p2) -> &'a Constant<'a> {
        arena.alloc(Constant::Bls12_381G2Element(g2))
    }

    pub fn ml_result(arena: &'a Bump, ml_res: &'a blst::blst_fp12) -> &'a Constant<'a> {
        arena.alloc(Constant::Bls12_381MlResult(ml_res))
    }

    pub fn unwrap_data<V>(&'a self) -> Result<&'a PlutusData<'a>, MachineError<'a, V>>
    where
        V: Eval<'a>,
    {
        match self {
            Constant::Data(data) => Ok(data),
            _ => Err(MachineError::not_data(self)),
        }
    }

    pub fn type_of(&self, arena: &'a Bump) -> &'a Type<'a> {
        match self {
            Constant::Integer(_) => Type::integer(arena),
            Constant::ByteString(_) => Type::byte_string(arena),
            Constant::String(_) => Type::string(arena),
            Constant::Boolean(_) => Type::bool(arena),
            Constant::Data(_) => Type::data(arena),
            Constant::ProtoList(t, _) => Type::list(arena, t),
            Constant::ProtoPair(t1, t2, _, _) => Type::pair(arena, t1, t2),
            Constant::Unit => Type::unit(arena),
            Constant::Bls12_381G1Element(_) => Type::g1(arena),
            Constant::Bls12_381G2Element(_) => Type::g2(arena),
            Constant::Bls12_381MlResult(_) => Type::ml_result(arena),
        }
    }
}
