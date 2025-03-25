mod decoder;
mod error;

pub use decoder::Ctx;
pub use decoder::Decoder;
pub use error::FlatDecodeError;

use bumpalo::{
    collections::{String as BumpString, Vec as BumpVec},
    Bump,
};

use crate::binder::Binder;
use crate::{
    constant::Constant,
    program::{Program, Version},
    term::Term,
};

use super::tag;
use super::{
    builtin,
    tag::{BUILTIN_TAG_WIDTH, CONST_TAG_WIDTH, TERM_TAG_WIDTH},
};

pub fn decode<'a, V>(arena: &'a Bump, bytes: &[u8]) -> Result<&'a Program<'a, V>, FlatDecodeError>
where
    V: Binder<'a>,
{
    let mut decoder = Decoder::new(bytes);

    let major = decoder.word()?;
    let minor = decoder.word()?;
    let patch = decoder.word()?;

    let version = Version::new(arena, major, minor, patch);

    let mut ctx = Ctx { arena };

    let term = decode_term(&mut ctx, &mut decoder)?;

    decoder.filler()?;

    Ok(Program::new(arena, version, term))
}

fn decode_term<'a, V>(
    ctx: &mut Ctx<'a>,
    decoder: &mut Decoder<'_>,
) -> Result<&'a Term<'a, V>, FlatDecodeError>
where
    V: Binder<'a>,
{
    let tag = decoder.bits8(TERM_TAG_WIDTH)?;

    match tag {
        // Var
        tag::VAR => Ok(Term::var(ctx.arena, V::var_decode(ctx.arena, decoder)?)),
        // Delay
        tag::DELAY => {
            let term = decode_term(ctx, decoder)?;

            Ok(term.delay(ctx.arena))
        }
        // Lambda
        tag::LAMBDA => {
            let param = V::parameter_decode(ctx.arena, decoder)?;

            let term = decode_term(ctx, decoder)?;

            Ok(term.lambda(ctx.arena, param))
        }
        // Apply
        tag::APPLY => {
            let function = decode_term(ctx, decoder)?;
            let argument = decode_term(ctx, decoder)?;

            let term = function.apply(ctx.arena, argument);

            Ok(term)
        }
        // Constant
        tag::CONSTANT => {
            let constant = decode_constant(ctx, decoder)?;

            Ok(Term::constant(ctx.arena, constant))
        }
        // Force
        tag::FORCE => {
            let term = decode_term(ctx, decoder)?;

            Ok(term.force(ctx.arena))
        }
        // Error
        tag::ERROR => Ok(Term::error(ctx.arena)),
        // Builtin
        tag::BUILTIN => {
            let builtin_tag = decoder.bits8(BUILTIN_TAG_WIDTH)?;

            let function = builtin::try_from_tag(ctx.arena, builtin_tag)?;

            let term = Term::builtin(ctx.arena, function);

            Ok(term)
        }
        // Constr
        tag::CONSTR => {
            let tag = decoder.word()?;
            let fields = decoder.list_with(ctx, decode_term)?;
            let fields = ctx.arena.alloc(fields);

            let term = Term::constr(ctx.arena, tag, fields);

            Ok(term)
        }
        // Case
        tag::CASE => {
            let constr = decode_term(ctx, decoder)?;
            let branches = decoder.list_with(ctx, decode_term)?;
            let branches = ctx.arena.alloc(branches);

            Ok(Term::case(ctx.arena, constr, branches))
        }
        _ => Err(FlatDecodeError::UnknownTermConstructor(tag)),
    }
}

// BLS literals not supported
fn decode_constant<'a>(
    ctx: &mut Ctx<'a>,
    d: &mut Decoder,
) -> Result<&'a Constant<'a>, FlatDecodeError> {
    let tags = decode_constant_tags(ctx, d)?;

    match &tags.as_slice() {
        [tag::INTEGER] => {
            let v = ctx.arena.alloc(d.integer()?);

            Ok(Constant::integer(ctx.arena, v))
        }
        [tag::BYTE_STRING] => {
            let b = d.bytes(ctx.arena)?;
            let b = ctx.arena.alloc(b);

            Ok(Constant::byte_string(ctx.arena, b))
        }
        [tag::STRING] => {
            let utf8_bytes = d.bytes(ctx.arena)?;

            let s = BumpString::from_utf8(utf8_bytes)
                .map_err(|e| FlatDecodeError::DecodeUtf8(e.utf8_error()))?;

            let s = ctx.arena.alloc(s);

            Ok(Constant::string(ctx.arena, s))
        }
        [tag::UNIT] => Ok(Constant::unit(ctx.arena)),
        [tag::BOOL] => {
            let v = d.bit()?;

            Ok(Constant::bool(ctx.arena, v))
        }
        [tag::PROTO_LIST_ONE, tag::PROTO_LIST_TWO, rest @ ..] => todo!("list"),

        [tag::PROTO_PAIR_ONE, tag::PROTO_PAIR_TWO, tag::PROTO_PAIR_THREE, rest @ ..] => {
            todo!("pair")
        }

        [tag::DATA] => {
            let cbor = d.bytes(ctx.arena)?;

            let data = minicbor::decode_with(&cbor, ctx)?;

            Ok(Constant::data(ctx.arena, data))
        }

        x => Err(FlatDecodeError::UnknownConstantConstructor(x.to_vec())),
    }
}

fn decode_constant_tags<'a>(
    ctx: &mut Ctx<'a>,
    d: &mut Decoder,
) -> Result<BumpVec<'a, u8>, FlatDecodeError> {
    d.list_with(ctx, |_arena, d| decode_constant_tag(d))
}

fn decode_constant_tag(d: &mut Decoder) -> Result<u8, FlatDecodeError> {
    d.bits8(CONST_TAG_WIDTH)
}
