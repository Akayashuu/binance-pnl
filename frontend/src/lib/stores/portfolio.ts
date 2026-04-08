import { writable } from 'svelte/store';
import { api } from '$lib/api/client';
import type { Portfolio } from '$lib/api/types';

interface PortfolioState {
	data: Portfolio | null;
	loading: boolean;
	error: string | null;
}

function createPortfolioStore() {
	const { subscribe, update, set } = writable<PortfolioState>({
		data: null,
		loading: false,
		error: null
	});

	let timer: ReturnType<typeof setInterval> | null = null;

	async function load() {
		update((s) => ({ ...s, loading: true, error: null }));
		try {
			const data = await api.getPortfolio();
			set({ data, loading: false, error: null });
		} catch (err) {
			set({ data: null, loading: false, error: (err as Error).message });
		}
	}

	function startPolling(intervalMs = 30_000) {
		if (timer) return;
		void load();
		timer = setInterval(() => void load(), intervalMs);
	}

	function stopPolling() {
		if (timer) {
			clearInterval(timer);
			timer = null;
		}
	}

	return { subscribe, load, startPolling, stopPolling };
}

export const portfolio = createPortfolioStore();
