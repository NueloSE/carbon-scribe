#![no_std]

mod errors;
mod storage;

use errors::Error;
use soroban_sdk::{contract, contractimpl, Address, Env, String};
use storage::*;

#[contract]
pub struct BufferPoolContract;

#[contractimpl]
impl BufferPoolContract {
    pub fn initialize(
        env: Env,
        admin: Address,
        governance: Address,
        carbon_asset_contract: Address,
        initial_percentage: i64,
    ) -> Result<(), Error> {
        if env.storage().instance().has(&soroban_sdk::Symbol::short("admin")) {
            return Err(Error::AlreadyExists);
        }

        if initial_percentage < 0 || initial_percentage > 10000 {
            return Err(Error::InvalidPercentage);
        }

        set_admin(&env, &admin);
        set_governance(&env, &governance);
        set_carbon_asset_contract(&env, &carbon_asset_contract);
        set_replenishment_percentage(&env, initial_percentage);
        set_total_value_locked(&env, 0);

        Ok(())
    }

    pub fn deposit(
        env: Env,
        caller: Address,
        token_id: u32,
        project_id: String,
    ) -> Result<(), Error> {
        let admin = get_admin(&env);
        let carbon_contract = get_carbon_asset_contract(&env);

        // TODO: need to emit event
        if caller != admin && caller != carbon_contract {
            return Err(Error::Unauthorized);
        }

        caller.require_auth();

        if has_custody_record(&env, token_id) {
            return Err(Error::AlreadyExists);
        }

        let record = CustodyRecord {
            token_id,
            deposited_at: env.ledger().timestamp(),
            depositor: caller,
            project_id,
        };

        set_custody_record(&env, token_id, &record);

        let tvl = get_total_value_locked(&env);
        set_total_value_locked(&env, tvl + 1);

        Ok(())
    }

    pub fn withdraw_to_replace(
        env: Env,
        governance_caller: Address,
        token_id: u32,
        target_invalidated_token: u32,
    ) -> Result<(), Error> {
        let governance = get_governance(&env);

        if governance_caller != governance {
            return Err(Error::Unauthorized);
        }

        governance_caller.require_auth();

        // TODO: validate token exists
        if !has_custody_record(&env, token_id) {
            return Err(Error::TokenNotFound);
        }

        let key = soroban_sdk::Symbol::short("custody");
        env.storage().persistent().remove(&(key, token_id));

        let tvl = get_total_value_locked(&env);
        set_total_value_locked(&env, tvl - 1);

        // TODO: emit event with target_invalidated_token

        Ok(())
    }
}



