CREATE TABLE IF NOT EXISTS assets (
    symbol TEXT PRIMARY KEY,
    name   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS trades (
    id          TEXT PRIMARY KEY,
    asset       TEXT NOT NULL REFERENCES assets(symbol),
    quote       TEXT NOT NULL,
    side        TEXT NOT NULL CHECK (side IN ('BUY', 'SELL')),
    quantity    NUMERIC(36, 18) NOT NULL CHECK (quantity > 0),
    price       NUMERIC(36, 18) NOT NULL CHECK (price >= 0),
    fee         NUMERIC(36, 18) NOT NULL DEFAULT 0,
    fee_asset   TEXT NOT NULL,
    executed_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_trades_asset_time ON trades (asset, executed_at);
CREATE INDEX IF NOT EXISTS idx_trades_executed_at ON trades (executed_at);

CREATE TABLE IF NOT EXISTS price_quotes (
    asset      TEXT PRIMARY KEY REFERENCES assets(symbol),
    price      NUMERIC(36, 18) NOT NULL,
    quote      TEXT NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
