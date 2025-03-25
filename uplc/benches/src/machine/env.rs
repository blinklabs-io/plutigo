use bumpalo::{collections::Vec as BumpVec, Bump};

use crate::binder::Eval;

use super::value::Value;

#[derive(Debug)]
pub struct Env<'a, V>(BumpVec<'a, &'a Value<'a, V>>)
where
    V: Eval<'a>;

impl<'a, V> Env<'a, V>
where
    V: Eval<'a>,
{
    pub fn new_in(arena: &'a Bump) -> &'a Self {
        arena.alloc(Self(BumpVec::new_in(arena)))
    }

    pub fn push(&'a self, arena: &'a Bump, argument: &'a Value<'a, V>) -> &'a Self {
        let mut new_env = self.0.clone();

        new_env.push(argument);

        arena.alloc(Self(new_env))
    }

    pub fn lookup(&'a self, name: usize) -> Option<&'a Value<'a, V>> {
        self.0.get(self.0.len() - name).copied()
    }
}
