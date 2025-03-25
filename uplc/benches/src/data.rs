use bumpalo::Bump;

use crate::{
    binder::Eval,
    constant::{integer_from, Constant, Integer},
    flat::Ctx,
    machine::MachineError,
};

#[derive(Debug, PartialEq)]
pub enum PlutusData<'a> {
    Constr {
        tag: u64,
        fields: &'a [&'a PlutusData<'a>],
    },
    Map(&'a [(&'a PlutusData<'a>, &'a PlutusData<'a>)]),
    Integer(&'a Integer),
    ByteString(&'a [u8]),
    List(&'a [&'a PlutusData<'a>]),
}

impl<'a> PlutusData<'a> {
    pub fn constr(
        arena: &'a Bump,
        tag: u64,
        fields: &'a [&'a PlutusData<'a>],
    ) -> &'a PlutusData<'a> {
        arena.alloc(PlutusData::Constr { tag, fields })
    }

    pub fn list(arena: &'a Bump, items: &'a [&'a PlutusData<'a>]) -> &'a PlutusData<'a> {
        arena.alloc(PlutusData::List(items))
    }

    pub fn map(
        arena: &'a Bump,
        items: &'a [(&'a PlutusData<'a>, &'a PlutusData<'a>)],
    ) -> &'a PlutusData<'a> {
        arena.alloc(PlutusData::Map(items))
    }

    pub fn integer(arena: &'a Bump, i: &'a Integer) -> &'a PlutusData<'a> {
        arena.alloc(PlutusData::Integer(i))
    }

    pub fn integer_from(arena: &'a Bump, i: i128) -> &'a PlutusData<'a> {
        arena.alloc(PlutusData::Integer(integer_from(arena, i)))
    }

    pub fn byte_string(arena: &'a Bump, bytes: &'a [u8]) -> &'a PlutusData<'a> {
        arena.alloc(PlutusData::ByteString(bytes))
    }

    pub fn from_cbor(
        arena: &'a Bump,
        cbor: &'_ [u8],
    ) -> Result<&'a PlutusData<'a>, minicbor::decode::Error> {
        minicbor::decode_with(cbor, &mut Ctx { arena })
    }

    pub fn unwrap_constr<V>(
        &'a self,
    ) -> Result<(&'a u64, &'a [&'a PlutusData<'a>]), MachineError<'a, V>>
    where
        V: Eval<'a>,
    {
        match self {
            PlutusData::Constr { tag, fields } => Ok((tag, fields)),
            _ => Err(MachineError::malformed_data(self)),
        }
    }

    pub fn unwrap_map<V>(
        &'a self,
    ) -> Result<&'a [(&'a PlutusData<'a>, &'a PlutusData<'a>)], MachineError<'a, V>>
    where
        V: Eval<'a>,
    {
        match self {
            PlutusData::Map(fields) => Ok(fields),
            _ => Err(MachineError::malformed_data(self)),
        }
    }

    pub fn unwrap_integer<V>(&'a self) -> Result<&'a Integer, MachineError<'a, V>>
    where
        V: Eval<'a>,
    {
        match self {
            PlutusData::Integer(i) => Ok(i),
            _ => Err(MachineError::malformed_data(self)),
        }
    }

    pub fn unwrap_byte_string<V>(&'a self) -> Result<&'a [u8], MachineError<'a, V>>
    where
        V: Eval<'a>,
    {
        match self {
            PlutusData::ByteString(bytes) => Ok(bytes),
            _ => Err(MachineError::malformed_data(self)),
        }
    }

    pub fn unwrap_list<V>(&'a self) -> Result<&'a [&'a PlutusData<'a>], MachineError<'a, V>>
    where
        V: Eval<'a>,
    {
        match self {
            PlutusData::List(items) => Ok(items),
            _ => Err(MachineError::malformed_data(self)),
        }
    }

    pub fn constant(&'a self, arena: &'a Bump) -> &'a Constant<'a> {
        Constant::data(arena, self)
    }
}
