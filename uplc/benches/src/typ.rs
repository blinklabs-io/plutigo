use bumpalo::Bump;

#[derive(Debug, PartialEq)]
pub enum Type<'a> {
    Bool,
    Integer,
    String,
    ByteString,
    Unit,
    List(&'a Type<'a>),
    Pair(&'a Type<'a>, &'a Type<'a>),
    Data,
    Bls12_381G1Element,
    Bls12_381G2Element,
    Bls12_381MlResult,
}

impl<'a> Type<'a> {
    pub fn integer(arena: &'a Bump) -> &'a Type<'a> {
        arena.alloc(Type::Integer)
    }

    pub fn bool(arena: &'a Bump) -> &'a Type<'a> {
        arena.alloc(Type::Bool)
    }

    pub fn string(arena: &'a Bump) -> &'a Type<'a> {
        arena.alloc(Type::String)
    }

    pub fn byte_string(arena: &'a Bump) -> &'a Type<'a> {
        arena.alloc(Type::ByteString)
    }

    pub fn unit(arena: &'a Bump) -> &'a Type<'a> {
        arena.alloc(Type::Unit)
    }

    pub fn data(arena: &'a Bump) -> &'a Type<'a> {
        arena.alloc(Type::Data)
    }

    pub fn list(arena: &'a Bump, inner: &'a Type<'a>) -> &'a Type<'a> {
        arena.alloc(Type::List(inner))
    }

    pub fn pair(arena: &'a Bump, fst: &'a Type<'a>, snd: &'a Type<'a>) -> &'a Type<'a> {
        arena.alloc(Type::Pair(fst, snd))
    }

    pub fn g1(arena: &'a Bump) -> &'a Type<'a> {
        arena.alloc(Type::Bls12_381G1Element)
    }

    pub fn g2(arena: &'a Bump) -> &'a Type<'a> {
        arena.alloc(Type::Bls12_381G2Element)
    }

    pub fn ml_result(arena: &'a Bump) -> &'a Type<'a> {
        arena.alloc(Type::Bls12_381MlResult)
    }
}
