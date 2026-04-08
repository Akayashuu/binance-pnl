import { writable } from 'svelte/store';

const STORAGE_KEY = 'binancetracker.display_currency';
const DEFAULT = 'EUR';

// Common fiat + stable currencies. The backend FX layer can route any of
// these via Binance ticker pairs (with USDT as the bridge).
export const SUPPORTED_DISPLAY_CURRENCIES = [
	'EUR',
	'USD',
	'USDT',
	'USDC',
	'GBP',
	'JPY',
	'CHF',
	'BTC'
];

function initial(): string {
	if (typeof window === 'undefined') return DEFAULT;
	const stored = window.localStorage.getItem(STORAGE_KEY);
	return stored && SUPPORTED_DISPLAY_CURRENCIES.includes(stored) ? stored : DEFAULT;
}

function createStore() {
	const { subscribe, set } = writable<string>(initial());

	return {
		subscribe,
		setCurrency(c: string) {
			if (typeof window !== 'undefined') {
				window.localStorage.setItem(STORAGE_KEY, c);
			}
			set(c);
		}
	};
}

export const displayCurrency = createStore();

// Read the current value synchronously, useful for the API client which
// can't subscribe to a store from a plain function call.
export function getDisplayCurrency(): string {
	if (typeof window === 'undefined') return DEFAULT;
	return window.localStorage.getItem(STORAGE_KEY) ?? DEFAULT;
}
