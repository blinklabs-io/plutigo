use bumpalo::collections::CollectIn;
use bumpalo::{collections::Vec as BumpVec, Bump};

use crate::{binder::Eval, term::Term};

use super::{env::Env, value::Value};

pub fn value_as_term<'a, V>(arena: &'a Bump, value: &'a Value<'a, V>) -> &'a Term<'a, V>
where
    V: Eval<'a>,
{
    match value {
        Value::Con(x) => arena.alloc(Term::Constant(x)),
        Value::Builtin(runtime) => {
            let mut term = Term::builtin(arena, runtime.fun);

            for _ in 0..runtime.forces {
                term = term.force(arena);
            }

            for arg in &runtime.args {
                term = term.apply(arena, value_as_term(arena, arg));
            }

            term
        }
        Value::Delay(body, env) => with_env(arena, 0, env, body.delay(arena)),
        Value::Lambda {
            parameter,
            body,
            env,
        } => with_env(arena, 0, env, body.lambda(arena, parameter)),
        Value::Constr(tag, fields) => {
            let fields: BumpVec<'_, _> = fields
                .iter()
                .map(|value| value_as_term(arena, value))
                .collect_in(arena);

            let fields = arena.alloc(fields);

            Term::constr(arena, *tag, fields)
        }
    }
}

fn with_env<'a, V>(
    arena: &'a Bump,
    lam_cnt: usize,
    env: &'a Env<'a, V>,
    term: &'a Term<'a, V>,
) -> &'a Term<'a, V>
where
    V: Eval<'a>,
{
    match term {
        Term::Var(name) => {
            let index = name.index();

            if lam_cnt >= index {
                Term::var(arena, name)
            } else {
                env.lookup(index - lam_cnt).map_or_else(
                    || Term::var(arena, *name),
                    |value| value_as_term(arena, value),
                )
            }
        }
        Term::Lambda { parameter, body } => {
            let body = with_env(arena, lam_cnt + 1, env, body);

            body.lambda(arena, *parameter)
        }
        Term::Apply { function, argument } => {
            let function = with_env(arena, lam_cnt, env, function);
            let argument = with_env(arena, lam_cnt, env, argument);

            function.apply(arena, argument)
        }

        Term::Delay(x) => {
            let body = with_env(arena, lam_cnt, env, x);

            body.delay(arena)
        }
        Term::Force(x) => {
            let body = with_env(arena, lam_cnt, env, x);

            body.force(arena)
        }
        rest => rest,
    }
}
