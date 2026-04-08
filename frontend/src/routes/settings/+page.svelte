<script lang="ts">
	import { onMount } from 'svelte';
	import { _, locale } from 'svelte-i18n';
	import { api } from '$lib/api/client';
	import type { Settings } from '$lib/api/types';
	import {
		Button,
		Card,
		CardContent,
		CardHeader,
		CardTitle,
		Input,
		Label
	} from '$lib/components/ui';
	import { persistLocale } from '$lib/i18n';

	let settings = $state<Settings | null>(null);
	let apiKey = $state('');
	let apiSecret = $state('');
	let quoteCurrency = $state('');
	let saving = $state(false);
	let message = $state<string | null>(null);
	let error = $state<string | null>(null);

	onMount(async () => {
		try {
			settings = await api.getSettings();
			quoteCurrency = settings.quote_currency ?? '';
		} catch (err) {
			error = (err as Error).message;
		}
	});

	async function save() {
		saving = true;
		message = null;
		error = null;
		try {
			await api.updateSettings({
				binance_api_key: apiKey || undefined,
				binance_api_secret: apiSecret || undefined,
				quote_currency: quoteCurrency || undefined
			});
			apiKey = '';
			apiSecret = '';
			settings = await api.getSettings();
			message = $_('settings.saved');
		} catch (err) {
			error = (err as Error).message;
		} finally {
			saving = false;
		}
	}

	function changeLocale(value: string) {
		locale.set(value);
		persistLocale(value);
	}
</script>

<h1 class="text-2xl font-semibold">{$_('settings.title')}</h1>

<div class="mt-6 space-y-6">
	<Card>
		<CardHeader>
			<CardTitle>{$_('settings.binance_section')}</CardTitle>
		</CardHeader>
		<CardContent class="space-y-4">
			<p class="text-sm text-muted-foreground">{$_('settings.binance_section_help')}</p>

			<div class="space-y-2">
				<Label for="api-key">
					{$_('settings.api_key')}
					<span class="ml-2 text-xs text-muted-foreground">
						{settings?.binance_api_key_set
							? $_('settings.api_key_set')
							: $_('settings.api_key_unset')}
					</span>
				</Label>
				<Input id="api-key" type="password" placeholder="••••••••" bind:value={apiKey} />
			</div>

			<div class="space-y-2">
				<Label for="api-secret">
					{$_('settings.api_secret')}
					<span class="ml-2 text-xs text-muted-foreground">
						{settings?.binance_api_secret_set
							? $_('settings.api_key_set')
							: $_('settings.api_key_unset')}
					</span>
				</Label>
				<Input id="api-secret" type="password" placeholder="••••••••" bind:value={apiSecret} />
			</div>

			<div class="space-y-2">
				<Label for="quote">{$_('settings.quote_currency')}</Label>
				<Input id="quote" placeholder="USDT" bind:value={quoteCurrency} />
			</div>
		</CardContent>
	</Card>

	<Card>
		<CardHeader>
			<CardTitle>{$_('settings.language')}</CardTitle>
		</CardHeader>
		<CardContent>
			<div class="flex gap-2">
				<Button
					variant={$locale === 'en' ? 'default' : 'outline'}
					size="sm"
					onclick={() => changeLocale('en')}>English</Button
				>
				<Button
					variant={$locale === 'fr' ? 'default' : 'outline'}
					size="sm"
					onclick={() => changeLocale('fr')}>Français</Button
				>
			</div>
		</CardContent>
	</Card>

	<div class="flex items-center gap-4">
		<Button onclick={save} disabled={saving}>
			{saving ? $_('common.loading') : $_('settings.save')}
		</Button>
		{#if message}
			<span class="text-sm text-success">{message}</span>
		{/if}
		{#if error}
			<span class="text-sm text-destructive">{error}</span>
		{/if}
	</div>
</div>
