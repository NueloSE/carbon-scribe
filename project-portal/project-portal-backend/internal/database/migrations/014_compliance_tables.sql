-- Data retention policies
CREATE TABLE retention_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- Policy scope
    data_category VARCHAR(100) NOT NULL, -- 'user_profile', 'project_data', 'financial_records', 'system_logs'
    jurisdiction VARCHAR(50) DEFAULT 'global', -- Country codes or 'global'
    
    -- Retention periods
    retention_period_days INTEGER NOT NULL, -- -1 = indefinite, 0 = immediate deletion
    archival_period_days INTEGER, -- When to move to cold storage
    review_period_days INTEGER DEFAULT 365, -- When to review for continued necessity
    
    -- Deletion behavior
    deletion_method VARCHAR(50) DEFAULT 'soft_delete', -- 'hard_delete', 'anonymize', 'pseudonymize'
    anonymization_rules JSONB, -- Rules for data anonymization
    
    -- Legal hold
    legal_hold_enabled BOOLEAN DEFAULT TRUE,
    
    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    version INTEGER DEFAULT 1,
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- User data requests (GDPR requests)
CREATE TABLE privacy_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL, -- Assuming users table exists, but not adding FK constraint to avoid dependency issues in this migration if users table is not yet created or in a different schema. Ideally should be REFERENCES users(id) ON DELETE CASCADE
    request_type VARCHAR(50) NOT NULL, -- 'export', 'deletion', 'correction', 'restriction'
    request_subtype VARCHAR(50), -- 'full_export', 'partial_export', 'complete_deletion', 'partial_deletion'
    
    -- Request details
    status VARCHAR(50) DEFAULT 'received', -- 'received', 'processing', 'completed', 'failed', 'cancelled'
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,
    estimated_completion TIMESTAMPTZ,
    
    -- Scope
    data_categories TEXT[], -- Specific data categories requested
    date_range_start TIMESTAMPTZ,
    date_range_end TIMESTAMPTZ,
    
    -- Verification
    verification_method VARCHAR(50), -- 'email', 'id_document', 'admin_override'
    verified_by UUID, -- REFERENCES users(id)
    verified_at TIMESTAMPTZ,
    
    -- Results
    export_file_url TEXT, -- For export requests
    export_file_hash VARCHAR(64),
    deletion_summary JSONB, -- For deletion requests
    error_message TEXT,
    
    -- Legal basis
    legal_basis VARCHAR(100), -- 'consent', 'contract', 'legal_obligation', 'legitimate_interest'
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Privacy preferences and consents
CREATE TABLE privacy_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL, -- REFERENCES users(id) ON DELETE CASCADE
    
    -- Communication preferences
    marketing_emails BOOLEAN DEFAULT FALSE,
    promotional_emails BOOLEAN DEFAULT FALSE,
    system_notifications BOOLEAN DEFAULT TRUE,
    third_party_sharing BOOLEAN DEFAULT FALSE,
    analytics_tracking BOOLEAN DEFAULT TRUE,
    
    -- Data processing preferences
    data_retention_consent BOOLEAN DEFAULT TRUE,
    research_participation BOOLEAN DEFAULT FALSE,
    automated_decision_making BOOLEAN DEFAULT FALSE,
    
    -- Regional preferences
    jurisdiction VARCHAR(50) DEFAULT 'GDPR', -- Default jurisdiction
    
    -- Versioning
    version INTEGER DEFAULT 1,
    previous_version_id UUID REFERENCES privacy_preferences(id),
    
    UNIQUE(user_id),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Consent records (granular tracking)
CREATE TABLE consent_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL, -- REFERENCES users(id) ON DELETE CASCADE
    
    -- Consent details
    consent_type VARCHAR(100) NOT NULL, -- 'marketing', 'privacy_policy', 'terms_of_service', 'cookies'
    consent_version VARCHAR(50) NOT NULL, -- Version of document consented to
    consent_given BOOLEAN NOT NULL,
    
    -- Context
    context TEXT, -- Where consent was given (URL, form identifier)
    purpose TEXT, -- Purpose of data processing
    
    -- Evidence
    ip_address INET,
    user_agent TEXT,
    geolocation VARCHAR(100),
    
    -- Validity
    expires_at TIMESTAMPTZ,
    withdrawn_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_consent_records_user ON consent_records (user_id, consent_type, created_at DESC);

-- Immutable audit logs (Write Once Read Many pattern)
CREATE TABLE audit_logs (
    -- Immutable ID (timestamp-based for ordering)
    log_id BIGSERIAL,
    
    -- Event metadata
    event_time TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    event_type VARCHAR(100) NOT NULL, -- 'data_access', 'data_modification', 'export', 'deletion'
    event_action VARCHAR(50) NOT NULL, -- 'read', 'create', 'update', 'delete', 'export'
    
    -- Actor information
    actor_id UUID, -- User or system that performed the action
    actor_type VARCHAR(50), -- 'user', 'system', 'api_client'
    actor_ip INET,
    
    -- Target information
    target_type VARCHAR(100), -- Type of data accessed ('user', 'project', 'document')
    target_id UUID, -- ID of the accessed entity
    target_owner_id UUID, -- Owner of the accessed data
    
    -- Data sensitivity
    data_category VARCHAR(100),
    sensitivity_level VARCHAR(50) DEFAULT 'normal', -- 'normal', 'sensitive', 'highly_sensitive'
    
    -- Context
    service_name VARCHAR(100) NOT NULL,
    endpoint VARCHAR(500),
    http_method VARCHAR(10),
    
    -- Changes (for modification events)
    old_values JSONB,
    new_values JSONB,
    
    -- Authorization
    permission_used VARCHAR(100),
    
    -- Verification
    signature VARCHAR(512), -- Cryptographic signature for tamper detection
    hash_chain VARCHAR(64), -- For creating hash chain of logs
    
    -- No updated_at column - immutable by design
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (log_id, event_time)
) PARTITION BY RANGE (event_time);

-- Create monthly partitions for audit logs (Example for 2024-2026)
CREATE TABLE audit_logs_y2024m01 PARTITION OF audit_logs FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
CREATE TABLE audit_logs_y2024m02 PARTITION OF audit_logs FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');
-- ... In a real scenario, these would be created dynamically or pre-created for a longer range.
-- Creating a default partition for future/unexpected dates to avoid insert errors
CREATE TABLE audit_logs_default PARTITION OF audit_logs DEFAULT;

CREATE INDEX idx_audit_logs_event_time ON audit_logs (event_time DESC);
CREATE INDEX idx_audit_logs_actor ON audit_logs (actor_id, event_time DESC);
CREATE INDEX idx_audit_logs_target ON audit_logs (target_type, target_id, event_time DESC);
CREATE INDEX idx_audit_logs_owner ON audit_logs (target_owner_id, event_time DESC);

-- Data retention schedule
CREATE TABLE retention_schedule (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id UUID NOT NULL REFERENCES retention_policies(id),
    data_type VARCHAR(100) NOT NULL, -- Specific data type this schedule applies to
    
    -- Schedule
    next_review_date DATE NOT NULL,
    next_action_date DATE, -- When action (archive/delete) should be taken
    action_type VARCHAR(50), -- 'review', 'archive', 'delete', 'anonymize'
    
    -- Status
    last_action_date DATE,
    last_action_type VARCHAR(50),
    last_action_result VARCHAR(50),
    
    -- Metrics
    record_count_estimate BIGINT,
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
