use chumsky::prelude::*;

use super::types::Extra;

pub fn hex_digit<'a>() -> impl Parser<'a, &'a str, u8, Extra<'a>> {
    one_of("0123456789abcdefABCDEF").map(|c: char| c.to_digit(16).unwrap() as u8)
}

pub fn hex_bytes<'a>() -> impl Parser<'a, &'a str, Vec<u8>, Extra<'a>> {
    hex_digit()
        .then(hex_digit())
        .map(|(high, low)| (high << 4) | low)
        .repeated()
        .collect()
}

pub fn comments<'a>() -> impl Parser<'a, &'a str, (), Extra<'a>> {
    just("--")
        .then(any().and_is(just('\n').not()).repeated())
        .padded()
        .repeated()
}
