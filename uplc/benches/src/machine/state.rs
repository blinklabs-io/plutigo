use bumpalo::Bump;

use crate::{binder::Eval, term::Term};

use super::{context::Context, env::Env, value::Value};

pub enum MachineState<'a, V>
where
    V: Eval<'a>,
{
    Return(&'a Context<'a, V>, &'a Value<'a, V>),
    Compute(&'a Context<'a, V>, &'a Env<'a, V>, &'a Term<'a, V>),
    Done(&'a Term<'a, V>),
}

impl<'a, V> MachineState<'a, V>
where
    V: Eval<'a>,
{
    pub fn compute(
        arena: &'a Bump,
        context: &'a Context<'a, V>,
        env: &'a Env<'a, V>,
        term: &'a Term<'a, V>,
    ) -> &'a mut MachineState<'a, V> {
        arena.alloc(MachineState::Compute(context, env, term))
    }

    pub fn return_(
        arena: &'a Bump,
        context: &'a Context<'a, V>,
        value: &'a Value<'a, V>,
    ) -> &'a mut MachineState<'a, V> {
        arena.alloc(MachineState::Return(context, value))
    }

    pub fn done(arena: &'a Bump, term: &'a Term<'a, V>) -> &'a mut MachineState<'a, V> {
        arena.alloc(MachineState::Done(term))
    }
}
