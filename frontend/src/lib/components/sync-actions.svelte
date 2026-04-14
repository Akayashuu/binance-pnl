<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { Button } from '$lib/components/ui';
	import { api } from '$lib/api/client';
	import { portfolio } from '$lib/stores/portfolio';
	import { RefreshCw } from 'lucide-svelte';

	let syncing = $state(false);
	let message = $state<string | null>(null);
	let errors = $state<string[]>([]);

	async function sync(full: boolean) {
		syncing = true;
		message = null;
		errors = [];
		try {
			const res = await api.sync(full);
			const parts = Object.entries(res.by_source ?? {})
				.filter(([, n]) => n > 0)
				.map(([src, n]) => `${n} ${src}`);
			message =
				parts.length > 0
					? `${$_('dashboard.sync_done', { values: { count: res.imported } })} (${parts.join(', ')})`
					: $_('dashboard.sync_done', { values: { count: res.imported } });
			if (res.errors?.length) errors = res.errors;
			await portfolio.load();
		} catch (err) {
			message = (err as Error).message;
		} finally {
			syncing = false;
		}
	}
</script>

<div class="flex gap-2">
	<Button variant="outline" onclick={() => sync(true)} disabled={syncing}>
		<RefreshCw class={`mr-2 h-4 w-4 ${syncing ? 'animate-spin' : ''}`} />
		{syncing ? $_('dashboard.syncing') : $_('dashboard.full_sync')}
	</Button>
	<Button onclick={() => sync(false)} disabled={syncing}>
		<RefreshCw class={`mr-2 h-4 w-4 ${syncing ? 'animate-spin' : ''}`} />
		{syncing ? $_('dashboard.syncing') : $_('dashboard.sync')}
	</Button>
</div>

{#if message}
	<p class="mt-2 text-sm text-muted-foreground">{message}</p>
{/if}
{#if errors.length > 0}
	<ul class="mt-1 space-y-0.5 text-xs text-destructive">
		{#each errors as e}<li>· {e}</li>{/each}
	</ul>
{/if}
