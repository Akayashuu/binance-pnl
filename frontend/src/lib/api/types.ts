// API DTOs — keep in sync with backend/internal/infrastructure/http/dto.go

export interface Money {
	amount: string;
	currency: string;
	display?: { amount: string; currency: string };
}

export interface PnL {
	asset: string;
	quote: string;
	held_quantity: string;
	average_cost: Money;
	current_price: Money;
	market_value: Money;
	cost_basis: Money;
	unrealized_pnl: Money;
	realized_pnl: Money;
	total_pnl: Money;
	unrealized_pct_bp: number;
}

export interface Portfolio {
	quote: string;
	generated_at: string;
	total_invested: Money;
	total_value: Money;
	unrealized_pnl: Money;
	realized_pnl: Money;
	total_pnl: Money;
	positions: PnL[];
}

export type Source = 'spot' | 'convert' | 'fiat_buy' | 'recurring' | 'deposit' | 'earn_reward';

export interface Trade {
	id: string;
	asset: string;
	quote: string;
	side: 'BUY' | 'SELL';
	source: Source;
	quantity: string;
	price: Money;
	fee: Money;
	executed_at: string;
	gross_value: Money;
	delta_total: Money;
	delta_pct: string;
	remaining_qty: string;
	remaining_pnl: Money;
}

export interface Lot {
	acquisition_id: string;
	source: Source;
	acquired_at: string;
	original_quantity: string;
	remaining_quantity: string;
	unit_cost: Money;
	cost_basis: Money;
	current_price: Money;
	current_value: Money;
	unrealized_pnl: Money;
	unrealized_pnl_pct: string;
}

export interface AssetDetail {
	pnl: PnL;
	trades: Trade[];
	lots: Lot[];
}

export interface Settings {
	binance_api_key_set: boolean;
	binance_api_secret_set: boolean;
	quote_currency: string;
}

export interface UpdateSettings {
	binance_api_key?: string;
	binance_api_secret?: string;
	quote_currency?: string;
}

export interface SyncResult {
	imported: number;
	by_source: Record<string, number>;
	partial_failure: boolean;
	errors?: string[];
}

export interface KlinePoint {
	time: number; // unix ms
	open: string;
	high: string;
	low: string;
	close: string;
	volume: string;
}
