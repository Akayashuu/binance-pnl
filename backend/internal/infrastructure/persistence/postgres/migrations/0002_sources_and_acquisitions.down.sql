DROP TABLE IF EXISTS acquisitions;
DROP INDEX IF EXISTS idx_trades_source;
ALTER TABLE trades DROP COLUMN IF EXISTS source;
