CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id VARCHAR(100) NOT NULL,
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency CHAR(3) NOT NULL DEFAULT 'INR',
    merchant_id VARCHAR(100) NOT NULL,
    country_code CHAR(2) NOT NULL,
    payment_method VARCHAR(30) NOT NULL,
    risk_score INTEGER NOT NULL DEFAULT 0
        CHECK (risk_score BETWEEN 0 AND 100),
    risk_decision VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (
            risk_decision IN ('pending', 'approve', 'review', 'flag')
        ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_customer_id
    ON transactions(customer_id);

CREATE INDEX IF NOT EXISTS idx_transactions_created_at
    ON transactions(created_at DESC);
