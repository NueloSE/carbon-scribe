-- Financing & Tokenization Service Database Schema
-- Migration: 005_financing_tables.sql

-- Carbon credit calculations
CREATE TABLE carbon_credits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id),
    vintage_year INTEGER NOT NULL,
    calculation_period_start DATE NOT NULL,
    calculation_period_end DATE NOT NULL,
    
    -- Credit details
    methodology_code VARCHAR(50) NOT NULL, -- 'VM0007', 'VM0015', etc.
    calculated_tons DECIMAL(12, 4) NOT NULL,
    buffered_tons DECIMAL(12, 4) NOT NULL, -- After uncertainty buffer
    issued_tons DECIMAL(12, 4), -- Actually minted tokens
    data_quality_score DECIMAL(3, 2),
    
    -- Stellar integration
    stellar_asset_code VARCHAR(12), -- e.g., 'CARBON001'
    stellar_asset_issuer VARCHAR(56), -- G... address
    token_ids JSONB, -- Array of minted token IDs from smart contract
    mint_transaction_hash VARCHAR(64),
    minted_at TIMESTAMPTZ,
    
    -- Status
    status VARCHAR(50) DEFAULT 'calculated', -- 'calculated', 'verified', 'minting', 'minted', 'retired'
    verification_id UUID REFERENCES verifications(id),
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Forward sale agreements
CREATE TABLE forward_sale_agreements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id),
    buyer_id UUID NOT NULL REFERENCES users(id),
    vintage_year INTEGER NOT NULL,
    
    -- Terms
    tons_committed DECIMAL(12, 4) NOT NULL,
    price_per_ton DECIMAL(10, 4) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    total_amount DECIMAL(14, 4) NOT NULL,
    delivery_date DATE NOT NULL,
    
    -- Payment
    deposit_percent DECIMAL(5, 2) NOT NULL,
    deposit_paid BOOLEAN DEFAULT FALSE,
    deposit_transaction_id VARCHAR(100),
    payment_schedule JSONB, -- Milestone payments
    
    -- Legal
    contract_hash VARCHAR(64), -- Hash of signed contract
    signed_by_seller_at TIMESTAMPTZ,
    signed_by_buyer_at TIMESTAMPTZ,
    
    -- Status
    status VARCHAR(50) DEFAULT 'pending', -- 'pending', 'active', 'completed', 'cancelled'
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Revenue distribution
CREATE TABLE revenue_distributions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    credit_sale_id UUID NOT NULL, -- References credit sales or forward sales
    distribution_type VARCHAR(50) NOT NULL, -- 'credit_sale', 'forward_sale', 'royalty'
    
    -- Amounts
    total_received DECIMAL(14, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    platform_fee_percent DECIMAL(5, 2) NOT NULL,
    platform_fee_amount DECIMAL(12, 4) NOT NULL,
    net_amount DECIMAL(14, 4) NOT NULL,
    
    -- Distribution splits
    beneficiaries JSONB NOT NULL, -- Array of {user_id, percent, amount, tax_withheld}
    
    -- Payment execution
    payment_batch_id VARCHAR(100), -- Reference to bulk payment execution
    payment_status VARCHAR(50) DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'failed'
    payment_processed_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Payment transactions
CREATE TABLE payment_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(100) UNIQUE, -- ID from payment processor
    user_id UUID REFERENCES users(id),
    project_id UUID REFERENCES projects(id),
    
    -- Payment details
    amount DECIMAL(14, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    payment_method VARCHAR(50) NOT NULL, -- 'credit_card', 'bank_transfer', 'stellar', 'mpesa'
    payment_provider VARCHAR(50) NOT NULL, -- 'stripe', 'paypal', 'stellar_network'
    
    -- Status
    status VARCHAR(50) DEFAULT 'initiated', -- 'initiated', 'processing', 'completed', 'failed', 'refunded'
    provider_status JSONB, -- Raw status from payment provider
    failure_reason TEXT,
    
    -- Blockchain specifics (for Stellar payments)
    stellar_transaction_hash VARCHAR(64),
    stellar_asset_code VARCHAR(12),
    stellar_asset_issuer VARCHAR(56),
    
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Credit pricing
CREATE TABLE credit_pricing_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    methodology_code VARCHAR(50) NOT NULL,
    region_code VARCHAR(10),
    vintage_year INTEGER,
    
    -- Pricing factors
    base_price DECIMAL(10, 4) NOT NULL,
    quality_multiplier JSONB, -- Factors for data quality, co-benefits
    market_multiplier DECIMAL(6, 4) DEFAULT 1.0,
    
    -- Validity
    valid_from DATE NOT NULL,
    valid_until DATE,
    is_active BOOLEAN DEFAULT TRUE,
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance optimization
CREATE INDEX idx_carbon_credits_project_id ON carbon_credits(project_id);
CREATE INDEX idx_carbon_credits_vintage_year ON carbon_credits(vintage_year);
CREATE INDEX idx_carbon_credits_status ON carbon_credits(status);
CREATE INDEX idx_carbon_credits_methodology ON carbon_credits(methodology_code);

CREATE INDEX idx_forward_sales_project_id ON forward_sale_agreements(project_id);
CREATE INDEX idx_forward_sales_buyer_id ON forward_sale_agreements(buyer_id);
CREATE INDEX idx_forward_sales_status ON forward_sale_agreements(status);
CREATE INDEX idx_forward_sales_delivery_date ON forward_sale_agreements(delivery_date);

CREATE INDEX idx_revenue_distributions_credit_sale_id ON revenue_distributions(credit_sale_id);
CREATE INDEX idx_revenue_distributions_payment_status ON revenue_distributions(payment_status);

CREATE INDEX idx_payment_transactions_user_id ON payment_transactions(user_id);
CREATE INDEX idx_payment_transactions_project_id ON payment_transactions(project_id);
CREATE INDEX idx_payment_transactions_status ON payment_transactions(status);
CREATE INDEX idx_payment_transactions_external_id ON payment_transactions(external_id);

CREATE INDEX idx_pricing_models_methodology ON credit_pricing_models(methodology_code);
CREATE INDEX idx_pricing_models_region ON credit_pricing_models(region_code);
CREATE INDEX idx_pricing_models_vintage ON credit_pricing_models(vintage_year);
CREATE INDEX idx_pricing_models_active ON credit_pricing_models(is_active) WHERE is_active = TRUE;
