use bumpalo::Bump;

use super::Binder;

#[derive(Debug)]
pub struct Name<'a> {
    text: &'a str,
    unique: usize,
}

impl<'a> Name<'a> {
    pub fn new(arena: &'a Bump, text: &'a str, unique: usize) -> &'a Self {
        arena.alloc(Name { text, unique })
    }
}

impl<'a> Binder<'a> for Name<'a> {
    fn var_encode(&self, e: &mut crate::flat::Encoder) -> Result<(), crate::flat::FlatEncodeError> {
        e.utf8(self.text)?;
        e.word(self.unique);

        Ok(())
    }

    fn var_decode(
        arena: &'a bumpalo::Bump,
        d: &mut crate::flat::Decoder,
    ) -> Result<&'a Self, crate::flat::FlatDecodeError> {
        let text = d.utf8(arena)?;
        let index = d.word()?;

        let nd = Name::new(arena, text, index);

        Ok(nd)
    }

    fn parameter_encode(
        &self,
        e: &mut crate::flat::Encoder,
    ) -> Result<(), crate::flat::FlatEncodeError> {
        self.var_encode(e)
    }

    fn parameter_decode(
        arena: &'a bumpalo::Bump,
        d: &mut crate::flat::Decoder,
    ) -> Result<&'a Self, crate::flat::FlatDecodeError> {
        Self::var_decode(arena, d)
    }
}
