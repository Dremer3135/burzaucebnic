<script lang="ts">
	import { auth } from '$lib/stores.svelte';
	import { parseAccountInput, type AccountParseResult } from '$lib/iban';
	import { Landmark, Info, CheckCircle2, AlertCircle, ArrowRight } from '@lucide/svelte';

	let payoutToBank = $state(true);
	let accountInput = $state('');
	let isSubmitting = $state(false);
	let submitError = $state('');

	// Reactive parse result
	let parseResult = $derived<AccountParseResult>(parseAccountInput(accountInput));

	// Validation
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
	class="fixed inset-0 z-50 bg-black/75 backdrop-blur-xs flex items-center justify-center p-4 overflow-y-auto"
	role="dialog"
	aria-modal="true"
	aria-labelledby="onboarding-title"
>
	<div
		class="bg-white border-2 border-black p-6 sm:p-8 max-w-md w-full text-black shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] relative my-auto animate-in fade-in zoom-in-95 duration-150"
	>
		<!-- Header -->
		<div class="mb-6">
			<div class="inline-flex items-center gap-1.5 px-2.5 py-1 bg-black text-white text-[11px] font-black uppercase tracking-wider mb-2.5">
				<Landmark class="w-3.5 h-3.5" />
				<span>Registrace prodejce</span>
			</div>
			<h2
				id="onboarding-title"
				class="text-2xl sm:text-3xl font-black uppercase tracking-tight text-black leading-tight mb-1.5"
			>
				DOKONČENÍ REGISTRACE
			</h2>
			<p class="text-xs sm:text-sm font-bold text-neutral-600">
				Kam vám máme poslat peníze z prodaných učebnic?
			</p>
		</div>

		{#if submitError}
			<div class="mb-4 p-3 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold flex items-start gap-2">
				<AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
				<span>{submitError}</span>
			</div>
		{/if}

		<form onsubmit={handleSubmit} class="space-y-5">
			<!-- Checkbox Option -->
			<div class="border-2 border-black p-3.5 bg-neutral-50 transition-colors">
				<label class="flex items-start gap-3 cursor-pointer select-none">
					<input
						type="checkbox"
						bind:checked={payoutToBank}
						class="w-5 h-5 mt-0.5 accent-black rounded-none border-2 border-black cursor-pointer shrink-0"
					/>
					<div>
						<span class="block text-sm font-black uppercase tracking-tight text-black leading-snug">
							Zaslat peníze z prodeje na bankovní účet
						</span>
						<span class="block text-xs font-semibold text-neutral-500 mt-0.5">
							{payoutToBank ? 'Peníze odešleme na váš bankovní účet.' : 'Zvolena výplata v hotovosti.'}
						</span>
					</div>
				</label>
			</div>

			<!-- Dynamic Content: Bank Account or Cash Notice -->
			{#if payoutToBank}
				<div class="space-y-2">
					<label for="iban" class="block text-xs font-black uppercase tracking-wider text-black">
						ČÍSLO ÚČTU / IBAN
					</label>

					<div class="relative">
						<!-- svelte-ignore a11y_autocomplete_valid -->
						<input
							type="text"
							id="iban"
							name="iban"
							autocomplete={"bank-account-number" as any}
							inputmode="text"
							autocapitalize="characters"
							spellcheck="false"
							required
							bind:value={accountInput}
							placeholder="2101234567/2010 nebo CZ..."
							class="w-full py-3 px-3.5 bg-white border-2 border-black text-black font-mono font-bold text-sm tracking-wide placeholder:font-sans placeholder:font-medium placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-black focus:border-black transition-all"
						/>
					</div>

					<!-- Feedback / Converted Preview -->
					{#if parseResult.type === 'czech'}
						<div class="p-2.5 bg-emerald-50 border-2 border-emerald-600 text-emerald-800 text-xs font-mono font-bold flex items-start gap-2">
							<CheckCircle2 class="w-4 h-4 text-emerald-600 shrink-0 mt-0.5" />
							<div>
								<div class="text-[10px] font-sans uppercase font-black tracking-wider text-emerald-900">
									Převedeno na IBAN:
								</div>
								<div class="text-xs sm:text-sm font-black tracking-wider">
									{parseResult.formattedIban}
								</div>
							</div>
						</div>
					{:else if parseResult.type === 'iban'}
						<div class="p-2.5 bg-emerald-50 border-2 border-emerald-600 text-emerald-800 text-xs font-mono font-bold flex items-start gap-2">
							<CheckCircle2 class="w-4 h-4 text-emerald-600 shrink-0 mt-0.5" />
							<div>
								<div class="text-[10px] font-sans uppercase font-black tracking-wider text-emerald-900">
									Platný IBAN:
								</div>
								<div class="text-xs sm:text-sm font-black tracking-wider">
									{parseResult.formattedIban}
								</div>
							</div>
						</div>
					{:else if parseResult.type === 'invalid' && accountInput.trim()}
						<div class="p-2.5 bg-amber-50 border-2 border-amber-600 text-amber-900 text-xs font-bold flex items-start gap-2">
							<AlertCircle class="w-4 h-4 text-amber-600 shrink-0 mt-0.5" />
							<span>{parseResult.error}</span>
						</div>
					{:else}
						<div class="text-[11px] font-semibold text-neutral-500">
							Zadejte české číslo účtu (např. <strong>2101234567/2010</strong>) nebo mezinárodní IBAN.
						</div>
					{/if}
				</div>
			{:else}
				<!-- Opt-out Notice -->
				<div class="p-4 bg-neutral-100 border-2 border-black text-black space-y-2">
					<div class="flex items-start gap-2.5">
						<Info class="w-5 h-5 text-black shrink-0 mt-0.5" />
						<div class="text-xs sm:text-sm font-bold leading-snug">
							Peníze vám budou vyplaceny v hotovosti spolu s vrácením neprodaných učebnic.
						</div>
					</div>
					<div class="text-[11px] font-medium text-neutral-600 pl-7.5">
						Výplata proběhne v hotovosti u pokladny na konci burzy při vyzvednutí neprodaných knih.
					</div>
				</div>
			{/if}

			<!-- Submit Action Button -->
			<div class="pt-2">
				<button
					type="submit"
					disabled={!isInputValid || isSubmitting}
					class="w-full flex items-center justify-center gap-2 py-3.5 px-4 bg-black text-white font-black text-xs sm:text-sm uppercase tracking-wider border-2 border-black hover:bg-neutral-800 active:scale-98 transition-all disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
				>
					{#if isSubmitting}
						<span class="inline-block animate-spin mr-1">⟳</span>
						<span>UKLÁDÁM...</span>
					{:else}
						<span>ULOŽIT A POKRAČOVAT</span>
						<ArrowRight class="w-4 h-4" />
					{/if}
				</button>
			</div>
		</form>
	</div>
</div>
