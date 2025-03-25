use chumsky::prelude::*;

use crate::typ::Type;

use super::types::{Extra, MapExtra};

pub fn parser<'a>() -> impl Parser<'a, &'a str, &'a Type<'a>, Extra<'a>> {
    recursive(|rec_typ| {
        choice((
            // integer
            text::keyword("integer")
                .ignored()
                .map_with(|_, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    Type::integer(state.arena)
                }),
            // bool
            text::keyword("bool")
                .ignored()
                .map_with(|_, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    Type::bool(state.arena)
                }),
            // bytestring
            text::keyword("bytestring")
                .ignored()
                .map_with(|_, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    Type::byte_string(state.arena)
                }),
            // string
            text::keyword("string")
                .ignored()
                .map_with(|_, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    Type::string(state.arena)
                }),
            // pair
            text::keyword("pair")
                .padded()
                .ignore_then(rec_typ.clone().padded())
                .then(rec_typ.clone().padded())
                .delimited_by(just('('), just(')'))
                .map_with(|(fst_type, snd_type), e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    Type::pair(state.arena, fst_type, snd_type)
                }),
            // list
            text::keyword("list")
                .padded()
                .ignore_then(rec_typ.padded())
                .delimited_by(just('('), just(')'))
                .map_with(|typ, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    Type::list(state.arena, typ)
                }),
            // data
            text::keyword("data")
                .ignored()
                .map_with(|_, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    Type::data(state.arena)
                }),
            // unit
            text::keyword("unit")
                .ignored()
                .map_with(|_, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    Type::unit(state.arena)
                }),
            // g1
            text::keyword("bls12_381_G1_element").ignored().map_with(
                |_, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    Type::g1(state.arena)
                },
            ),
            // g2
            text::keyword("bls12_381_G2_element").ignored().map_with(
                |_, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    Type::g2(state.arena)
                },
            ),
        ))
        .boxed()
    })
}
