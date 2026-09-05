<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth, eventStore } from '$lib/stores.svelte';
	import LoginModal from '$lib/components/LoginModal.svelte';
	import { Tag, ScanLine, Clock, ShieldCheck, RefreshCw } from '@lucide/svelte';

	function redirectToDefaultPage() {
		if (auth.user && eventStore.isMarketActive() && eventStore.event) {
			const target = eventStore.event.defaultPage === 'seeprice' ? '/seeprice' : '/sell';
			goto(target);
		}
	}

	$effect(() => {
		if (!eventStore.isLoading) {
			redirectToDefaultPage();
		}
	});

	function handleLoginSuccess() {
		redirectToDefaultPage();
	}
</script>

<div class="flex-1 flex flex-col items-center justify-center p-4 bg-white text-black overflow-y-auto">
	{#if eventStore.isLoading}
		<div class="flex flex-col items-center justify-center py-16 gap-3">
			<RefreshCw class="w-8 h-8 animate-spin text-black" />
			<span class="text-xs font-black uppercase text-neutral-400">Načítám stav burzy...</span>
		</div>
	{:else if !auth.user}
		<div class="w-full flex justify-center py-6">
			<LoginModal onsuccess={handleLoginSuccess} />
		</div>
	{:else}
		<div class="max-w-md w-full bg-white border-2 border-black p-6 text-center shadow-none">
			<div class="inline-flex p-3 bg-neutral-100 text-black border-2 border-black mb-4">
				<Clock class="w-8 h-8" />
			</div>

			<h1 class="text-2xl font-black uppercase tracking-tight text-black mb-2">
				Burza je momentálně uzavřena
			</h1>

			{#if eventStore.event}
				<p class="text-sm font-semibold text-neutral-700 mb-6">
					Akce <strong class="text-black uppercase font-black">{eventStore.event.name}</strong> právě neprobíhá.
				</p>
			{:else}
				<p class="text-sm font-bold text-neutral-500 mb-6 uppercase">
					Momentálně není naplánována žádná aktivní burza učebnic.
				</p>
			{/if}

			<div class="space-y-3">
				<p class="text-xs font-black uppercase tracking-wider text-neutral-500">Přejít do sekce:</p>
				<div class="grid grid-cols-2 gap-2">
					<a
						href="/sell"
						class="flex items-center justify-center gap-2 py-3 px-3 bg-black text-white font-black text-xs uppercase tracking-wider hover:bg-neutral-800 transition-colors border-2 border-black"
					>
						<Tag class="w-4 h-4 text-white" />
						PRODEJ
					</a>
					<a
						href="/seeprice"
						class="flex items-center justify-center gap-2 py-3 px-3 bg-black text-white font-black text-xs uppercase tracking-wider hover:bg-neutral-800 transition-colors border-2 border-black"
					>
						<ScanLine class="w-4 h-4 text-white" />
						ZJISTIT CENU
					</a>
				</div>
				{#if auth.isCashier}
					<a
						href="/cashier"
						class="w-full flex items-center justify-center gap-2 py-3 px-3 bg-neutral-100 hover:bg-neutral-200 text-black font-black text-xs uppercase tracking-wider border-2 border-black transition-colors mt-2"
					>
						<ShieldCheck class="w-4 h-4 text-black" />
						POKLADNA
					</a>
				{/if}
			</div>
		</div>
	{/if}
</div>
