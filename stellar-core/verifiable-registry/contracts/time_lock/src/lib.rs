#![no_std]

use soroban_sdk::{
    contract, contractclient, contracterror, contractimpl, contracttype, panic_with_error, Address,
    Env, Map, Option, Symbol, Vec,
};

#[contractclient(name = "CarbonAssetClient")]
pub trait CarbonAsset {
    fn transfer_from(env: Env, from: Address, to: Address, token_id: u32);
    fn owner_of(env: Env, token_id: u32) -> Address;
    fn get_vintage_unlock_timestamp(env: Env, token_id: u32) -> u64;
}

#[contracterror]
#[derive(Copy, Clone, Eq, PartialEq, Debug)]
pub enum TimeLockError {
    AlreadyInitialized = 1,
    NotInitialized = 2,
    AlreadyLocked = 3,
    NotLocked = 4,
    VintageCheckMissing = 5,
    VintageMismatch = 6,
}

#[contracttype]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LockRecord {
    pub token_id: u32,
    pub owner: Address,
    pub unlock_timestamp: u64,
    pub deposited_at: u64,
}

#[contracttype]
enum DataKey {
    Admin,
    CarbonAssetContract,
    ValidateVintage,
    VintageCheckContract,
    LockRecords,
}

const EVENT_LOCKED: Symbol = Symbol::short("locked");
const EVENT_RELEASED: Symbol = Symbol::short("released");
const EVENT_FORCE_RELEASED: Symbol = Symbol::short("force_rls");

#[contract]
pub struct TimeLockContract;

#[contractimpl]
impl TimeLockContract {
    pub fn initialize(
        env: Env,
        admin: Address,
        carbon_asset_contract: Address,
        validate_vintage: bool,
        vintage_check_contract: Option<Address>,
    ) {
        if env.storage().persistent().has(&DataKey::Admin) {
            panic_with_error!(env, TimeLockError::AlreadyInitialized);
        }

        admin.require_auth();
        env.storage().persistent().set(&DataKey::Admin, &admin);
        env.storage()
            .persistent()
            .set(&DataKey::CarbonAssetContract, &carbon_asset_contract);
        env.storage()
            .persistent()
            .set(&DataKey::ValidateVintage, &validate_vintage);
        env.storage()
            .persistent()
            .set(&DataKey::VintageCheckContract, &vintage_check_contract);
        env.storage()
            .persistent()
            .set(&DataKey::LockRecords, &Map::<u32, LockRecord>::new(&env));
    }

    pub fn lock_credit(env: Env, token_id: u32, unlock_timestamp: u64) {
        let carbon_asset = get_carbon_asset_contract(&env);
        let invoker = env.invoker();

        if invoker != carbon_asset {
            invoker.require_auth();
        }

        if should_validate_vintage(&env) {
            validate_vintage_unlock(&env, token_id, unlock_timestamp);
        }

        let mut lock_records = read_lock_records(&env);
        if lock_records.contains_key(token_id) {
            panic_with_error!(env, TimeLockError::AlreadyLocked);
        }

        let owner = if invoker == carbon_asset {
            CarbonAssetClient::new(&env, &carbon_asset).owner_of(&token_id)
        } else {
            invoker
        };

        CarbonAssetClient::new(&env, &carbon_asset).transfer_from(
            &owner,
            &env.current_contract_address(),
            &token_id,
        );

        let record = LockRecord {
            token_id,
            owner,
            unlock_timestamp,
            deposited_at: env.ledger().timestamp(),
        };

        lock_records.set(token_id, record.clone());
        write_lock_records(&env, lock_records);

        env.events().publish((EVENT_LOCKED, token_id), record);
    }

    pub fn release_if_eligible(env: Env, token_id: u32) {
        let mut lock_records = read_lock_records(&env);
        let record = match lock_records.get(token_id) {
            Option::Some(value) => value,
            Option::None => return,
        };

        if env.ledger().timestamp() < record.unlock_timestamp {
            return;
        }

        let carbon_asset = get_carbon_asset_contract(&env);
        CarbonAssetClient::new(&env, &carbon_asset).transfer_from(
            &env.current_contract_address(),
            &record.owner,
            &token_id,
        );

        lock_records.remove(token_id);
        write_lock_records(&env, lock_records);

        env.events().publish((EVENT_RELEASED, token_id), record);
    }

    pub fn batch_release(env: Env, token_ids: Vec<u32>) {
        for token_id in token_ids.iter() {
            Self::release_if_eligible(env.clone(), token_id);
        }
    }

    pub fn force_release(env: Env, token_id: u32) {
        let admin = get_admin(&env);
        admin.require_auth();

        let mut lock_records = read_lock_records(&env);
        let record = match lock_records.get(token_id) {
            Option::Some(value) => value,
            Option::None => panic_with_error!(env, TimeLockError::NotLocked),
        };

        let carbon_asset = get_carbon_asset_contract(&env);
        CarbonAssetClient::new(&env, &carbon_asset).transfer_from(
            &env.current_contract_address(),
            &record.owner,
            &token_id,
        );

        lock_records.remove(token_id);
        write_lock_records(&env, lock_records);

        env.events().publish((EVENT_FORCE_RELEASED, token_id), record);
    }

    pub fn get_lock_status(env: Env, token_id: u32) -> Option<LockRecord> {
        read_lock_records(&env).get(token_id)
    }

    pub fn get_tokens_locked_until(env: Env, timestamp: u64) -> Vec<u32> {
        let lock_records = read_lock_records(&env);
        let mut result = Vec::new(&env);
        for (token_id, record) in lock_records.iter() {
            if record.unlock_timestamp > timestamp {
                result.push_back(token_id);
            }
        }
        result
    }

    pub fn get_admin(env: Env) -> Address {
        get_admin(&env)
    }

    pub fn get_carbon_asset_contract(env: Env) -> Address {
        get_carbon_asset_contract(&env)
    }
}

fn get_admin(env: &Env) -> Address {
    env.storage()
        .persistent()
        .get(&DataKey::Admin)
        .unwrap_or_else(|| panic_with_error!(env, TimeLockError::NotInitialized))
}

fn get_carbon_asset_contract(env: &Env) -> Address {
    env.storage()
        .persistent()
        .get(&DataKey::CarbonAssetContract)
        .unwrap_or_else(|| panic_with_error!(env, TimeLockError::NotInitialized))
}

fn should_validate_vintage(env: &Env) -> bool {
    env.storage()
        .persistent()
        .get(&DataKey::ValidateVintage)
        .unwrap_or(false)
}

fn get_vintage_check_contract(env: &Env) -> Option<Address> {
    env.storage()
        .persistent()
        .get(&DataKey::VintageCheckContract)
        .unwrap_or(Option::None)
}

fn validate_vintage_unlock(env: &Env, token_id: u32, unlock_timestamp: u64) {
    let check_contract = match get_vintage_check_contract(env) {
        Option::Some(addr) => addr,
        Option::None => panic_with_error!(env, TimeLockError::VintageCheckMissing),
    };

    let expected_unlock =
        CarbonAssetClient::new(env, &check_contract).get_vintage_unlock_timestamp(&token_id);
    if unlock_timestamp < expected_unlock {
        panic_with_error!(env, TimeLockError::VintageMismatch);
    }
}

fn read_lock_records(env: &Env) -> Map<u32, LockRecord> {
    env.storage()
        .persistent()
        .get(&DataKey::LockRecords)
        .unwrap_or_else(|| Map::<u32, LockRecord>::new(env))
}

fn write_lock_records(env: &Env, records: Map<u32, LockRecord>) {
    env.storage().persistent().set(&DataKey::LockRecords, &records);
}
