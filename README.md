# binancetracker

A small self-hosted thing I built because I was tired of guessing how my Binance
buys were doing. It pulls every transaction Binance lets me see (spot, Convert,
Buy Crypto, deposits) and tells me, for each one: how much it cost, how much
it's worth right now, and the gap between the two.

The whole point is to answer one question — "on this purchase, am I up or
down?" — without uploading anything to Coinstats / CoinTracker / whatever.

Read-only API keys. No trading endpoints are ever called.

## What it actually does

- Imports your spot trades, Convert orders, and "Buy Crypto" (card / SEPA)
  purchases from Binance.
- Imports crypto deposits and prices them at the spot price of the moment they
  hit the account (so an external transfer still has a meaningful cost basis).
- Lets you add a "fund" by hand if you got coins from somewhere Binance doesn't
  see — and if you don't remember the price, leave it empty and it'll fetch
  the historical close from Binance klines.
- Computes a per-asset position (weighted average cost) **and** a per-lot
  FIFO breakdown so you can see, for each individual purchase, the unrealised
  P&L on what's left.
- Shows everything in your currency of choice (EUR, USD, GBP, JPY, BTC…) with
  the native quote underneath. Toggle in the header.
- Edit / delete the manual entries when you mess up.

## What it intentionally doesn't do

- No trading. No order placement. The Binance API key only needs read access.
- No "smart" tax reporting. Cost basis is AVCO + FIFO, no LIFO, no specific
  identification, no HIFO.
- No Earn / Simple Earn rewards yet — Binance deprecated the legacy lending
  endpoint and the Go SDK doesn't expose the new one. The hook is there for
  when it lands.
- No cross-currency Convert (e.g. BTC ↔ ETH) unless one leg is your configured
  quote currency. Adding it would need a historical price lookup at convert
  time and wasn't worth it for v1.
- No FX historical accuracy: when you have an EUR fiat-buy and the position is
  in USDT, the conversion uses the **current** USDT/EUR rate, not the rate of
  the day. Cost basis is approximate on old EUR trades. Quantity is exact.

## Stack

Go 1.23 backend (hexagonal — `domain ← application ← infrastructure`),
SvelteKit 5 frontend, PostgreSQL 16, all glued together with Docker Compose.
The backend talks to Binance via [adshao/go-binance](https://github.com/adshao/go-binance).

## Run it

```bash
git clone https://github.com/<you>/binancetracker.git
cd binancetracker
cp .env.example .env

# 32-byte AES key for encrypting your Binance secret at rest in postgres
openssl rand -base64 32   # paste into ENCRYPTION_KEY=

docker compose up --build
```

Then open http://localhost:5173, go to **Settings**, paste your Binance API
key + secret, and click **Sync Binance**. The first sync walks ~1 year of
history (Buy Crypto, deposits) and ~6 months of Convert.

## Getting a Binance read-only key

1. Binance → **API Management** → **Create API** → **System generated**.
2. Under **API restrictions**, untick everything except **Enable Reading**.
   You also need access to the SAPI endpoints (deposits, fiat payments,
   convert history). The default key has it.
3. If you whitelist an IP, make sure it's the public IP of wherever the
   backend is running, not your laptop.

If you get `-2015 Invalid API-key, IP, or permissions for action`, the key
either doesn't have reading enabled or your IP isn't in the whitelist.

## Layout

```
backend/
  cmd/api/                       composition root — the only place infra is wired
  internal/
    domain/                      pure business types (Trade, Acquisition, Lot, …)
    application/                 use cases + ports
    infrastructure/              postgres, binance, http, fx
frontend/
  src/lib/                       components, stores, api client
  src/routes/                    SvelteKit pages
```

The hexagonal split is real: `domain` and `application` import zero infra
packages. Adding Kraken would mean writing a new `ExchangeImporter` adapter
under `internal/infrastructure/kraken` and wiring it in `cmd/api/main.go`.
Nothing in `internal/application` would need to change.

## Hacking on it

```bash
# Backend
cd backend && go test ./... && go vet ./...

# Frontend
cd frontend && pnpm install && pnpm run check && pnpm dev
```

The Postgres migrations live in
`backend/internal/infrastructure/persistence/postgres/migrations/` and run
automatically at server start via golang-migrate.

## License

MIT.
