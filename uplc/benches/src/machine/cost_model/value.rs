use rug::integer::BorrowInteger;

use crate::{
    binder::Eval,
    constant::{Constant, Integer},
    data::PlutusData,
    machine::value::Value,
};

pub const UNIT_EX_MEM: i64 = 1;
pub const BOOL_EX_MEM: i64 = 1;

pub fn integer_ex_mem(i: &Integer) -> i64 {
    if *i == 0 {
        1
    } else {
        (integer_log2(i.as_abs()) / 64) + 1
    }
}

pub fn integer_log2(i: BorrowInteger<'_>) -> i64 {
    if i.is_zero() {
        return 0;
    }

    (i.significant_bits() - 1) as i64
}

pub fn integer_log2_x(i: &Integer) -> i64 {
    if i.is_zero() {
        return 0;
    }

    (i.significant_bits() - 1) as i64
}

pub fn byte_string_ex_mem(b: &[u8]) -> i64 {
    if b.is_empty() {
        1
    } else {
        ((b.len() as i64 - 1) / 8) + 1
    }
}

pub fn string_ex_mem(s: &str) -> i64 {
    s.chars().count() as i64
}

pub fn pair_ex_mem(l: &Constant, r: &Constant) -> i64 {
    constant_ex_mem(l) + constant_ex_mem(r)
}

pub fn proto_list_ex_mem(items: &[&Constant]) -> i64 {
    items
        .iter()
        .fold(0, |acc, constant| acc + constant_ex_mem(constant))
}

pub fn value_ex_mem<'a, V>(v: &'a Value<'a, V>) -> i64
where
    V: Eval<'a>,
{
    match v {
        Value::Con(c) => constant_ex_mem(c),
        Value::Lambda { .. } => 1,
        Value::Builtin(_) => 1,
        Value::Delay(_, _) => 1,
        Value::Constr(_, _) => 1,
    }
}

pub fn constant_ex_mem(c: &Constant) -> i64 {
    match c {
        Constant::Integer(i) => integer_ex_mem(i),
        Constant::ByteString(b) => byte_string_ex_mem(b),
        Constant::String(s) => string_ex_mem(s),
        Constant::Unit => UNIT_EX_MEM,
        Constant::Boolean(_) => BOOL_EX_MEM,
        Constant::ProtoList(_, items) => proto_list_ex_mem(items),
        Constant::ProtoPair(_, _, l, r) => pair_ex_mem(l, r),
        Constant::Data(d) => data_ex_mem(d),
        Constant::Bls12_381G1Element(_) => g1_element_ex_mem(),
        Constant::Bls12_381G2Element(_) => g2_element_ex_mem(),
        Constant::Bls12_381MlResult(_) => ml_result_ex_mem(),
    }
}

pub fn data_ex_mem(d: &PlutusData) -> i64 {
    match d {
        PlutusData::Constr { fields, .. } => data_list_ex_mem(fields),
        PlutusData::Map(items) => data_map_ex_mem(items),
        PlutusData::Integer(i) => data_integer_ex_mem(i),
        PlutusData::ByteString(b) => data_byte_string_ex_mem(b),
        PlutusData::List(items) => data_list_ex_mem(items),
    }
}

pub fn data_integer_ex_mem(i: &Integer) -> i64 {
    4 + integer_ex_mem(i)
}

pub fn data_byte_string_ex_mem(b: &[u8]) -> i64 {
    4 + byte_string_ex_mem(b)
}

pub fn data_list_ex_mem(items: &[&PlutusData]) -> i64 {
    4 + items.iter().fold(0, |acc, d| acc + data_ex_mem(d))
}

pub fn data_map_ex_mem(items: &[(&PlutusData, &PlutusData)]) -> i64 {
    4 + items
        .iter()
        .fold(0, |acc, (k, v)| acc + data_ex_mem(k) + data_ex_mem(v))
}

pub fn g1_element_ex_mem() -> i64 {
    size_of::<blst::blst_p1>() as i64 / 8
}

pub fn g2_element_ex_mem() -> i64 {
    size_of::<blst::blst_p2>() as i64 / 8
}

pub fn ml_result_ex_mem() -> i64 {
    size_of::<blst::blst_fp12>() as i64 / 8
}

#[cfg(test)]
mod tests {
    use std::str::FromStr;

    use super::integer_log2;

    #[test]
    fn integer_log2_oracle() {
        // Values come from the Haskell implementation
        assert_eq!(integer_log2(rug::Integer::from(0).as_abs()), 0);
        assert_eq!(integer_log2(rug::Integer::from(1).as_abs()), 0);
        assert_eq!(integer_log2(rug::Integer::from(42).as_abs()), 5);

        assert_eq!(
            integer_log2(
                rug::Integer::from_str("18446744073709551615")
                    .unwrap()
                    .as_abs()
            ),
            63
        );
        assert_eq!(
            integer_log2(
                rug::Integer::from_str("999999999999999999999999999999")
                    .unwrap()
                    .as_abs()
            ),
            99
        );
        assert_eq!(
            integer_log2(
                rug::Integer::from_str("170141183460469231731687303715884105726")
                    .unwrap()
                    .as_abs()
            ),
            126
        );
        assert_eq!(
            integer_log2(
                rug::Integer::from_str("170141183460469231731687303715884105727")
                    .unwrap()
                    .as_abs()
            ),
            126
        );
        assert_eq!(
            integer_log2(
                rug::Integer::from_str("170141183460469231731687303715884105728")
                    .unwrap()
                    .as_abs()
            ),
            127
        );
        assert_eq!(
            integer_log2(
                rug::Integer::from_str("340282366920938463463374607431768211458")
                    .unwrap()
                    .as_abs()
            ),
            128
        );
        assert_eq!(
            integer_log2(
                rug::Integer::from_str("999999999999999999999999999999999999999999")
                    .unwrap()
                    .as_abs()
            ),
            139
        );
        assert_eq!(
            integer_log2(
                rug::Integer::from_str("999999999999999999999999999999999999999999999999999999999999999999999999999999999999")
                    .unwrap()
                    .as_abs()
            ),
            279
        );
    }
}
