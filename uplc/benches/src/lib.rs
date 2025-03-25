pub mod binder;
pub mod bls;
pub mod builtin;
pub mod constant;
pub mod data;
pub mod flat;
pub mod machine;
pub mod program;
pub mod syn;
pub mod term;
pub mod typ;

pub use bumpalo;

#[cfg(test)]
mod tests {
    use pretty_assertions::assert_eq;

    use crate::binder::DeBruijn;
    use crate::program::Version;

    use super::program::Program;
    use super::term::Term;

    #[test]
    fn add_integer() {
        let arena = bumpalo::Bump::new();

        let term = Term::add_integer(&arena)
            .apply(&arena, Term::integer_from(&arena, 1))
            .apply(&arena, Term::integer_from(&arena, 3));

        let version = Version::plutus_v3(&arena);

        let program = Program::<DeBruijn>::new(&arena, version, term);

        let result = program.eval(&arena);

        assert_eq!(result.term.unwrap(), Term::integer_from(&arena, 4));
    }

    #[test]
    fn fibonacci() {
        let arena = &bumpalo::Bump::new();

        let double_force = Term::var(arena, DeBruijn::new(arena, 1))
            .apply(arena, Term::var(arena, DeBruijn::new(arena, 1)))
            .lambda(arena, DeBruijn::zero(arena))
            .delay(arena)
            .force(arena)
            .apply(
                arena,
                Term::var(arena, DeBruijn::new(arena, 3))
                    .apply(
                        arena,
                        Term::var(arena, DeBruijn::new(arena, 1))
                            .apply(arena, Term::var(arena, DeBruijn::new(arena, 1)))
                            .lambda(arena, DeBruijn::zero(arena))
                            .delay(arena)
                            .force(arena)
                            .apply(arena, Term::var(arena, DeBruijn::new(arena, 2))),
                    )
                    .apply(arena, Term::var(arena, DeBruijn::new(arena, 1)))
                    .lambda(arena, DeBruijn::zero(arena))
                    .lambda(arena, DeBruijn::zero(arena)),
            )
            .lambda(arena, DeBruijn::zero(arena))
            .delay(arena)
            .delay(arena)
            .force(arena)
            .force(arena);

        let if_condition = Term::if_then_else(arena)
            .force(arena)
            .apply(arena, Term::var(arena, DeBruijn::new(arena, 3)))
            .apply(arena, Term::var(arena, DeBruijn::new(arena, 2)))
            .apply(arena, Term::var(arena, DeBruijn::new(arena, 1)))
            .apply(arena, Term::unit(arena))
            .lambda(arena, DeBruijn::zero(arena))
            .lambda(arena, DeBruijn::zero(arena))
            .lambda(arena, DeBruijn::zero(arena))
            .delay(arena)
            .force(arena);

        let add = Term::add_integer(arena)
            .apply(
                arena,
                Term::var(arena, DeBruijn::new(arena, 3)).apply(
                    arena,
                    Term::subtract_integer(arena)
                        .apply(arena, Term::var(arena, DeBruijn::new(arena, 2)))
                        .apply(arena, Term::integer_from(arena, 1)),
                ),
            )
            .apply(
                arena,
                Term::var(arena, DeBruijn::new(arena, 3)).apply(
                    arena,
                    Term::subtract_integer(arena)
                        .apply(arena, Term::var(arena, DeBruijn::new(arena, 2)))
                        .apply(arena, Term::integer_from(arena, 2)),
                ),
            )
            .lambda(arena, DeBruijn::zero(arena));

        let term = double_force
            .apply(
                arena,
                if_condition
                    .apply(
                        arena,
                        Term::less_than_equals_integer(arena)
                            .apply(arena, Term::var(arena, DeBruijn::new(arena, 1)))
                            .apply(arena, Term::integer_from(arena, 1)),
                    )
                    .apply(
                        arena,
                        Term::var(arena, DeBruijn::new(arena, 2))
                            .lambda(arena, DeBruijn::zero(arena)),
                    )
                    .apply(arena, add)
                    .lambda(arena, DeBruijn::zero(arena))
                    .lambda(arena, DeBruijn::zero(arena)),
            )
            .apply(arena, Term::var(arena, DeBruijn::new(arena, 1)))
            .lambda(arena, DeBruijn::zero(arena))
            .apply(arena, Term::integer_from(arena, 15));

        let version = Version::plutus_v3(arena);

        let program = Program::new(arena, version, term);

        let result = program.eval(arena);

        assert_eq!(result.term.unwrap(), Term::integer_from(arena, 610));
    }
}
