<script lang="ts">
	import { auth } from '$lib/stores.svelte';
	import { parseAccountInput, type AccountParseResult } from '$lib/iban';
	import { AlertCircle } from '@lucide/svelte';

	let payoutToBank = $state(true);
	let accountInput = $state('');
	let isSubmitting = $state(false);
	let submitError = $state('');

	// Reactive parse result
	let parseResult = $derived<AccountParseResult>(parseAccountInput(accountInput));

	// Validation: valid if user opted out, or if valid Czech/IBAN account is parsed
	let isInputValid = $derived.by(() => {
		if (!payoutToBank) return true;
		return parseResult.type === 'czech' || parseResult.type === 'iban';
	});

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (!isInputValid || isSubmitting) return;

		submitError = '';
		isSubmitting = true;

		try {
			const finalIban = payoutToBank && parseResult.rawIban ? parseResult.rawIban : '';

			await auth.updateProfile({
				payoutToBank,
				iban: finalIban,
				onboardingComplete: true
			});
		} catch (err: any) {
			console.error('Failed to save onboarding:', err);
			submitError = err?.message || 'Nastala chyba při ukládání. Zkuste to prosím znovu.';
			isSubmitting = false;
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		// Prevent Esc key from closing modal
		if (event.key === 'Escape') {
			event.preventDefault();
			event.stopPropagation();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Modal Overlay: unclosable, full viewport backdrop -->
<div
	class="fixed inset-0 z-50 bg-black/70 backdrop-blur-xs flex items-center justify-center p-4 overflow-y-auto"
	role="dialog"
	aria-modal="true"
	aria-labelledby="onboarding-title"
>
	<div
		class="bg-white border-2 border-black p-5 sm:p-6 max-w-sm w-full text-black shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] relative my-auto"
	>
		<!-- Header -->
		<div class="mb-5">
			<h2
				id="onboarding-title"
				class="text-xl sm:text-2xl font-black uppercase tracking-tight text-black leading-tight mb-1"
			>
				DOKONČENÍ REGISTRACE
			</h2>
			<p class="text-xs font-semibold text-neutral-600">
				Kam vám máme poslat peníze z prodaných učebnic?
			</p>
		</div>

		{#if submitError}
			<div class="mb-4 p-2.5 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold flex items-start gap-2">
				<AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
				<span>{submitError}</span>
			</div>
		{/if}

		<form onsubmit={handleSubmit} class="space-y-4">
			<!-- Account Input Field -->
			<div>
				<label for="iban" class="block text-xs font-black uppercase tracking-wider text-black mb-1.5">
					Číslo účtu / IBAN
				</label>
				<input
					type="text"
					id="iban"
					name="iban"
					autocomplete={"bank-account-number" as any}
					inputmode="text"
					autocapitalize="characters"
					spellcheck="false"
					disabled={!payoutToBank}
					required={payoutToBank}
					bind:value={accountInput}
					placeholder="2101234567/2010 nebo CZ..."
					class="w-full py-2.5 px-3 bg-white border-2 border-black text-black font-mono font-bold text-sm tracking-wide placeholder:font-sans placeholder:font-normal placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-black focus:border-black disabled:bg-neutral-100 disabled:text-neutral-400 disabled:border-neutral-300 transition-colors"
				/>

				<!-- Simple Confirmation / Error Text -->
				{#if payoutToBank}
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
				{/if}
			</div>

			<!-- Small Checkbox Under Input -->
			<div>
				<label class="flex items-center gap-2 cursor-pointer select-none text-xs font-bold text-black">
					<input
						type="checkbox"
						bind:checked={payoutToBank}
						class="w-4 h-4 accent-black rounded-none border border-black cursor-pointer shrink-0"
					/>
					<span>Zaslat peníze z prodeje na bankovní účet</span>
				</label>

				{#if !payoutToBank}
					<p class="text-xs text-neutral-600 font-medium mt-2 pl-6">
						Peníze vám budou vyplaceny v hotovosti spolu s vrácením neprodaných učebnic.
					</p>
				{/if}
			</div>

			<!-- Submit Action Button -->
			<div class="pt-2">
				<button
					type="submit"
					disabled={!isInputValid || isSubmitting}
					class="w-full py-3 px-4 bg-black text-white font-black text-xs uppercase tracking-wider border-2 border-black hover:bg-neutral-800 active:scale-98 transition-all disabled:bg-neutral-200 disabled:text-neutral-400 disabled:border-neutral-300 disabled:cursor-not-allowed cursor-pointer"
				>
					{isSubmitting ? 'UKLÁDÁM...' : 'ULOŽIT A POKRAČOVAT'}
				</button>
			</div>
		</form>
	</div>
</div>
