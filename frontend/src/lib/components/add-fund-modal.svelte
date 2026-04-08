<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { Button, Input, Label } from '$lib/components/ui';
	import { api } from '$lib/api/client';
	import { X } from 'lucide-svelte';

	type Props = {
		open: boolean;
		defaultQuote: string;
		// When set, the modal acts as an edit form for an existing manual
		// acquisition. When undefined, it creates a new one.
		editing?: {
			id: string;
			asset: string;
			quote: string;
			quantity: string;
			unit_cost: string;
			acquired_at: string;
		};
		onClose: () => void;
		onCreated: () => void;
	};

	let props: Props = $props();

	let asset = $state('');
	let quantity = $state('');
	let unitCost = $state('');
	let quote = $state('');
	let acquiredAt = $state(new Date().toISOString().slice(0, 16));
	let submitting = $state(false);
	let error = $state<string | null>(null);

	$effect(() => {
		if (!props.open) return;
		error = null;
		if (props.editing) {
			asset = props.editing.asset;
			quantity = props.editing.quantity;
			unitCost = props.editing.unit_cost;
			quote = props.editing.quote;
			acquiredAt = new Date(props.editing.acquired_at).toISOString().slice(0, 16);
		} else {
			asset = '';
			quantity = '';
			unitCost = '';
			quote = props.defaultQuote;
			acquiredAt = new Date().toISOString().slice(0, 16);
		}
	});

	function handleKey(e: KeyboardEvent) {
		if (e.key === 'Escape') props.onClose();
	}

	async function submit(e: Event) {
		e.preventDefault();
		submitting = true;
		error = null;
		try {
			const body = {
				asset: asset.trim().toUpperCase(),
				quote: quote.trim().toUpperCase(),
				quantity: quantity.trim(),
				unit_cost: unitCost.trim(),
				acquired_at: new Date(acquiredAt).toISOString()
			};
			if (props.editing) {
				await api.updateAcquisition(props.editing.id, body);
			} else {
				await api.createAcquisition(body);
			}
			props.onCreated();
			props.onClose();
		} catch (err) {
			error = (err as Error).message;
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:window onkeydown={props.open ? handleKey : null} />

{#if props.open}
	<button
		type="button"
		class="fixed inset-0 z-50 flex cursor-default items-center justify-center bg-black/60 p-4"
		onclick={props.onClose}
		aria-label="Close"
	>
		<div
			class="w-full max-w-md cursor-auto rounded-lg border bg-card p-6 text-left shadow-xl"
			role="dialog"
			aria-modal="true"
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
		>
			<div class="mb-4 flex items-center justify-between">
				<h2 class="text-lg font-semibold">
					{props.editing ? $_('add_fund.title_edit') : $_('add_fund.title')}
				</h2>
				<span class="text-muted-foreground" aria-hidden="true">
					<X class="h-4 w-4" />
				</span>
			</div>

			<form class="space-y-3" onsubmit={submit}>
				<div>
					<Label for="asset">{$_('add_fund.asset')}</Label>
					<Input id="asset" bind:value={asset} placeholder="BTC" required />
				</div>
				<div class="grid grid-cols-2 gap-3">
					<div>
						<Label for="qty">{$_('add_fund.quantity')}</Label>
						<Input id="qty" bind:value={quantity} placeholder="0.05" required />
					</div>
					<div>
						<Label for="cost">{$_('add_fund.unit_cost')}</Label>
						<Input id="cost" bind:value={unitCost} placeholder="auto" />
						<p class="mt-1 text-[10px] text-muted-foreground">
							{$_('add_fund.unit_cost_hint')}
						</p>
					</div>
				</div>
				<div>
					<Label for="quote">{$_('add_fund.quote')}</Label>
					<Input id="quote" bind:value={quote} required />
				</div>
				<div>
					<Label for="when">{$_('add_fund.acquired_at')}</Label>
					<Input id="when" type="datetime-local" bind:value={acquiredAt} required />
				</div>

				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}

				<div class="flex justify-end gap-2 pt-2">
					<Button type="button" onclick={props.onClose}>
						{$_('common.cancel')}
					</Button>
					<Button type="submit" disabled={submitting}>
						{submitting
							? $_('common.loading')
							: props.editing
								? $_('add_fund.save')
								: $_('add_fund.submit')}
					</Button>
				</div>
			</form>
		</div>
	</button>
{/if}
