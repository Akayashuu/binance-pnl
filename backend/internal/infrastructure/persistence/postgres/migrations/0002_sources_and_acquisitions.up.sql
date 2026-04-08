-- Migration 0002: introduce per-trade source tracking and a separate
-- acquisitions table for non-trade events (deposits, Earn rewards).

ALTER TABLE trades
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'spot'
        CHECK (source IN ('spot', 'convert', 'fiat_buy', 'recurring'));

CREATE INDEX IF NOT EXISTS idx_trades_source ON trades (source);

CREATE TABLE IF NOT EXISTS acquisitions (
    id              TEXT PRIMARY KEY,
    asset           TEXT NOT NULL REFERENCES assets(symbol),
    quote           TEXT NOT NULL,
    source          TEXT NOT NULL CHECK (source IN ('deposit', 'earn_reward')),
    quantity        NUMERIC(36, 18) NOT NULL CHECK (quantity > 0),
    unit_cost       NUMERIC(36, 18) NOT NULL CHECK (unit_cost >= 0),
    acquired_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_acquisitions_asset_time ON acquisitions (asset, acquired_at);
CREATE INDEX IF NOT EXISTS idx_acquisitions_acquired_at ON acquisitions (acquired_at);
CREATE INDEX IF NOT EXISTS idx_acquisitions_source ON acquisitions (source);
