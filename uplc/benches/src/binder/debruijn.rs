use bumpalo::Bump;

use super::{Binder, Eval};

#[derive(Debug, Eq, PartialEq)]
pub struct DeBruijn(usize);

impl DeBruijn {
    pub fn new(arena: &Bump, i: usize) -> &Self {
        arena.alloc(DeBruijn(i))
    }

    pub fn zero(arena: &Bump) -> &Self {
        arena.alloc(DeBruijn(0))
    }
}

impl<'a> Binder<'a> for DeBruijn {
    fn var_encode(&self, e: &mut crate::flat::Encoder) -> Result<(), crate::flat::FlatEncodeError> {
        e.word(self.0);

        Ok(())
    }

    fn var_decode(
        arena: &'a bumpalo::Bump,
        d: &mut crate::flat::Decoder,
    ) -> Result<&'a Self, crate::flat::FlatDecodeError> {
        let i = d.word()?;

        let d = DeBruijn::new(arena, i);

        Ok(d)
    }

    fn parameter_encode(
        &self,
        _e: &mut crate::flat::Encoder,
    ) -> Result<(), crate::flat::FlatEncodeError> {
        Ok(())
    }

    fn parameter_decode(
        arena: &'a bumpalo::Bump,
        _d: &mut crate::flat::Decoder,
    ) -> Result<&'a Self, crate::flat::FlatDecodeError> {
        let d = DeBruijn::new(arena, 0);

        Ok(d)
    }
}

impl Eval<'_> for DeBruijn {
    fn index(&self) -> usize {
        self.0
    }
}
