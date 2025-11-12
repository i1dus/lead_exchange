-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS deals
(
    deal_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lead_id        UUID        NOT NULL REFERENCES leads(lead_id) ON DELETE CASCADE,
    seller_user_id UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    buyer_user_id  UUID        REFERENCES users(user_id) ON DELETE SET NULL,
    price          NUMERIC(12, 2) NOT NULL CHECK (price > 0),
    status         TEXT        NOT NULL DEFAULT 'PENDING',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_deals_lead_id ON deals(lead_id);
CREATE INDEX idx_deals_seller_user_id ON deals(seller_user_id);
CREATE INDEX idx_deals_buyer_user_id ON deals(buyer_user_id);
CREATE INDEX idx_deals_status ON deals(status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS deals;

-- +goose StatementEnd


