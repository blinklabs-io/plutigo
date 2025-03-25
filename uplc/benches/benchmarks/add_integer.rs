use bumpalo::Bump;
use criterion::{criterion_group, Criterion};

use uplc::term::Term;

use super::utils;

pub fn run(c: &mut Criterion) {
    c.bench_function("add_integer", |b| {
        b.iter_with_setup(
            || {
                utils::setup_term(|arena: &Bump| {
                    Term::add_integer(arena)
                        .apply(arena, Term::integer_from(arena, 1))
                        .apply(arena, Term::integer_from(arena, 3))
                })
            },
            // Benchmark: only the eval call
            |state| state.exec(),
        )
    });
}

criterion_group!(add_integer, run);
