use crate::machine::ExBudget;

use super::costing::{
    Cost, OneArgumentCosting, SixArgumentsCosting, ThreeArgumentsCosting, TwoArgumentsCosting,
};

#[derive(Debug, PartialEq)]
pub struct BuiltinCosts {
    add_integer: TwoArgumentsCosting,
    subtract_integer: TwoArgumentsCosting,
    multiply_integer: TwoArgumentsCosting,
    divide_integer: TwoArgumentsCosting,
    quotient_integer: TwoArgumentsCosting,
    remainder_integer: TwoArgumentsCosting,
    mod_integer: TwoArgumentsCosting,
    equals_integer: TwoArgumentsCosting,
    less_than_integer: TwoArgumentsCosting,
    less_than_equals_integer: TwoArgumentsCosting,
    // Bytestrings
    append_byte_string: TwoArgumentsCosting,
    cons_byte_string: TwoArgumentsCosting,
    slice_byte_string: ThreeArgumentsCosting,
    length_of_byte_string: OneArgumentCosting,
    index_byte_string: TwoArgumentsCosting,
    equals_byte_string: TwoArgumentsCosting,
    less_than_byte_string: TwoArgumentsCosting,
    less_than_equals_byte_string: TwoArgumentsCosting,
    // Cryptography and hashes
    sha2_256: OneArgumentCosting,
    sha3_256: OneArgumentCosting,
    blake2b_224: OneArgumentCosting,
    blake2b_256: OneArgumentCosting,
    keccak_256: OneArgumentCosting,
    verify_ed25519_signature: ThreeArgumentsCosting,
    verify_ecdsa_secp256k1_signature: ThreeArgumentsCosting,
    verify_schnorr_secp256k1_signature: ThreeArgumentsCosting,
    // Strings
    append_string: TwoArgumentsCosting,
    equals_string: TwoArgumentsCosting,
    encode_utf8: OneArgumentCosting,
    decode_utf8: OneArgumentCosting,
    // Bool
    if_then_else: ThreeArgumentsCosting,
    // Unit
    choose_unit: TwoArgumentsCosting,
    // Tracing
    trace: TwoArgumentsCosting,
    // Pairs
    fst_pair: OneArgumentCosting,
    snd_pair: OneArgumentCosting,
    // Lists
    choose_list: ThreeArgumentsCosting,
    mk_cons: TwoArgumentsCosting,
    head_list: OneArgumentCosting,
    tail_list: OneArgumentCosting,
    null_list: OneArgumentCosting,
    // Data
    choose_data: SixArgumentsCosting,
    constr_data: TwoArgumentsCosting,
    map_data: OneArgumentCosting,
    list_data: OneArgumentCosting,
    i_data: OneArgumentCosting,
    b_data: OneArgumentCosting,
    un_constr_data: OneArgumentCosting,
    un_map_data: OneArgumentCosting,
    un_list_data: OneArgumentCosting,
    un_i_data: OneArgumentCosting,
    un_b_data: OneArgumentCosting,
    equals_data: TwoArgumentsCosting,
    // Misc constructors
    mk_pair_data: TwoArgumentsCosting,
    mk_nil_data: OneArgumentCosting,
    mk_nil_pair_data: OneArgumentCosting,
    serialise_data: OneArgumentCosting,
    // BLST
    bls12_381_g1_add: TwoArgumentsCosting,
    bls12_381_g1_neg: OneArgumentCosting,
    bls12_381_g1_scalar_mul: TwoArgumentsCosting,
    bls12_381_g1_equal: TwoArgumentsCosting,
    bls12_381_g1_compress: OneArgumentCosting,
    bls12_381_g1_uncompress: OneArgumentCosting,
    bls12_381_g1_hash_to_group: TwoArgumentsCosting,
    bls12_381_g2_add: TwoArgumentsCosting,
    bls12_381_g2_neg: OneArgumentCosting,
    bls12_381_g2_scalar_mul: TwoArgumentsCosting,
    bls12_381_g2_equal: TwoArgumentsCosting,
    bls12_381_g2_compress: OneArgumentCosting,
    bls12_381_g2_uncompress: OneArgumentCosting,
    bls12_381_g2_hash_to_group: TwoArgumentsCosting,
    bls12_381_miller_loop: TwoArgumentsCosting,
    bls12_381_mul_ml_result: TwoArgumentsCosting,
    bls12_381_final_verify: TwoArgumentsCosting,
    // bitwise
    integer_to_byte_string: ThreeArgumentsCosting,
    byte_string_to_integer: TwoArgumentsCosting,
}

impl Default for BuiltinCosts {
    fn default() -> Self {
        BuiltinCosts::v3()
    }
}

impl BuiltinCosts {
    pub fn add_integer(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.add_integer.mem.cost(args),
            self.add_integer.cpu.cost(args),
        )
    }

    pub fn subtract_integer(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.subtract_integer.mem.cost(args),
            self.subtract_integer.cpu.cost(args),
        )
    }

    pub fn equals_integer(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.equals_integer.mem.cost(args),
            self.equals_integer.cpu.cost(args),
        )
    }

    pub fn less_than_equals_integer(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.less_than_equals_integer.mem.cost(args),
            self.less_than_equals_integer.cpu.cost(args),
        )
    }

    pub fn multiply_integer(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.multiply_integer.mem.cost(args),
            self.multiply_integer.cpu.cost(args),
        )
    }

    pub fn divide_integer(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.divide_integer.mem.cost(args),
            self.divide_integer.cpu.cost(args),
        )
    }

    pub fn quotient_integer(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.quotient_integer.mem.cost(args),
            self.quotient_integer.cpu.cost(args),
        )
    }

    pub fn remainder_integer(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.remainder_integer.mem.cost(args),
            self.remainder_integer.cpu.cost(args),
        )
    }

    pub fn mod_integer(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.mod_integer.mem.cost(args),
            self.mod_integer.cpu.cost(args),
        )
    }

    pub fn less_than_integer(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.less_than_integer.mem.cost(args),
            self.less_than_integer.cpu.cost(args),
        )
    }

    pub fn append_byte_string(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.append_byte_string.mem.cost(args),
            self.append_byte_string.cpu.cost(args),
        )
    }

    pub fn equals_byte_string(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.equals_byte_string.mem.cost(args),
            self.equals_byte_string.cpu.cost(args),
        )
    }

    pub fn cons_byte_string(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.cons_byte_string.mem.cost(args),
            self.cons_byte_string.cpu.cost(args),
        )
    }

    pub fn slice_byte_string(&self, args: [i64; 3]) -> ExBudget {
        ExBudget::new(
            self.slice_byte_string.mem.cost(args),
            self.slice_byte_string.cpu.cost(args),
        )
    }

    pub fn length_of_byte_string(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.length_of_byte_string.mem.cost(args),
            self.length_of_byte_string.cpu.cost(args),
        )
    }

    pub fn index_byte_string(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.index_byte_string.mem.cost(args),
            self.index_byte_string.cpu.cost(args),
        )
    }

    pub fn less_than_byte_string(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.less_than_byte_string.mem.cost(args),
            self.less_than_byte_string.cpu.cost(args),
        )
    }

    pub fn less_than_equals_byte_string(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.less_than_equals_byte_string.mem.cost(args),
            self.less_than_equals_byte_string.cpu.cost(args),
        )
    }

    pub fn sha2_256(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.sha2_256.mem.cost(args), self.sha2_256.cpu.cost(args))
    }

    pub fn sha3_256(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.sha3_256.mem.cost(args), self.sha3_256.cpu.cost(args))
    }

    pub fn blake2b_256(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.blake2b_256.mem.cost(args),
            self.blake2b_256.cpu.cost(args),
        )
    }

    pub fn keccak_256(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.keccak_256.mem.cost(args),
            self.keccak_256.cpu.cost(args),
        )
    }

    pub fn blake2b_224(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.blake2b_224.mem.cost(args),
            self.blake2b_224.cpu.cost(args),
        )
    }

    pub fn verify_ed25519_signature(&self, args: [i64; 3]) -> ExBudget {
        ExBudget::new(
            self.verify_ed25519_signature.mem.cost(args),
            self.verify_ed25519_signature.cpu.cost(args),
        )
    }

    pub fn verify_ecdsa_secp256k1_signature(&self, args: [i64; 3]) -> ExBudget {
        ExBudget::new(
            self.verify_ecdsa_secp256k1_signature.mem.cost(args),
            self.verify_ecdsa_secp256k1_signature.cpu.cost(args),
        )
    }

    pub fn verify_schnorr_secp256k1_signature(&self, args: [i64; 3]) -> ExBudget {
        ExBudget::new(
            self.verify_schnorr_secp256k1_signature.mem.cost(args),
            self.verify_schnorr_secp256k1_signature.cpu.cost(args),
        )
    }

    pub fn append_string(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.append_string.mem.cost(args),
            self.append_string.cpu.cost(args),
        )
    }

    pub fn equals_string(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.equals_string.mem.cost(args),
            self.equals_string.cpu.cost(args),
        )
    }

    pub fn encode_utf8(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.encode_utf8.mem.cost(args),
            self.encode_utf8.cpu.cost(args),
        )
    }

    pub fn decode_utf8(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.decode_utf8.mem.cost(args),
            self.decode_utf8.cpu.cost(args),
        )
    }

    pub fn choose_unit(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.choose_unit.mem.cost(args),
            self.choose_unit.cpu.cost(args),
        )
    }

    pub fn trace(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(self.trace.mem.cost(args), self.trace.cpu.cost(args))
    }

    pub fn fst_pair(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.fst_pair.mem.cost(args), self.fst_pair.cpu.cost(args))
    }

    pub fn snd_pair(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.snd_pair.mem.cost(args), self.snd_pair.cpu.cost(args))
    }

    pub fn choose_list(&self, args: [i64; 3]) -> ExBudget {
        ExBudget::new(
            self.choose_list.mem.cost(args),
            self.choose_list.cpu.cost(args),
        )
    }

    pub fn mk_cons(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(self.mk_cons.mem.cost(args), self.mk_cons.cpu.cost(args))
    }

    pub fn head_list(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.head_list.mem.cost(args), self.head_list.cpu.cost(args))
    }

    pub fn tail_list(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.tail_list.mem.cost(args), self.tail_list.cpu.cost(args))
    }

    pub fn null_list(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.null_list.mem.cost(args), self.null_list.cpu.cost(args))
    }

    pub fn choose_data(&self, args: [i64; 6]) -> ExBudget {
        ExBudget::new(
            self.choose_data.mem.cost(args),
            self.choose_data.cpu.cost(args),
        )
    }

    pub fn constr_data(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.constr_data.mem.cost(args),
            self.constr_data.cpu.cost(args),
        )
    }

    pub fn map_data(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.map_data.mem.cost(args), self.map_data.cpu.cost(args))
    }

    pub fn list_data(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.list_data.mem.cost(args), self.list_data.cpu.cost(args))
    }

    pub fn i_data(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.i_data.mem.cost(args), self.i_data.cpu.cost(args))
    }

    pub fn b_data(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.b_data.mem.cost(args), self.b_data.cpu.cost(args))
    }

    pub fn un_constr_data(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.un_constr_data.mem.cost(args),
            self.un_constr_data.cpu.cost(args),
        )
    }

    pub fn un_map_data(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.un_map_data.mem.cost(args),
            self.un_map_data.cpu.cost(args),
        )
    }

    pub fn un_list_data(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.un_list_data.mem.cost(args),
            self.un_list_data.cpu.cost(args),
        )
    }

    pub fn un_i_data(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.un_i_data.mem.cost(args), self.un_i_data.cpu.cost(args))
    }

    pub fn un_b_data(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(self.un_b_data.mem.cost(args), self.un_b_data.cpu.cost(args))
    }

    pub fn equals_data(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.equals_data.mem.cost(args),
            self.equals_data.cpu.cost(args),
        )
    }

    pub fn mk_pair_data(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.mk_pair_data.mem.cost(args),
            self.mk_pair_data.cpu.cost(args),
        )
    }

    pub fn mk_nil_data(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.mk_nil_data.mem.cost(args),
            self.mk_nil_data.cpu.cost(args),
        )
    }

    pub fn mk_nil_pair_data(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.mk_nil_pair_data.mem.cost(args),
            self.mk_nil_pair_data.cpu.cost(args),
        )
    }

    pub fn bls12_381_g1_add(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g1_add.mem.cost(args),
            self.bls12_381_g1_add.cpu.cost(args),
        )
    }

    pub fn bls12_381_g1_neg(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g1_neg.mem.cost(args),
            self.bls12_381_g1_neg.cpu.cost(args),
        )
    }

    pub fn bls12_381_g1_scalar_mul(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g1_scalar_mul.mem.cost(args),
            self.bls12_381_g1_scalar_mul.cpu.cost(args),
        )
    }

    pub fn bls12_381_g1_equal(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g1_equal.mem.cost(args),
            self.bls12_381_g1_equal.cpu.cost(args),
        )
    }

    pub fn bls12_381_g1_compress(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g1_compress.mem.cost(args),
            self.bls12_381_g1_compress.cpu.cost(args),
        )
    }

    pub fn bls12_381_g1_uncompress(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g1_uncompress.mem.cost(args),
            self.bls12_381_g1_uncompress.cpu.cost(args),
        )
    }

    pub fn bls12_381_g1_hash_to_group(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g1_hash_to_group.mem.cost(args),
            self.bls12_381_g1_hash_to_group.cpu.cost(args),
        )
    }

    pub fn bls12_381_g2_add(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g2_add.mem.cost(args),
            self.bls12_381_g2_add.cpu.cost(args),
        )
    }

    pub fn bls12_381_g2_neg(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g2_neg.mem.cost(args),
            self.bls12_381_g2_neg.cpu.cost(args),
        )
    }

    pub fn bls12_381_g2_scalar_mul(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g2_scalar_mul.mem.cost(args),
            self.bls12_381_g2_scalar_mul.cpu.cost(args),
        )
    }

    pub fn bls12_381_g2_equal(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g2_equal.mem.cost(args),
            self.bls12_381_g2_equal.cpu.cost(args),
        )
    }

    pub fn bls12_381_g2_compress(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g2_compress.mem.cost(args),
            self.bls12_381_g2_compress.cpu.cost(args),
        )
    }

    pub fn bls12_381_g2_uncompress(&self, args: [i64; 1]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g2_uncompress.mem.cost(args),
            self.bls12_381_g2_uncompress.cpu.cost(args),
        )
    }

    pub fn bls12_381_g2_hash_to_group(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_g2_hash_to_group.mem.cost(args),
            self.bls12_381_g2_hash_to_group.cpu.cost(args),
        )
    }

    pub fn bls12_381_miller_loop(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_miller_loop.mem.cost(args),
            self.bls12_381_miller_loop.cpu.cost(args),
        )
    }

    pub fn bls12_381_mul_ml_result(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_mul_ml_result.mem.cost(args),
            self.bls12_381_mul_ml_result.cpu.cost(args),
        )
    }

    pub fn bls12_381_final_verify(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.bls12_381_final_verify.mem.cost(args),
            self.bls12_381_final_verify.cpu.cost(args),
        )
    }

    pub fn integer_to_byte_string(&self, args: [i64; 3]) -> ExBudget {
        ExBudget::new(
            self.integer_to_byte_string.mem.cost(args),
            self.integer_to_byte_string.cpu.cost(args),
        )
    }

    pub fn byte_string_to_integer(&self, args: [i64; 2]) -> ExBudget {
        ExBudget::new(
            self.byte_string_to_integer.mem.cost(args),
            self.byte_string_to_integer.cpu.cost(args),
        )
    }

    pub fn v3() -> Self {
        Self {
            add_integer: TwoArgumentsCosting::new(
                TwoArgumentsCosting::max_size(1, 1),
                TwoArgumentsCosting::max_size(100788, 420),
            ),
            subtract_integer: TwoArgumentsCosting::new(
                TwoArgumentsCosting::max_size(1, 1),
                TwoArgumentsCosting::max_size(100788, 420),
            ),

            multiply_integer: TwoArgumentsCosting::new(
                TwoArgumentsCosting::added_sizes(0, 1),
                TwoArgumentsCosting::multiplied_sizes(90434, 519),
            ),
            divide_integer: TwoArgumentsCosting::new(
                TwoArgumentsCosting::subtracted_sizes(0, 1, 1),
                TwoArgumentsCosting::const_above_diagonal_into_quadratic_x_and_y(
                    85848, 85848, 123203, 7305, -900, 1716, 549, 57,
                ),
            ),
            quotient_integer: TwoArgumentsCosting::new(
                TwoArgumentsCosting::subtracted_sizes(0, 1, 1),
                TwoArgumentsCosting::const_above_diagonal_into_quadratic_x_and_y(
                    85848, 85848, 123203, 7305, -900, 1716, 549, 57,
                ),
            ),
            remainder_integer: TwoArgumentsCosting::new(
                TwoArgumentsCosting::linear_in_y(0, 1),
                TwoArgumentsCosting::const_above_diagonal_into_quadratic_x_and_y(
                    85848, 85848, 123203, 7305, -900, 1716, 549, 57,
                ),
            ),
            mod_integer: TwoArgumentsCosting::new(
                TwoArgumentsCosting::linear_in_y(0, 1),
                TwoArgumentsCosting::const_above_diagonal_into_quadratic_x_and_y(
                    85848, 85848, 123203, 7305, -900, 1716, 549, 57,
                ),
            ),
            equals_integer: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(1),
                TwoArgumentsCosting::min_size(51775, 558),
            ),
            less_than_integer: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(1),
                TwoArgumentsCosting::min_size(44749, 541),
            ),
            less_than_equals_integer: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(1),
                TwoArgumentsCosting::min_size(43285, 552),
            ),
            append_byte_string: TwoArgumentsCosting::new(
                TwoArgumentsCosting::added_sizes(0, 1),
                TwoArgumentsCosting::added_sizes(1000, 173),
            ),
            cons_byte_string: TwoArgumentsCosting::new(
                TwoArgumentsCosting::added_sizes(0, 1),
                TwoArgumentsCosting::linear_in_y(72010, 178),
            ),
            slice_byte_string: ThreeArgumentsCosting::new(
                ThreeArgumentsCosting::linear_in_z(4, 0),
                ThreeArgumentsCosting::linear_in_z(20467, 1),
            ),
            length_of_byte_string: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(10),
                OneArgumentCosting::constant_cost(22100),
            ),
            index_byte_string: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(4),
                TwoArgumentsCosting::constant_cost(13169),
            ),
            equals_byte_string: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(1),
                TwoArgumentsCosting::linear_on_diagonal(24548, 29498, 38),
            ),
            less_than_byte_string: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(1),
                TwoArgumentsCosting::min_size(28999, 74),
            ),
            less_than_equals_byte_string: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(1),
                TwoArgumentsCosting::min_size(28999, 74),
            ),
            sha2_256: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(4),
                OneArgumentCosting::linear_cost(270652, 22588),
            ),
            sha3_256: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(4),
                OneArgumentCosting::linear_cost(1457325, 64566),
            ),
            blake2b_256: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(4),
                OneArgumentCosting::linear_cost(201305, 8356),
            ),
            verify_ed25519_signature: ThreeArgumentsCosting::new(
                ThreeArgumentsCosting::constant_cost(10),
                ThreeArgumentsCosting::linear_in_y(53384111, 14333),
            ),
            verify_ecdsa_secp256k1_signature: ThreeArgumentsCosting::new(
                ThreeArgumentsCosting::constant_cost(10),
                ThreeArgumentsCosting::constant_cost(43053543),
            ),
            verify_schnorr_secp256k1_signature: ThreeArgumentsCosting::new(
                ThreeArgumentsCosting::constant_cost(10),
                ThreeArgumentsCosting::linear_in_y(43574283, 26308),
            ),
            append_string: TwoArgumentsCosting::new(
                TwoArgumentsCosting::added_sizes(4, 1),
                TwoArgumentsCosting::added_sizes(1000, 59957),
            ),
            equals_string: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(1),
                TwoArgumentsCosting::linear_on_diagonal(39184, 1000, 60594),
            ),
            encode_utf8: OneArgumentCosting::new(
                OneArgumentCosting::linear_cost(4, 2),
                OneArgumentCosting::linear_cost(1000, 42921),
            ),
            decode_utf8: OneArgumentCosting::new(
                OneArgumentCosting::linear_cost(4, 2),
                OneArgumentCosting::linear_cost(91189, 769),
            ),
            if_then_else: ThreeArgumentsCosting::new(
                ThreeArgumentsCosting::constant_cost(1),
                ThreeArgumentsCosting::constant_cost(76049),
            ),
            choose_unit: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(4),
                TwoArgumentsCosting::constant_cost(61462),
            ),
            trace: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(32),
                TwoArgumentsCosting::constant_cost(59498),
            ),
            fst_pair: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(141895),
            ),
            snd_pair: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(141992),
            ),
            choose_list: ThreeArgumentsCosting::new(
                ThreeArgumentsCosting::constant_cost(32),
                ThreeArgumentsCosting::constant_cost(132994),
            ),
            mk_cons: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(32),
                TwoArgumentsCosting::constant_cost(72362),
            ),
            head_list: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(83150),
            ),
            tail_list: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(81663),
            ),
            null_list: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(74433),
            ),
            choose_data: SixArgumentsCosting::new(
                SixArgumentsCosting::constant_cost(32),
                SixArgumentsCosting::constant_cost(94375),
            ),
            constr_data: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(32),
                TwoArgumentsCosting::constant_cost(22151),
            ),
            map_data: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(68246),
            ),
            list_data: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(33852),
            ),
            i_data: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(15299),
            ),
            b_data: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(11183),
            ),
            un_constr_data: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(24588),
            ),
            un_map_data: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(24623),
            ),
            un_list_data: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(25933),
            ),
            un_i_data: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(20744),
            ),
            un_b_data: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(20142),
            ),
            equals_data: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(1),
                TwoArgumentsCosting::min_size(898148, 27279),
            ),
            mk_pair_data: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(32),
                TwoArgumentsCosting::constant_cost(11546),
            ),
            mk_nil_data: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(7243),
            ),
            mk_nil_pair_data: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(32),
                OneArgumentCosting::constant_cost(7391),
            ),

            serialise_data: OneArgumentCosting::new(
                OneArgumentCosting::linear_cost(0, 2),
                OneArgumentCosting::linear_cost(955506, 213312),
            ),
            blake2b_224: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(4),
                OneArgumentCosting::linear_cost(207616, 8310),
            ),
            keccak_256: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(4),
                OneArgumentCosting::linear_cost(2261318, 64571),
            ),
            bls12_381_g1_add: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(18),
                TwoArgumentsCosting::constant_cost(962335),
            ),
            bls12_381_g1_neg: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(18),
                OneArgumentCosting::constant_cost(267929),
            ),
            bls12_381_g1_scalar_mul: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(18),
                TwoArgumentsCosting::linear_in_x(76433006, 8868),
            ),
            bls12_381_g1_equal: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(1),
                TwoArgumentsCosting::constant_cost(442008),
            ),
            bls12_381_g1_compress: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(6),
                OneArgumentCosting::constant_cost(2780678),
            ),
            bls12_381_g1_uncompress: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(18),
                OneArgumentCosting::constant_cost(52948122),
            ),
            bls12_381_g1_hash_to_group: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(18),
                TwoArgumentsCosting::linear_in_x(52538055, 3756),
            ),
            bls12_381_g2_add: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(36),
                TwoArgumentsCosting::constant_cost(1995836),
            ),
            bls12_381_g2_neg: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(36),
                OneArgumentCosting::constant_cost(284546),
            ),
            bls12_381_g2_scalar_mul: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(36),
                TwoArgumentsCosting::linear_in_x(158_221_314, 26_549),
            ),
            bls12_381_g2_equal: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(1),
                TwoArgumentsCosting::constant_cost(901_022),
            ),
            bls12_381_g2_compress: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(12),
                OneArgumentCosting::constant_cost(3_227_919),
            ),
            bls12_381_g2_uncompress: OneArgumentCosting::new(
                OneArgumentCosting::constant_cost(36),
                OneArgumentCosting::constant_cost(74_698_472),
            ),
            bls12_381_g2_hash_to_group: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(36),
                TwoArgumentsCosting::linear_in_x(166_917_843, 4_307),
            ),
            bls12_381_miller_loop: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(72),
                TwoArgumentsCosting::constant_cost(254006273),
            ),
            bls12_381_mul_ml_result: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(72),
                TwoArgumentsCosting::constant_cost(2174038),
            ),
            bls12_381_final_verify: TwoArgumentsCosting::new(
                TwoArgumentsCosting::constant_cost(1),
                TwoArgumentsCosting::constant_cost(333849714),
            ),
            integer_to_byte_string: ThreeArgumentsCosting::new(
                ThreeArgumentsCosting::literal_in_y_or_linear_in_z(0, 1),
                ThreeArgumentsCosting::quadratic_in_z(1293828, 28716, 63),
            ),
            byte_string_to_integer: TwoArgumentsCosting::new(
                TwoArgumentsCosting::linear_in_y(0, 1),
                TwoArgumentsCosting::quadratic_in_y(1006041, 43623, 251),
            ),
        }
    }
}
