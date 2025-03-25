use bumpalo::collections::Vec as BumpVec;
use minicbor::data::{IanaTag, Tag};
use rug::ops::NegAssign;

use crate::data::PlutusData;

use super::Ctx;

impl<'a, 'b> minicbor::decode::Decode<'b, Ctx<'a>> for &'a PlutusData<'a> {
    fn decode(
        decoder: &mut minicbor::Decoder<'b>,
        ctx: &mut Ctx<'a>,
    ) -> Result<Self, minicbor::decode::Error> {
        let typ = decoder.datatype()?;

        match typ {
            minicbor::data::Type::Tag => {
                let mut probe = decoder.probe();

                let tag = probe.tag()?;

                if matches!(tag.as_u64(), 121..=127 | 1280..=1400 | 102) {
                    let x = decoder.tag()?.as_u64();

                    return match x {
                        121..=127 => {
                            let mut fields = BumpVec::new_in(ctx.arena);

                            for x in decoder.array_iter_with(ctx)? {
                                fields.push(x?);
                            }

                            let fields = ctx.arena.alloc(fields);

                            let data = PlutusData::constr(ctx.arena, x - 121, fields);

                            Ok(data)
                        }
                        1280..=1400 => {
                            let mut fields = BumpVec::new_in(ctx.arena);

                            for x in decoder.array_iter_with(ctx)? {
                                fields.push(x?);
                            }

                            let fields = ctx.arena.alloc(fields);

                            let data = PlutusData::constr(ctx.arena, (x - 1280) + 7, fields);

                            Ok(data)
                        }
                        102 => {
                            todo!("tagged data")
                        }
                        _ => {
                            let e = minicbor::decode::Error::message(format!(
                                "unknown tag for plutus data tag: {}",
                                tag
                            ));

                            Err(e)
                        }
                    };
                }

                match tag.try_into() {
                    Ok(x @ IanaTag::PosBignum | x @ IanaTag::NegBignum) => {
                        let mut bytes = BumpVec::new_in(ctx.arena);

                        for chunk in decoder.bytes_iter()? {
                            let chunk = chunk?;

                            bytes.extend_from_slice(chunk);
                        }

                        let integer = ctx
                            .arena
                            .alloc(rug::Integer::from_digits(&bytes, rug::integer::Order::Msf));

                        if x == IanaTag::NegBignum {
                            integer.neg_assign();
                        }

                        Ok(PlutusData::integer(ctx.arena, integer))
                    }

                    _ => {
                        let e = minicbor::decode::Error::message(format!(
                            "unknown tag for plutus data tag: {tag}",
                        ));

                        Err(e)
                    }
                }
            }
            minicbor::data::Type::Map | minicbor::data::Type::MapIndef => {
                let mut fields = BumpVec::new_in(ctx.arena);

                for x in decoder.map_iter_with(ctx)? {
                    let x = x?;

                    fields.push(x);
                }

                let fields = ctx.arena.alloc(fields);

                Ok(PlutusData::map(ctx.arena, fields))
            }
            minicbor::data::Type::Bytes | minicbor::data::Type::BytesIndef => {
                let mut bs = BumpVec::new_in(ctx.arena);

                for chunk in decoder.bytes_iter()? {
                    let chunk = chunk?;

                    bs.extend_from_slice(chunk);
                }

                let bs = ctx.arena.alloc(bs);

                Ok(PlutusData::byte_string(ctx.arena, bs))
            }
            minicbor::data::Type::Array | minicbor::data::Type::ArrayIndef => {
                let mut fields = BumpVec::new_in(ctx.arena);

                for x in decoder.array_iter_with(ctx)? {
                    fields.push(x?);
                }

                let fields = ctx.arena.alloc(fields);

                Ok(PlutusData::list(ctx.arena, fields))
            }
            minicbor::data::Type::U8
            | minicbor::data::Type::U16
            | minicbor::data::Type::U32
            | minicbor::data::Type::U64
            | minicbor::data::Type::I8
            | minicbor::data::Type::I16
            | minicbor::data::Type::I32
            | minicbor::data::Type::I64
            | minicbor::data::Type::Int => {
                let i: i128 = decoder.int()?.into();

                Ok(PlutusData::integer_from(ctx.arena, i))
            }
            any => {
                let e = minicbor::decode::Error::message(format!(
                    "bad cbor data type ({any:?}) for plutus data"
                ));

                Err(e)
            }
        }
    }
}

impl<C> minicbor::encode::Encode<C> for PlutusData<'_> {
    fn encode<W: minicbor::encode::Write>(
        &self,
        e: &mut minicbor::Encoder<W>,
        ctx: &mut C,
    ) -> Result<(), minicbor::encode::Error<W::Error>> {
        match self {
            PlutusData::Constr { tag, fields } => {
                e.tag(Tag::new(*tag))?;

                match tag {
                    102 => {
                        todo!("tagged data")
                    }
                    // TODO: figure out if we need to care about def vs indef
                    // if indef we need to call e.begin_array()?, then loop, then e.end()?;
                    _ => {
                        fields.encode(e, ctx)?;
                    }
                }
            }
            // stolen from pallas
            // we use definite array to match the approach used by haskell's plutus
            // implementation https://github.com/input-output-hk/plutus/blob/9538fc9829426b2ecb0628d352e2d7af96ec8204/plutus-core/plutus-core/src/PlutusCore/Data.hs#L152
            PlutusData::Map(map) => {
                let len: u64 = map
                    .len()
                    .try_into()
                    .expect("setting map length should work fine");

                e.map(len)?;

                for (k, v) in map.iter() {
                    k.encode(e, ctx)?;
                    v.encode(e, ctx)?;
                }
            }
            PlutusData::Integer(_) => todo!(),
            // we match the haskell implementation by encoding bytestrings longer than 64
            // bytes as indefinite lists of bytes
            PlutusData::ByteString(bs) => {
                const CHUNK_SIZE: usize = 64;

                if bs.len() <= 64 {
                    e.bytes(bs)?;
                } else {
                    e.begin_bytes()?;

                    for b in bs.chunks(CHUNK_SIZE) {
                        e.bytes(b)?;
                    }

                    e.end()?;
                }
            }
            PlutusData::List(_) => todo!(),
        }

        Ok(())
    }
}
