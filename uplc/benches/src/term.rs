use bumpalo::Bump;

use crate::{
    builtin::DefaultFunction,
    constant::{integer_from, Constant, Integer},
    data::PlutusData,
};

#[derive(Debug, PartialEq, Clone)]
pub enum Term<'a, V> {
    Var(&'a V),

    Lambda {
        parameter: &'a V,
        body: &'a Term<'a, V>,
    },

    Apply {
        function: &'a Term<'a, V>,
        argument: &'a Term<'a, V>,
    },

    Delay(&'a Term<'a, V>),

    Force(&'a Term<'a, V>),

    Case {
        constr: &'a Term<'a, V>,
        branches: &'a [&'a Term<'a, V>],
    },

    Constr {
        // TODO: revisit what the best type is for this
        tag: usize,
        fields: &'a [&'a Term<'a, V>],
    },

    Constant(&'a Constant<'a>),

    Builtin(&'a DefaultFunction),

    Error,
}

impl<'a, V> Term<'a, V> {
    pub fn var(arena: &'a Bump, i: &'a V) -> &'a Term<'a, V> {
        arena.alloc(Term::Var(i))
    }

    pub fn apply(&'a self, arena: &'a Bump, argument: &'a Term<'a, V>) -> &'a Term<'a, V> {
        arena.alloc(Term::Apply {
            function: self,
            argument,
        })
    }

    pub fn lambda(&'a self, arena: &'a Bump, parameter: &'a V) -> &'a Term<'a, V> {
        arena.alloc(Term::Lambda {
            parameter,
            body: self,
        })
    }

    pub fn force(&'a self, arena: &'a Bump) -> &'a Term<'a, V> {
        arena.alloc(Term::Force(self))
    }

    pub fn delay(&'a self, arena: &'a Bump) -> &'a Term<'a, V> {
        arena.alloc(Term::Delay(self))
    }

    pub fn constant(arena: &'a Bump, constant: &'a Constant<'a>) -> &'a Term<'a, V> {
        arena.alloc(Term::Constant(constant))
    }

    pub fn constr(arena: &'a Bump, tag: usize, fields: &'a [&'a Term<'a, V>]) -> &'a Term<'a, V> {
        arena.alloc(Term::Constr { tag, fields })
    }

    pub fn case(
        arena: &'a Bump,
        constr: &'a Term<'a, V>,
        branches: &'a [&'a Term<'a, V>],
    ) -> &'a Term<'a, V> {
        arena.alloc(Term::Case { constr, branches })
    }

    pub fn integer(arena: &'a Bump, i: &'a Integer) -> &'a Term<'a, V> {
        let constant = arena.alloc(Constant::Integer(i));

        Term::constant(arena, constant)
    }

    pub fn integer_from(arena: &'a Bump, i: i128) -> &'a Term<'a, V> {
        Self::integer(arena, integer_from(arena, i))
    }

    pub fn byte_string(arena: &'a Bump, bytes: &'a [u8]) -> &'a Term<'a, V> {
        let constant = Constant::byte_string(arena, bytes);

        Term::constant(arena, constant)
    }

    pub fn string(arena: &'a Bump, s: &'a str) -> &'a Term<'a, V> {
        let constant = Constant::string(arena, s);

        Term::constant(arena, constant)
    }

    pub fn bool(arena: &'a Bump, v: bool) -> &'a Term<'a, V> {
        let constant = Constant::bool(arena, v);

        Term::constant(arena, constant)
    }

    pub fn data(arena: &'a Bump, d: &'a PlutusData<'a>) -> &'a Term<'a, V> {
        let constant = Constant::data(arena, d);

        Term::constant(arena, constant)
    }

    pub fn data_byte_string(arena: &'a Bump, bytes: &'a [u8]) -> &'a Term<'a, V> {
        let data = PlutusData::byte_string(arena, bytes);

        Term::data(arena, data)
    }

    pub fn data_integer(arena: &'a Bump, i: &'a Integer) -> &'a Term<'a, V> {
        let data = PlutusData::integer(arena, i);

        Term::data(arena, data)
    }

    pub fn data_integer_from(arena: &'a Bump, i: i128) -> &'a Term<'a, V> {
        let data = PlutusData::integer_from(arena, i);

        Term::data(arena, data)
    }

    pub fn unit(arena: &'a Bump) -> &'a Term<'a, V> {
        let constant = Constant::unit(arena);

        Term::constant(arena, constant)
    }

    pub fn builtin(arena: &'a Bump, fun: &'a DefaultFunction) -> &'a Term<'a, V> {
        arena.alloc(Term::Builtin(fun))
    }

    pub fn error(arena: &'a Bump) -> &'a Term<'a, V> {
        arena.alloc(Term::Error)
    }

    pub fn add_integer(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::AddInteger);

        Term::builtin(arena, fun)
    }

    pub fn multiply_integer(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::MultiplyInteger);

        Term::builtin(arena, fun)
    }

    pub fn divide_integer(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::DivideInteger);

        Term::builtin(arena, fun)
    }

    pub fn quotient_integer(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::QuotientInteger);

        Term::builtin(arena, fun)
    }

    pub fn remainder_integer(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::RemainderInteger);

        Term::builtin(arena, fun)
    }

    pub fn mod_integer(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::ModInteger);

        Term::builtin(arena, fun)
    }

    pub fn subtract_integer(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::SubtractInteger);

        Term::builtin(arena, fun)
    }

    pub fn equals_integer(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::EqualsInteger);

        Term::builtin(arena, fun)
    }

    pub fn less_than_equals_integer(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::LessThanEqualsInteger);

        Term::builtin(arena, fun)
    }

    pub fn less_than_integer(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::LessThanInteger);

        Term::builtin(arena, fun)
    }

    pub fn if_then_else(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::IfThenElse);

        Term::builtin(arena, fun)
    }

    pub fn append_byte_string(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::AppendByteString);

        Term::builtin(arena, fun)
    }

    pub fn equals_byte_string(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::EqualsByteString);

        Term::builtin(arena, fun)
    }

    pub fn cons_byte_string(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::ConsByteString);

        Term::builtin(arena, fun)
    }

    pub fn slice_byte_string(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::SliceByteString);

        Term::builtin(arena, fun)
    }

    pub fn length_of_byte_string(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::LengthOfByteString);

        Term::builtin(arena, fun)
    }

    pub fn index_byte_string(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::IndexByteString);

        Term::builtin(arena, fun)
    }

    pub fn less_than_byte_string(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::LessThanByteString);

        Term::builtin(arena, fun)
    }

    pub fn less_than_equals_byte_string(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::LessThanEqualsByteString);

        Term::builtin(arena, fun)
    }

    pub fn sha2_256(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Sha2_256);

        Term::builtin(arena, fun)
    }

    pub fn sha3_256(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Sha3_256);

        Term::builtin(arena, fun)
    }

    pub fn blake2b_256(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Blake2b_256);

        Term::builtin(arena, fun)
    }

    pub fn keccak_256(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Keccak_256);

        Term::builtin(arena, fun)
    }

    pub fn blake2b_224(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Blake2b_224);

        Term::builtin(arena, fun)
    }

    pub fn verify_ed25519_signature(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::VerifyEd25519Signature);

        Term::builtin(arena, fun)
    }

    pub fn verify_ecdsa_secp256k1_signature(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::VerifyEcdsaSecp256k1Signature);

        Term::builtin(arena, fun)
    }

    pub fn verify_schnorr_secp256k1_signature(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::VerifySchnorrSecp256k1Signature);

        Term::builtin(arena, fun)
    }

    pub fn append_string(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::AppendString);

        Term::builtin(arena, fun)
    }

    pub fn equals_string(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::EqualsString);

        Term::builtin(arena, fun)
    }

    pub fn encode_utf8(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::EncodeUtf8);

        Term::builtin(arena, fun)
    }

    pub fn decode_utf8(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::DecodeUtf8);

        Term::builtin(arena, fun)
    }

    pub fn choose_unit(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::ChooseUnit);

        Term::builtin(arena, fun)
    }

    pub fn trace(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Trace);

        Term::builtin(arena, fun)
    }

    pub fn fst_pair(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::FstPair);

        Term::builtin(arena, fun)
    }

    pub fn snd_pair(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::SndPair);

        Term::builtin(arena, fun)
    }

    pub fn choose_list(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::ChooseList);

        Term::builtin(arena, fun)
    }

    pub fn mk_cons(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::MkCons);

        Term::builtin(arena, fun)
    }

    pub fn head_list(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::HeadList);

        Term::builtin(arena, fun)
    }

    pub fn tail_list(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::TailList);

        Term::builtin(arena, fun)
    }

    pub fn null_list(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::NullList);

        Term::builtin(arena, fun)
    }

    pub fn choose_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::ChooseData);

        Term::builtin(arena, fun)
    }

    pub fn constr_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::ConstrData);

        Term::builtin(arena, fun)
    }

    pub fn map_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::MapData);

        Term::builtin(arena, fun)
    }

    pub fn list_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::ListData);

        Term::builtin(arena, fun)
    }

    pub fn i_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::IData);

        Term::builtin(arena, fun)
    }

    pub fn b_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::BData);

        Term::builtin(arena, fun)
    }

    pub fn un_constr_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::UnConstrData);

        Term::builtin(arena, fun)
    }

    pub fn un_map_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::UnMapData);

        Term::builtin(arena, fun)
    }

    pub fn un_list_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::UnListData);

        Term::builtin(arena, fun)
    }

    pub fn un_i_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::UnIData);

        Term::builtin(arena, fun)
    }

    pub fn un_b_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::UnBData);

        Term::builtin(arena, fun)
    }

    pub fn equals_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::EqualsData);

        Term::builtin(arena, fun)
    }

    pub fn mk_pair_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::MkPairData);

        Term::builtin(arena, fun)
    }

    pub fn mk_nil_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::MkNilData);

        Term::builtin(arena, fun)
    }

    pub fn mk_nil_pair_data(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::MkNilPairData);

        Term::builtin(arena, fun)
    }

    pub fn bls12_381_g1_add(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G1_Add);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g1_neg(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G1_Neg);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g1_scalar_mul(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G1_ScalarMul);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g1_equal(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G1_Equal);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g1_compress(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G1_Compress);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g1_uncompress(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G1_Uncompress);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g1_hash_to_group(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G1_HashToGroup);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g2_add(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G2_Add);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g2_neg(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G2_Neg);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g2_scalar_mul(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G2_ScalarMul);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g2_equal(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G2_Equal);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g2_compress(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G2_Compress);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g2_uncompress(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G2_Uncompress);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_g2_hash_to_group(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_G2_HashToGroup);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_miller_loop(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_MillerLoop);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_mul_ml_result(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_MulMlResult);

        Term::builtin(arena, fun)
    }
    pub fn bls12_381_final_verify(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::Bls12_381_FinalVerify);

        Term::builtin(arena, fun)
    }
    pub fn integer_to_byte_string(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::IntegerToByteString);

        Term::builtin(arena, fun)
    }
    pub fn byte_string_to_integer(arena: &'a Bump) -> &'a Term<'a, V> {
        let fun = arena.alloc(DefaultFunction::ByteStringToInteger);

        Term::builtin(arena, fun)
    }
}
