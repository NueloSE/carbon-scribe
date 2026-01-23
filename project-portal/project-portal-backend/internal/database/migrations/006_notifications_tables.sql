-- 006_notifications_tables.sql
-- Migration for notification system tables

-- Notification Categories
CREATE TABLE IF NOT EXISTS notification_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    default_channels TEXT[] NOT NULL DEFAULT ARRAY['EMAIL', 'IN_APP'],
    is_critical BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Add index on code for lookups
CREATE INDEX IF NOT EXISTS idx_notification_categories_code ON notification_categories(code);

-- Sent Notifications (for relational queries and reporting)
CREATE TABLE IF NOT EXISTS sent_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    category_id UUID REFERENCES notification_categories(id),
    channel VARCHAR(50) NOT NULL CHECK (channel IN ('EMAIL', 'SMS', 'PUSH', 'WEBSOCKET', 'IN_APP')),
    subject VARCHAR(500),
    content TEXT NOT NULL,
    status VARCHAR(50) DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'SENT', 'DELIVERED', 'FAILED', 'BOUNCED', 'COMPLAINT')),
    provider_id VARCHAR(255),
    sent_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    opened_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_sent_notifications_user_id ON sent_notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_sent_notifications_status ON sent_notifications(status);
CREATE INDEX IF NOT EXISTS idx_sent_notifications_channel ON sent_notifications(channel);
CREATE INDEX IF NOT EXISTS idx_sent_notifications_created_at ON sent_notifications(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sent_notifications_category_id ON sent_notifications(category_id);

-- User Notification Preferences (RDS backup for DynamoDB)
CREATE TABLE IF NOT EXISTS user_notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    channel VARCHAR(50) NOT NULL,
    category_code VARCHAR(100) NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    quiet_hours_start TIME,
    quiet_hours_end TIME,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, channel, category_code)
);

CREATE INDEX IF NOT EXISTS idx_user_notification_preferences_user_id ON user_notification_preferences(user_id);

-- Notification Templates (RDS backup for DynamoDB)
CREATE TABLE IF NOT EXISTS notification_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(100) NOT NULL,
    language VARCHAR(10) NOT NULL DEFAULT 'en',
    version_id VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    subject TEXT NOT NULL,
    body TEXT NOT NULL,
    variables TEXT[] DEFAULT ARRAY[]::TEXT[],
    metadata JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(type, language, version_id)
);

CREATE INDEX IF NOT EXISTS idx_notification_templates_type_lang ON notification_templates(type, language);
CREATE INDEX IF NOT EXISTS idx_notification_templates_active ON notification_templates(is_active) WHERE is_active = TRUE;

-- Alert Rules (RDS backup for DynamoDB)
CREATE TABLE IF NOT EXISTS alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    conditions JSONB NOT NULL DEFAULT '[]',
    actions JSONB NOT NULL DEFAULT '[]',
    is_active BOOLEAN DEFAULT TRUE,
    last_triggered_at TIMESTAMPTZ,
    trigger_count INTEGER DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_project_id ON alert_rules(project_id);
CREATE INDEX IF NOT EXISTS idx_alert_rules_active ON alert_rules(is_active) WHERE is_active = TRUE;

-- Delivery Logs (for analytics and debugging)
CREATE TABLE IF NOT EXISTS notification_delivery_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL,
    user_id UUID NOT NULL,
    channel VARCHAR(50) NOT NULL,
    template_id UUID REFERENCES notification_templates(id),
    status VARCHAR(50) NOT NULL,
    provider_message_id VARCHAR(255),
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    sent_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_delivery_logs_notification_id ON notification_delivery_logs(notification_id);
CREATE INDEX IF NOT EXISTS idx_delivery_logs_user_id ON notification_delivery_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_delivery_logs_status ON notification_delivery_logs(status);
CREATE INDEX IF NOT EXISTS idx_delivery_logs_created_at ON notification_delivery_logs(created_at DESC);

-- Insert default notification categories
INSERT INTO notification_categories (code, name, description, default_channels, is_critical) VALUES
    ('MONITORING_ALERTS', 'Monitoring Alerts', 'Alerts from IoT monitoring systems', ARRAY['EMAIL', 'SMS', 'WEBSOCKET'], TRUE),
    ('PAYMENT_UPDATES', 'Payment Updates', 'Credit and payment notifications', ARRAY['EMAIL', 'IN_APP'], FALSE),
    ('PROJECT_UPDATES', 'Project Updates', 'Project status and milestone updates', ARRAY['EMAIL', 'IN_APP'], FALSE),
    ('VERIFICATION_STATUS', 'Verification Status', 'Project verification status changes', ARRAY['EMAIL', 'IN_APP', 'WEBSOCKET'], TRUE),
    ('SYSTEM_ANNOUNCEMENTS', 'System Announcements', 'Platform-wide announcements', ARRAY['EMAIL', 'IN_APP'], FALSE)
ON CONFLICT (code) DO NOTHING;

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
CREATE TRIGGER update_notification_categories_updated_at
    BEFORE UPDATE ON notification_categories
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sent_notifications_updated_at
    BEFORE UPDATE ON sent_notifications
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_notification_preferences_updated_at
    BEFORE UPDATE ON user_notification_preferences
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_notification_templates_updated_at
    BEFORE UPDATE ON notification_templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_alert_rules_updated_at
    BEFORE UPDATE ON alert_rules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
