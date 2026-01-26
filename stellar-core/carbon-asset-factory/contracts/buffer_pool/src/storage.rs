use soroban_sdk::{contracttype, Address, Env, String, Symbol};

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CustodyRecord {
    pub token_id: u32,
    pub deposited_at: u64,
    pub depositor: Address,
    pub project_id: String,
}

const ADMIN: Symbol = Symbol::short("admin");
const GOVERNANCE: Symbol = Symbol::short("gov");
const CARBON_CONTRACT: Symbol = Symbol::short("carbon");
const REPLENISH_PCT: Symbol = Symbol::short("rep_pct");
const TVL: Symbol = Symbol::short("tvl");
pub const CUSTODY: Symbol = Symbol::short("custody");

pub fn get_admin(env: &Env) -> Address {
    env.storage().instance().get(&ADMIN).unwrap()
}

pub fn set_admin(env: &Env, admin: &Address) {
    env.storage().instance().set(&ADMIN, admin);
}

pub fn get_governance(env: &Env) -> Address {
    env.storage().instance().get(&GOVERNANCE).unwrap()
}

pub fn set_governance(env: &Env, governance: &Address) {
    env.storage().instance().set(&GOVERNANCE, governance);
}

pub fn get_carbon_asset_contract(env: &Env) -> Address {
    env.storage().instance().get(&CARBON_CONTRACT).unwrap()
}

pub fn set_carbon_asset_contract(env: &Env, contract: &Address) {
    env.storage().instance().set(&CARBON_CONTRACT, contract);
}

pub fn get_replenishment_percentage(env: &Env) -> i64 {
    env.storage().instance().get(&REPLENISH_PCT).unwrap_or(500)
}

pub fn set_replenishment_percentage(env: &Env, percentage: i64) {
    env.storage().instance().set(&REPLENISH_PCT, &percentage);
}

pub fn get_total_value_locked(env: &Env) -> i128 {
    env.storage().instance().get(&TVL).unwrap_or(0)
}

pub fn set_total_value_locked(env: &Env, tvl: i128) {
    env.storage().instance().set(&TVL, &tvl);
}

pub fn get_custody_record(env: &Env, token_id: u32) -> Option<CustodyRecord> {
    env.storage().persistent().get(&(CUSTODY, token_id))
}

pub fn set_custody_record(env: &Env, token_id: u32, record: &CustodyRecord) {
    env.storage().persistent().set(&(CUSTODY, token_id), record);
}

pub fn has_custody_record(env: &Env, token_id: u32) -> bool {
    env.storage().persistent().has(&(CUSTODY, token_id))
}

