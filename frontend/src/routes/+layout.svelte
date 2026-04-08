<script lang="ts">
	import '../app.css';
	import { _, isLoading } from 'svelte-i18n';
	import { page } from '$app/stores';
	import { setupI18n } from '$lib/i18n';
	import { displayCurrency, SUPPORTED_DISPLAY_CURRENCIES } from '$lib/stores/display-currency';
	import { portfolio } from '$lib/stores/portfolio';
	import { LineChart, ListOrdered, Settings as SettingsIcon } from 'lucide-svelte';

	setupI18n();

	let { children } = $props();

	const navItems = [
		{ href: '/', label: 'nav.dashboard', icon: LineChart },
		{ href: '/trades', label: 'nav.trades', icon: ListOrdered },
		{ href: '/settings', label: 'nav.settings', icon: SettingsIcon }
	];

	function isActive(href: string, pathname: string): boolean {
		if (href === '/') return pathname === '/' || pathname === '';
		return pathname.startsWith(href);
	}

	function onCurrencyChange(e: Event) {
		const value = (e.currentTarget as HTMLSelectElement).value;
		displayCurrency.setCurrency(value);
		// Re-fetch the dashboard so the new display currency takes effect
		// immediately without waiting for the next poll tick.
		void portfolio.load();
	}
</script>

{#if $isLoading}
	<div class="flex h-screen items-center justify-center text-muted-foreground">Loading…</div>
{:else}
	<div class="flex min-h-screen flex-col">
		<header class="border-b bg-card">
			<div class="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
				<a href="/" class="flex items-center gap-2">
					<span class="text-lg font-semibold">{$_('app.title')}</span>
					<span class="hidden text-xs text-muted-foreground sm:inline">— {$_('app.tagline')}</span>
				</a>
				<nav class="flex items-center gap-1">
					{#each navItems as item}
						{@const Icon = item.icon}
						<a
							href={item.href}
							class="flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground {isActive(
								item.href,
								$page.url.pathname
							)
								? 'bg-accent text-accent-foreground'
								: 'text-muted-foreground'}"
						>
							<Icon class="h-4 w-4" />
							{$_(item.label)}
						</a>
					{/each}
					<select
						class="ml-2 rounded-md border bg-background px-2 py-1 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
						value={$displayCurrency}
						onchange={onCurrencyChange}
						aria-label="Display currency"
					>
						{#each SUPPORTED_DISPLAY_CURRENCIES as cc}
							<option value={cc}>{cc}</option>
						{/each}
					</select>
				</nav>
			</div>
		</header>

		<main class="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
			{@render children()}
		</main>

		<footer class="border-t py-4 text-center text-xs text-muted-foreground">
			binancetracker · MIT · self-hosted
		</footer>
	</div>
{/if}
