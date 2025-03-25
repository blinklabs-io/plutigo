use bumpalo::{collections::Vec as BumpVec, Bump};

use crate::{
    binder::Eval,
    constant::{Constant, Integer},
    term::Term,
    typ::Type,
};

use super::{env::Env, runtime::Runtime, MachineError};

#[derive(Debug)]
pub enum Value<'a, V>
where
    V: Eval<'a>,
{
    Con(&'a Constant<'a>),
    Lambda {
        parameter: &'a V,
        body: &'a Term<'a, V>,
        env: &'a Env<'a, V>,
    },
    Builtin(&'a Runtime<'a, V>),
    Delay(&'a Term<'a, V>, &'a Env<'a, V>),
    Constr(usize, &'a [&'a Value<'a, V>]),
}

impl<'a, V> Value<'a, V>
where
    V: Eval<'a>,
{
    pub fn con(arena: &'a Bump, constant: &'a Constant<'a>) -> &'a Value<'a, V> {
        arena.alloc(Value::Con(constant))
    }

    pub fn lambda(
        arena: &'a Bump,
        parameter: &'a V,
        body: &'a Term<'a, V>,
        env: &'a Env<'a, V>,
    ) -> &'a Value<'a, V> {
        arena.alloc(Value::Lambda {
            parameter,
            body,
            env,
        })
    }

    pub fn delay(arena: &'a Bump, body: &'a Term<'a, V>, env: &'a Env<'a, V>) -> &'a Value<'a, V> {
        arena.alloc(Value::Delay(body, env))
    }

    pub fn constr_empty(arena: &'a Bump, tag: usize) -> &'a Value<'a, V> {
        let empty = BumpVec::new_in(arena);
        let empty = arena.alloc(empty);

        arena.alloc(Value::Constr(tag, empty))
    }

    pub fn constr(arena: &'a Bump, tag: usize, values: &'a [&'a Value<'a, V>]) -> &'a Value<'a, V> {
        arena.alloc(Value::Constr(tag, values))
    }

    pub fn builtin(arena: &'a Bump, runtime: &'a Runtime<'a, V>) -> &'a Value<'a, V> {
        arena.alloc(Value::Builtin(runtime))
    }

    pub fn integer(arena: &'a Bump, i: &'a Integer) -> &'a Value<'a, V> {
        let con = arena.alloc(Constant::Integer(i));

        Value::con(arena, con)
    }

    pub fn byte_string(arena: &'a Bump, b: &'a [u8]) -> &'a Value<'a, V> {
        let con = arena.alloc(Constant::ByteString(b));

        Value::con(arena, con)
    }

    pub fn string(arena: &'a Bump, s: &'a str) -> &'a Value<'a, V> {
        let con = arena.alloc(Constant::String(s));

        Value::con(arena, con)
    }

    pub fn bool(arena: &'a Bump, b: bool) -> &'a Value<'a, V> {
        let con = arena.alloc(Constant::Boolean(b));

        Value::con(arena, con)
    }

    pub fn unwrap_integer(&'a self) -> Result<&'a Integer, MachineError<'a, V>> {
        let inner = self.unwrap_constant()?;

        let Constant::Integer(integer) = inner else {
            return Err(MachineError::type_mismatch(Type::Integer, inner));
        };

        Ok(integer)
    }

    pub fn unwrap_byte_string(&'a self) -> Result<&'a [u8], MachineError<'a, V>> {
        let inner = self.unwrap_constant()?;

        let Constant::ByteString(byte_string) = inner else {
            return Err(MachineError::type_mismatch(Type::ByteString, inner));
        };

        Ok(byte_string)
    }

    pub fn unwrap_string(&'a self) -> Result<&'a str, MachineError<'a, V>> {
        let inner = self.unwrap_constant()?;

        let Constant::String(string) = inner else {
            return Err(MachineError::type_mismatch(Type::String, inner));
        };

        Ok(string)
    }

    pub fn unwrap_bool(&'a self) -> Result<bool, MachineError<'a, V>> {
        let inner = self.unwrap_constant()?;

        let Constant::Boolean(b) = inner else {
            return Err(MachineError::type_mismatch(Type::Bool, inner));
        };

        Ok(*b)
    }

    pub fn unwrap_pair(
        &'a self,
    ) -> Result<
        (
            &'a Type<'a>,
            &'a Type<'a>,
            &'a Constant<'a>,
            &'a Constant<'a>,
        ),
        MachineError<'a, V>,
    > {
        let inner = self.unwrap_constant()?;

        let Constant::ProtoPair(t1, t2, first, second) = inner else {
            return Err(MachineError::expected_pair(inner));
        };

        Ok((t1, t2, first, second))
    }

    pub fn unwrap_list(
        &'a self,
    ) -> Result<(&'a Type<'a>, &'a [&'a Constant<'a>]), MachineError<'a, V>> {
        let inner = self.unwrap_constant()?;

        let Constant::ProtoList(t1, list) = inner else {
            return Err(MachineError::expected_list(inner));
        };

        Ok((t1, list))
    }

    pub fn unwrap_map(
        &'a self,
    ) -> Result<(&'a Type<'a>, &'a [&'a Constant<'a>]), MachineError<'a, V>> {
        let inner = self.unwrap_constant()?;

        let Constant::ProtoList(t1, list) = inner else {
            return Err(MachineError::expected_list(inner));
        };

        Ok((t1, list))
    }

    pub fn unwrap_constant(&'a self) -> Result<&'a Constant<'a>, MachineError<'a, V>> {
        let Value::Con(item) = self else {
            return Err(MachineError::NotAConstant(self));
        };

        Ok(item)
    }

    pub fn unwrap_unit(&'a self) -> Result<(), MachineError<'a, V>> {
        let inner = self.unwrap_constant()?;

        let Constant::Unit = inner else {
            return Err(MachineError::type_mismatch(Type::Unit, inner));
        };

        Ok(())
    }

    pub fn unwrap_bls12_381_g1_element(&'a self) -> Result<&'a blst::blst_p1, MachineError<'a, V>> {
        let inner = self.unwrap_constant()?;

        let Constant::Bls12_381G1Element(g1) = inner else {
            return Err(MachineError::type_mismatch(Type::Bls12_381G1Element, inner));
        };

        Ok(g1)
    }

    pub fn unwrap_bls12_381_g2_element(&'a self) -> Result<&'a blst::blst_p2, MachineError<'a, V>> {
        let inner = self.unwrap_constant()?;

        let Constant::Bls12_381G2Element(g2) = inner else {
            return Err(MachineError::type_mismatch(Type::Bls12_381G2Element, inner));
        };

        Ok(g2)
    }

    pub fn unwrap_bls12_381_ml_result(
        &'a self,
    ) -> Result<&'a blst::blst_fp12, MachineError<'a, V>> {
        let inner = self.unwrap_constant()?;

        let Constant::Bls12_381MlResult(ml_res) = inner else {
            return Err(MachineError::type_mismatch(Type::Bls12_381MlResult, inner));
        };

        Ok(ml_res)
    }
}

impl<'a> Constant<'a> {
    pub fn value<V>(&'a self, arena: &'a Bump) -> &'a Value<'a, V>
    where
        V: Eval<'a>,
    {
        Value::con(arena, self)
    }
}
