pub mod builtin_costs;
mod costing;
pub mod ex_budget;
mod machine_costs;
mod value;

pub use value::*;

#[derive(Default, Debug, PartialEq)]
pub struct CostModel {
    pub machine_costs: machine_costs::MachineCosts,
    pub builtin_costs: builtin_costs::BuiltinCosts,
}

#[repr(usize)]
pub enum StepKind {
    Constant = 0,
    Var = 1,
    Lambda = 2,
    Apply = 3,
    Delay = 4,
    Force = 5,
    Builtin = 6,
    Constr = 7,
    Case = 8,
}
