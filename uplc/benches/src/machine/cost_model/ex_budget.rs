#[derive(Debug, Copy, Clone, PartialEq)]
pub struct ExBudget {
    pub mem: i64,
    pub cpu: i64,
}

impl Default for ExBudget {
    fn default() -> Self {
        Self::machine()
    }
}

impl ExBudget {
    pub fn new(mem: i64, cpu: i64) -> Self {
        ExBudget { mem, cpu }
    }

    pub fn max() -> Self {
        Self::machine_max()
    }

    pub fn occurrences(&mut self, n: i64) {
        self.mem *= n;
        self.cpu *= n;
    }

    pub fn machine() -> Self {
        ExBudget {
            mem: 14_000_000,
            cpu: 10_000_000_000,
        }
    }

    pub fn machine_max() -> Self {
        ExBudget {
            mem: 14_000_000_000_000,
            cpu: 10_000_000_000_000_000,
        }
    }

    pub fn start_up() -> Self {
        ExBudget { mem: 100, cpu: 100 }
    }

    pub fn var() -> Self {
        ExBudget {
            mem: 100,
            cpu: 16000,
        }
    }

    pub fn constant() -> Self {
        ExBudget {
            mem: 100,
            cpu: 16000,
        }
    }

    pub fn lambda() -> Self {
        ExBudget {
            mem: 100,
            cpu: 16000,
        }
    }

    pub fn delay() -> Self {
        ExBudget {
            mem: 100,
            cpu: 16000,
        }
    }

    pub fn force() -> Self {
        ExBudget {
            mem: 100,
            cpu: 16000,
        }
    }

    pub fn apply() -> Self {
        ExBudget {
            mem: 100,
            cpu: 16000,
        }
    }

    pub fn builtin() -> Self {
        ExBudget {
            mem: 100,
            cpu: 16000,
        }
    }

    pub fn constr() -> Self {
        ExBudget {
            mem: 100,
            cpu: 16000,
        }
    }

    pub fn case() -> Self {
        ExBudget {
            mem: 100,
            cpu: 16000,
        }
    }
}

impl std::ops::Sub for ExBudget {
    type Output = ExBudget;

    fn sub(self, rhs: Self) -> Self::Output {
        ExBudget {
            mem: self.mem - rhs.mem,
            cpu: self.cpu - rhs.cpu,
        }
    }
}
