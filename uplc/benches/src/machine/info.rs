use super::ExBudget;

#[derive(Debug)]
pub struct MachineInfo {
    pub consumed_budget: ExBudget,
    pub logs: Vec<String>,
}
