mod encoder;
mod error;

pub use encoder::Encoder;
pub use error::FlatEncodeError;

use crate::{binder::Binder, constant::Constant, program::Program, term::Term, typ::Type};

use super::tag;

pub fn encode<'a, V>(program: &'a Program<'a, V>) -> Result<Vec<u8>, FlatEncodeError>
where
    V: Binder<'a>,
{
    let mut encoder = Encoder::default();

    encoder
        .word(program.version.major())
        .word(program.version.minor())
        .word(program.version.patch());

    encode_term(&mut encoder, program.term)?;

    encoder.filler();

    Ok(encoder.buffer)
}

fn encode_term<'a, V>(encoder: &mut Encoder, term: &'a Term<'a, V>) -> Result<(), FlatEncodeError>
where
    V: Binder<'a>,
{
    match term {
        Term::Var(name) => {
            encode_term_tag(encoder, tag::VAR)?;

            name.var_encode(encoder)?;
        }
        Term::Lambda { parameter, body } => {
            encode_term_tag(encoder, tag::LAMBDA)?;

            parameter.parameter_encode(encoder)?;

            encode_term(encoder, body)?;
        }
        Term::Apply { function, argument } => {
            encode_term_tag(encoder, tag::APPLY)?;

            encode_term(encoder, function)?;

            encode_term(encoder, argument)?;
        }
        Term::Delay(body) => {
            encode_term_tag(encoder, tag::DELAY)?;

            encode_term(encoder, body)?;
        }
        Term::Force(body) => {
            encode_term_tag(encoder, tag::FORCE)?;

            encode_term(encoder, body)?;
        }
        Term::Case { constr, branches } => {
            encode_term_tag(encoder, tag::CASE)?;

            encode_term(encoder, constr)?;

            encoder.list_with(branches, |e, t| encode_term(e, t))?;
        }
        Term::Constr { tag, fields } => {
            encode_term_tag(encoder, tag::CONSTR)?;

            encoder.word(*tag);

            encoder.list_with(fields, |e, t| encode_term(e, t))?;
        }
        Term::Constant(c) => {
            encode_term_tag(encoder, tag::CONSTANT)?;

            encode_constant(encoder, c)?;
        }
        Term::Builtin(b) => {
            encode_term_tag(encoder, tag::BUILTIN)?;

            encoder.bits(tag::BUILTIN_TAG_WIDTH as i64, **b as u8);
        }
        Term::Error => {
            encode_term_tag(encoder, tag::ERROR)?;
        }
    }

    Ok(())
}

fn encode_constant<'a>(e: &mut Encoder, constant: &'a Constant<'a>) -> Result<(), FlatEncodeError> {
    match constant {
        Constant::Integer(i) => {
            e.list_with(&[tag::INTEGER], encode_constant_tag)?;

            e.integer(i);
        }
        Constant::ByteString(b) => {
            e.list_with(&[tag::BYTE_STRING], encode_constant_tag)?;

            e.bytes(b)?;
        }
        Constant::String(s) => {
            e.list_with(&[tag::STRING], encode_constant_tag)?;

            e.utf8(s)?;
        }
        Constant::Unit => {
            e.list_with(&[tag::UNIT], encode_constant_tag)?;
        }
        Constant::Boolean(b) => {
            e.list_with(&[tag::BOOL], encode_constant_tag)?;

            e.bool(*b);
        }
        Constant::Data(data) => {
            e.list_with(&[tag::DATA], encode_constant_tag)?;

            let data = minicbor::to_vec(*data)?;

            e.bytes(&data)?;
        }
        Constant::ProtoList(typ, list) => {
            let mut type_encodings = vec![tag::PROTO_LIST_ONE, tag::PROTO_LIST_TWO];

            encode_type(typ, &mut type_encodings)?;

            e.list_with(&type_encodings, encode_constant_tag)?;

            e.list_with(list, encode_constant_value)?;
        }
        Constant::ProtoPair(fst_type, snd_type, fst, snd) => {
            let mut type_encodings = vec![
                tag::PROTO_PAIR_ONE,
                tag::PROTO_PAIR_TWO,
                tag::PROTO_PAIR_THREE,
            ];

            encode_type(fst_type, &mut type_encodings)?;

            encode_type(snd_type, &mut type_encodings)?;

            e.list_with(&type_encodings, encode_constant_tag)?;

            encode_constant_value(e, fst)?;
            encode_constant_value(e, snd)?;
        }
        Constant::Bls12_381G1Element(_)
        | Constant::Bls12_381G2Element(_)
        | Constant::Bls12_381MlResult(_) => return Err(FlatEncodeError::BlsElementNotSupported),
    }

    Ok(())
}

fn encode_term_tag(e: &mut Encoder, tag: u8) -> Result<(), FlatEncodeError> {
    safe_encode_bits(e, tag::TERM_TAG_WIDTH, tag)
}

fn encode_constant_tag(e: &mut Encoder, tag: &u8) -> Result<(), FlatEncodeError> {
    safe_encode_bits(e, tag::CONST_TAG_WIDTH, *tag)
}

fn encode_type(typ: &Type, bytes: &mut Vec<u8>) -> Result<(), FlatEncodeError> {
    match typ {
        Type::Integer => bytes.push(tag::INTEGER),
        Type::ByteString => bytes.push(tag::BYTE_STRING),
        Type::String => bytes.push(tag::STRING),
        Type::Unit => bytes.push(tag::UNIT),
        Type::Bool => bytes.push(tag::BOOL),
        Type::List(sub_typ) => {
            bytes.extend(vec![tag::PROTO_LIST_ONE, tag::PROTO_LIST_TWO]);

            encode_type(sub_typ, bytes)?;
        }
        Type::Pair(type1, type2) => {
            bytes.extend(vec![
                tag::PROTO_PAIR_ONE,
                tag::PROTO_PAIR_TWO,
                tag::PROTO_PAIR_THREE,
            ]);

            encode_type(type1, bytes)?;
            encode_type(type2, bytes)?;
        }
        Type::Data => bytes.push(tag::DATA),
        Type::Bls12_381G1Element | Type::Bls12_381G2Element | Type::Bls12_381MlResult => {
            return Err(FlatEncodeError::BlsElementNotSupported)
        }
    }

    Ok(())
}

fn encode_constant_value<'a>(e: &mut Encoder, x: &'a &Constant<'a>) -> Result<(), FlatEncodeError> {
    match *x {
        Constant::Integer(x) => {
            e.integer(x);
        }
        Constant::ByteString(b) => {
            e.bytes(b)?;
        }
        Constant::String(s) => {
            e.utf8(s)?;
        }
        Constant::Unit => (),
        Constant::Boolean(b) => {
            e.bool(*b);
        }
        Constant::ProtoList(_, list) => {
            e.list_with(list, encode_constant_value)?;
        }
        Constant::ProtoPair(_, _, a, b) => {
            encode_constant_value(e, a)?;

            encode_constant_value(e, b)?;
        }
        Constant::Data(data) => {
            todo!();
        }
        Constant::Bls12_381G1Element(_)
        | Constant::Bls12_381G2Element(_)
        | Constant::Bls12_381MlResult(_) => return Err(FlatEncodeError::BlsElementNotSupported),
    }

    Ok(())
}

fn safe_encode_bits(e: &mut Encoder, num_bits: usize, byte: u8) -> Result<(), FlatEncodeError> {
    if 2_u8.pow(num_bits as u32) <= byte {
        Err(FlatEncodeError::Overflow { byte, num_bits })
    } else {
        e.bits(num_bits as i64, byte);

        Ok(())
    }
}
