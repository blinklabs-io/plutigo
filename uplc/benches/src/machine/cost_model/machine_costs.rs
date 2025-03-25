use crate::machine::ExBudget;

#[derive(Debug, PartialEq)]
pub struct MachineCosts([ExBudget; 9]);

impl Default for MachineCosts {
    fn default() -> Self {
        Self::new()
    }
}

impl MachineCosts {
    pub fn new() -> Self {
        MachineCosts([
            ExBudget::constant(),
            ExBudget::var(),
            ExBudget::lambda(),
            ExBudget::apply(),
            ExBudget::delay(),
            ExBudget::force(),
            ExBudget::builtin(),
            ExBudget::constr(),
            ExBudget::case(),
        ])
    }

    pub fn get(&self, index: usize) -> ExBudget {
        self.0[index]
    }
}
