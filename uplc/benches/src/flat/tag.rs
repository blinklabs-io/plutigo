// Widths
pub const TERM_TAG_WIDTH: usize = 4;
pub const CONST_TAG_WIDTH: usize = 4;
pub const BUILTIN_TAG_WIDTH: usize = 7;

// Term Tags
pub const VAR: u8 = 0;
pub const DELAY: u8 = 1;
pub const LAMBDA: u8 = 2;
pub const APPLY: u8 = 3;
pub const CONSTANT: u8 = 4;
pub const FORCE: u8 = 5;
pub const ERROR: u8 = 6;
pub const BUILTIN: u8 = 7;
pub const CONSTR: u8 = 8;
pub const CASE: u8 = 9;

// Constant Tags
pub const INTEGER: u8 = 0;
pub const BYTE_STRING: u8 = 1;
pub const STRING: u8 = 2;
pub const UNIT: u8 = 3;
pub const BOOL: u8 = 4;
pub const DATA: u8 = 8;
pub const PROTO_LIST_ONE: u8 = 7;
pub const PROTO_LIST_TWO: u8 = 5;
pub const PROTO_PAIR_ONE: u8 = 7;
pub const PROTO_PAIR_TWO: u8 = 7;
pub const PROTO_PAIR_THREE: u8 = 6;
