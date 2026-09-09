<script lang="ts">
	import { auth } from '$lib/stores.svelte';
	import { parseAccountInput, type AccountParseResult } from '$lib/iban';
	import { AlertCircle, CheckCircle2, X } from '@lucide/svelte';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	let payoutToBank = $state(true);
	let accountInput = $state('');
	let isSubmitting = $state(false);
	let submitError = $state('');
	let successMessage = $state('');

	$effect(() => {
		if (open && auth.user) {
			payoutToBank = auth.user.payoutToBank ?? true;
			accountInput = auth.user.iban || '';
			submitError = '';
			successMessage = '';
		}
	});

	// Reactive parse result
	let parseResult = $derived<AccountParseResult>(parseAccountInput(accountInput));

	// Validation: valid if user opted for cash, or if valid Czech/IBAN account is parsed
	let isInputValid = $derived.by(() => {
		if (!payoutToBank) return true;
		return parseResult.type === 'czech' || parseResult.type === 'iban';
	});

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (!isInputValid || isSubmitting) return;

		submitError = '';
		successMessage = '';
		isSubmitting = true;

		try {
			const finalIban = payoutToBank && parseResult.rawIban ? parseResult.rawIban : '';

			await auth.updateProfile({
				payoutToBank,
				iban: finalIban
			});

			successMessage = 'Způsob výplaty byl úspěšně uložen.';
			setTimeout(() => {
				open = false;
				successMessage = '';
			}, 800);
		} catch (err: any) {
			console.error('Failed to save payout settings:', err);
			submitError = err?.message || 'Nastala chyba při ukládání. Zkuste to prosím znovu.';
		} finally {
			isSubmitting = false;
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (open && event.key === 'Escape') {
			open = false;
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<!-- Modal Overlay -->
	<div
		class="fixed inset-0 z-50 bg-black/70 backdrop-blur-xs flex items-center justify-center p-4 overflow-y-auto"
		role="dialog"
		aria-modal="true"
		aria-labelledby="payout-modal-title"
	>
		<!-- Backdrop click closer -->
		<div
			class="fixed inset-0 cursor-default"
			onclick={() => (open = false)}
			role="presentation"
		></div>

		<div
			class="bg-white border-2 border-black p-5 sm:p-6 max-w-sm w-full text-black shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] relative my-auto z-10"
		>
			<!-- Close Button -->
			<button
				onclick={() => (open = false)}
				class="absolute top-4 right-4 p-1.5 border-2 border-black bg-white text-black hover:bg-neutral-100 cursor-pointer transition-colors"
				title="Zavřít"
				aria-label="Zavřít"
			>
				<X class="w-4 h-4" />
			</button>

			<!-- Header -->
			<div class="mb-5 pr-8">
				<h2
					id="payout-modal-title"
					class="text-xl sm:text-2xl font-black uppercase tracking-tight text-black leading-tight mb-1"
				>
					ZPŮSOB VÝPLATY
				</h2>
				<p class="text-xs font-semibold text-neutral-600">
					Jak chcete vyplatit peníze z prodaných učebnic?
				</p>
			</div>

			{#if submitError}
				<div class="mb-4 p-2.5 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold flex items-start gap-2">
					<AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
					<span>{submitError}</span>
				</div>
			{/if}

			{#if successMessage}
				<div class="mb-4 p-2.5 bg-emerald-50 border-2 border-emerald-600 text-emerald-800 text-xs font-bold flex items-center gap-2">
					<CheckCircle2 class="w-4 h-4 shrink-0 text-emerald-600" />
					<span>{successMessage}</span>
				</div>
			{/if}

			<form onsubmit={handleSubmit} class="space-y-4">
				<!-- Method choice -->
				<div class="space-y-2">
					<label
						class="flex items-start gap-2.5 p-2.5 border-2 border-black cursor-pointer transition-colors {payoutToBank ? 'bg-neutral-50' : 'hover:bg-neutral-50'}"
					>
						<input
							type="radio"
							name="payoutMethod"
							checked={payoutToBank}
							onchange={() => (payoutToBank = true)}
							class="w-4 h-4 accent-black mt-0.5 cursor-pointer shrink-0"
						/>
						<div>
							<span class="block text-xs font-black uppercase tracking-wide text-black">
								Zaslat na bankovní účet
							</span>
							<span class="block text-[11px] text-neutral-600 font-medium">
								Peníze vám zašleme automaticky po skončení burzy.
							</span>
						</div>
					</label>

					<label
						class="flex items-start gap-2.5 p-2.5 border-2 border-black cursor-pointer transition-colors {!payoutToBank ? 'bg-neutral-50' : 'hover:bg-neutral-50'}"
					>
						<input
							type="radio"
							name="payoutMethod"
							checked={!payoutToBank}
							onchange={() => (payoutToBank = false)}
							class="w-4 h-4 accent-black mt-0.5 cursor-pointer shrink-0"
						/>
						<div>
							<span class="block text-xs font-black uppercase tracking-wide text-black">
								Vyplatit v hotovosti
							</span>
							<span class="block text-[11px] text-neutral-600 font-medium">
								Peníze obdržíte u pokladny při vrácení neprodaných učebnic.
							</span>
						</div>
					</label>
				</div>

				<!-- Account Input Field (only if bank payout) -->
				{#if payoutToBank}
					<div class="pt-1">
						<label for="payout-iban" class="block text-xs font-black uppercase tracking-wider text-black mb-1.5">
							Číslo účtu / IBAN
						</label>
						<input
							type="text"
							id="payout-iban"
							name="iban"
							autocomplete={"bank-account-number" as any}
							inputmode="text"
							autocapitalize="characters"
							spellcheck="false"
							required={payoutToBank}
							bind:value={accountInput}
							placeholder="2101234567/2010 nebo CZ..."
							class="w-full py-2.5 px-3 bg-white border-2 border-black text-black font-mono font-bold text-sm tracking-wide placeholder:font-sans placeholder:font-normal placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-black focus:border-black transition-colors"
						/>

						{#if parseResult.type === 'czech' || parseResult.type === 'iban'}
							<p class="text-xs font-bold text-emerald-700 mt-1.5 flex items-center gap-1.5">
								<span>✓</span>
								<span>IBAN: {parseResult.formattedIban}</span>
							</p>
						{:else if parseResult.type === 'invalid' && accountInput.trim()}
							<p class="text-xs font-semibold text-red-600 mt-1.5">
								{parseResult.error}
							</p>
						{/if}
					</div>
				{/if}

				<!-- Action Buttons -->
				<div class="pt-3 flex items-center gap-2">
					<button
						type="button"
						onclick={() => (open = false)}
						class="flex-1 py-2.5 px-3 bg-white text-black font-black text-xs uppercase tracking-wider border-2 border-black hover:bg-neutral-100 cursor-pointer transition-colors"
					>
						ZRUŠIT
					</button>

					<button
						type="submit"
						disabled={!isInputValid || isSubmitting}
						class="flex-1 py-2.5 px-3 bg-black text-white font-black text-xs uppercase tracking-wider border-2 border-black hover:bg-neutral-800 active:scale-98 transition-all disabled:bg-neutral-200 disabled:text-neutral-400 disabled:border-neutral-300 disabled:cursor-not-allowed cursor-pointer"
					>
						{isSubmitting ? 'UKLÁDÁM...' : 'ULOŽIT ZMĚNY'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}
