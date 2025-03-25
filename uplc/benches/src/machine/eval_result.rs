use crate::{binder::Eval, term::Term};

use super::{info::MachineInfo, MachineError};

#[derive(Debug)]
pub struct EvalResult<'a, V>
where
    V: Eval<'a>,
{
    pub term: Result<&'a Term<'a, V>, MachineError<'a, V>>,
    pub info: MachineInfo,
}
