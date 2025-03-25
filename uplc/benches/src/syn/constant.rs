use bumpalo::{
    collections::{String as BumpString, Vec as BumpVec},
    Bump,
};
use chumsky::prelude::*;
use rug::ops::NegAssign;

use crate::{
    bls::Compressable,
    constant::{self, Constant, Integer},
    data::PlutusData,
    typ::Type,
};

use super::{
    data, typ,
    types::{Extra, MapExtra},
    utils::hex_bytes,
};

pub fn parser<'a>() -> impl Parser<'a, &'a str, &'a Constant<'a>, Extra<'a>> {
    text::keyword("con")
        .padded()
        .ignore_then(typ::parser().padded())
        .then(value_parser().padded())
        .delimited_by(just('('), just(')'))
        .validate(|(ty, constant), e: &mut MapExtra<'a, '_>, emitter| {
            let state = e.state();

            let (constant, is_correct) = check_type(state.arena, constant, ty);

            if !is_correct {
                // emit an error
                emitter.emit(Rich::custom(e.span(), "type mismatch"));
            }

            constant
        })
}

fn check_type<'a>(
    arena: &'a Bump,
    con: TempConstant<'a>,
    expected_type: &'a Type<'a>,
) -> (&'a Constant<'a>, bool) {
    let constant = match (con, expected_type) {
        (TempConstant::Integer(i), Type::Integer) => Constant::integer(arena, i),
        (TempConstant::ByteString(b), Type::ByteString) => Constant::byte_string(arena, b),
        (TempConstant::String(s), Type::String) => Constant::string(arena, s),
        (TempConstant::Boolean(b), Type::Bool) => Constant::bool(arena, b),
        (TempConstant::Data(d), Type::Data) => Constant::data(arena, d),
        (TempConstant::Unit, Type::Unit) => Constant::unit(arena),

        (TempConstant::ProtoList(list), Type::List(inner)) => {
            let mut constants = BumpVec::with_capacity_in(list.len(), arena);

            for con in list {
                let (constant, is_correct) = check_type(arena, con, inner);

                if !is_correct {
                    return (Constant::unit(arena), false);
                }

                constants.push(constant);
            }

            let constants = arena.alloc(constants);

            Constant::proto_list(arena, inner, constants)
        }

        (TempConstant::ProtoPair(fst, snd), Type::Pair(fst_ty, snd_ty)) => {
            let (fst, fst_correct) = check_type(arena, *fst, fst_ty);
            let (snd, snd_correct) = check_type(arena, *snd, snd_ty);

            if !fst_correct || !snd_correct {
                return (Constant::unit(arena), false);
            }

            Constant::proto_pair(arena, fst_ty, snd_ty, fst, snd)
        }

        (TempConstant::BlsElement(element), Type::Bls12_381G1Element) => {
            let Ok(element) = blst::blst_p1::uncompress(arena, &element) else {
                return (Constant::unit(arena), false);
            };

            Constant::g1(arena, element)
        }

        (TempConstant::BlsElement(element), Type::Bls12_381G2Element) => {
            let Ok(element) = blst::blst_p2::uncompress(arena, &element) else {
                return (Constant::unit(arena), false);
            };

            Constant::g2(arena, element)
        }
        _ => return (Constant::unit(arena), false),
    };

    (constant, true)
}

#[derive(Debug, PartialEq)]
enum TempConstant<'a> {
    Integer(&'a Integer),
    ByteString(&'a [u8]),
    String(&'a str),
    Boolean(bool),
    Data(&'a PlutusData<'a>),
    ProtoList(BumpVec<'a, TempConstant<'a>>),
    ProtoPair(Box<TempConstant<'a>>, Box<TempConstant<'a>>),
    BlsElement(Vec<u8>),
    Unit,
}

fn value_parser<'a>() -> impl Parser<'a, &'a str, TempConstant<'a>, Extra<'a>> {
    recursive(|con| {
        choice((
            // integer
            just('-')
                .ignored()
                .or_not()
                .padded()
                .then_ignore(just('+').padded().or_not())
                .then_ignore(just('0').repeated().or_not())
                .then(text::int(10))
                .padded()
                .map_with(|(maybe_negative, v), e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    let i = state.arena.alloc(Integer::from_str_radix(v, 10).unwrap());

                    if maybe_negative.is_some() {
                        i.neg_assign();
                    };

                    TempConstant::Integer(i)
                }),
            // bls element
            just("0x")
                .ignore_then(hex_bytes())
                .padded()
                .map(TempConstant::BlsElement),
            just('0')
                .padded()
                .to_slice()
                .map_with(|v, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    let value = v.parse::<i128>().unwrap();

                    let i = constant::integer_from(state.arena, value);

                    TempConstant::Integer(i)
                }),
            // bytestring
            just('#')
                .ignore_then(hex_bytes())
                .padded()
                .map_with(|v, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    let bytes = BumpVec::from_iter_in(v, state.arena);
                    let bytes = state.arena.alloc(bytes);

                    TempConstant::ByteString(bytes)
                }),
            // string any utf8 encoded string surrounded in double quotes
            just('"')
                .ignore_then(string_content())
                .then_ignore(just('"'))
                .padded()
                .map_with(|v, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    let string = BumpString::from_str_in(&v, state.arena);
                    let string = state.arena.alloc(string);

                    TempConstant::String(string)
                }),
            // plutus data
            data::parser()
                .delimited_by(just('('), just(')'))
                .map(TempConstant::Data),
            // list
            con.clone()
                .separated_by(just(','))
                .allow_trailing()
                .collect()
                .delimited_by(just('['), just(']'))
                .map_with(|fields: Vec<TempConstant<'_>>, e: &mut MapExtra<'a, '_>| {
                    let state = e.state();

                    let fields = BumpVec::from_iter_in(fields, state.arena);

                    TempConstant::ProtoList(fields)
                }),
            // pair
            con.clone()
                .padded()
                .then_ignore(just(','))
                .then(con.padded())
                .delimited_by(just('('), just(')'))
                .map(|(fst_value, snd_value)| {
                    TempConstant::ProtoPair(fst_value.into(), snd_value.into())
                }),
            // bool
            choice((just("False"), just("True")))
                .padded()
                .map(|v| TempConstant::Boolean(v == "True")),
            // unit
            just("()").padded().ignored().map(|_v| TempConstant::Unit),
        ))
        .boxed()
    })
}

fn string_content<'a>() -> impl Parser<'a, &'a str, String, Extra<'a>> {
    let escape_sequence = just('\\').ignore_then(choice((
        just('a').to('\u{07}'),
        just('b').to('\u{08}'),
        just('f').to('\u{0C}'),
        just('n').to('\n'),
        just('r').to('\r'),
        just('t').to('\t'),
        just('v').to('\u{0B}'),
        just('\\'),
        just('"'),
        just('\''),
        just('&'),
        just('x').ignore_then(
            any()
                .filter(|c: &char| c.is_ascii_hexdigit())
                .repeated()
                .at_least(1)
                .collect::<String>()
                .validate(|s, e, emitter| {
                    u32::from_str_radix(&s, 16)
                        .ok()
                        .and_then(char::from_u32)
                        .unwrap_or_else(|| {
                            emitter.emit(Rich::custom(e.span(), "Invalid hex escape"));

                            '0'
                        })
                }),
        ),
        just('o').ignore_then(
            any()
                .filter(|c: &char| c.is_digit(8))
                .repeated()
                .at_least(1)
                .collect::<String>()
                .validate(|s, e, emitter| {
                    u32::from_str_radix(&s, 8)
                        .ok()
                        .and_then(char::from_u32)
                        .unwrap_or_else(|| {
                            emitter.emit(Rich::custom(e.span(), "Invalid octal escape"));

                            '0'
                        })
                }),
        ),
        any()
            .filter(|c: &char| c.is_ascii_digit())
            .repeated()
            .at_least(1)
            .collect::<String>()
            .validate(|s, e, emitter| {
                s.parse::<u32>()
                    .ok()
                    .and_then(char::from_u32)
                    .unwrap_or_else(|| {
                        emitter.emit(Rich::custom(e.span(), "Invalid decimal escape"));

                        '0'
                    })
            }),
    )));

    let regular_char = none_of("\\\"");

    choice((regular_char, escape_sequence))
        .repeated()
        .collect::<String>()
}
