use chumsky::{prelude::*, Parser};

use crate::program::Version;

use super::types::{Extra, MapExtra};

pub fn parser<'a>() -> impl Parser<'a, &'a str, &'a mut Version<'a>, Extra<'a>> {
    text::int(10)
        .map(|v: &str| v.parse().unwrap())
        .then_ignore(just('.'))
        .then(text::int(10).map(|v: &str| v.parse().unwrap()))
        .then_ignore(just('.'))
        .then(text::int(10).map(|v: &str| v.parse().unwrap()))
        .map_with(|((major, minor), patch), e: &mut MapExtra<'a, '_>| {
            let state = e.state();

            let version = Version::new(state.arena, major, minor, patch);

            state.set_version(*version);

            version
        })
}
