import { env } from '$env/dynamic/public';
import { getDisplayCurrency } from '$lib/stores/display-currency';
import type {
	AssetDetail,
	KlinePoint,
	Portfolio,
	Settings,
	SyncResult,
	Trade,
	UpdateSettings
} from './types';

const BASE = (env.PUBLIC_API_BASE_URL ?? 'http://localhost:8080').replace(/\/$/, '');

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
	const res = await fetch(`${BASE}${path}`, {
		...init,
		headers: {
			'Content-Type': 'application/json',
			Accept: 'application/json',
			...(init.headers ?? {})
		}
	});
	if (!res.ok) {
		const text = await res.text().catch(() => '');
		throw new Error(`API ${res.status}: ${text || res.statusText}`);
	}
	if (res.status === 204) return undefined as T;
	return (await res.json()) as T;
}

// withDisplay appends ?display=XXX (or &display=XXX) so the backend converts
// money values into the user-chosen currency without us having to thread it
// through every call site.
function withDisplay(path: string): string {
	const sep = path.includes('?') ? '&' : '?';
	return `${path}${sep}display=${encodeURIComponent(getDisplayCurrency())}`;
}

export interface CreateAcquisitionInput {
	asset: string;
	quote: string;
	quantity: string;
	unit_cost: string;
	acquired_at?: string;
}

export const api = {
	getPortfolio: () => request<Portfolio>(withDisplay('/api/v1/portfolio')),
	listTrades: (asset?: string) =>
		request<Trade[]>(
			withDisplay(`/api/v1/trades${asset ? `?asset=${encodeURIComponent(asset)}` : ''}`)
		),
	getAssetDetail: (symbol: string) =>
		request<AssetDetail>(withDisplay(`/api/v1/assets/${encodeURIComponent(symbol)}`)),
	sync: (full = false) =>
		request<SyncResult>(`/api/v1/sync${full ? '?full=true' : ''}`, { method: 'POST' }),
	getSettings: () => request<Settings>('/api/v1/settings'),
	updateSettings: (body: UpdateSettings) =>
		request<void>('/api/v1/settings', { method: 'PUT', body: JSON.stringify(body) }),
	createAcquisition: (body: CreateAcquisitionInput) =>
		request<{ id: string }>('/api/v1/acquisitions', {
			method: 'POST',
			body: JSON.stringify(body)
		}),
	updateAcquisition: (id: string, body: CreateAcquisitionInput) =>
		request<{ id: string }>(`/api/v1/acquisitions/${encodeURIComponent(id)}`, {
			method: 'PUT',
			body: JSON.stringify(body)
		}),
	deleteAcquisition: (id: string) =>
		request<void>(`/api/v1/acquisitions/${encodeURIComponent(id)}`, { method: 'DELETE' }),
	getKlines: (symbol: string, interval = '1d', limit = 90) =>
		request<KlinePoint[]>(
			`/api/v1/klines/${encodeURIComponent(symbol)}?interval=${interval}&limit=${limit}`
		)
};
