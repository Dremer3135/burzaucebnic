<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth, eventStore } from '$lib/stores.svelte';
	import LoginModal from '$lib/components/LoginModal.svelte';
	import { ShoppingBag, Tag, Clock, ShieldCheck } from '@lucide/svelte';

	$effect(() => {
		if (auth.user && !eventStore.isLoading) {
			if (eventStore.isSellActive()) {
				goto('/sell');
			} else if (eventStore.isBuyActive()) {
				goto('/buy');
			}
		}
	});

	function formatDate(dStr?: string) {
		if (!dStr) return 'Nespecifikováno';
		const d = new Date(dStr);
		return d.toLocaleString('cs-CZ', {
			day: 'numeric',
			month: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}
</script>

<div class="flex-1 flex flex-col items-center justify-center p-4 bg-white text-black">
	{#if !auth.user}
		<div class="w-full flex justify-center py-6">
			<LoginModal />
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

				<div class="bg-neutral-50 border-2 border-black p-4 mb-6 text-left text-xs space-y-3">
					<div class="flex items-center justify-between gap-2">
						<span class="font-bold text-neutral-600 flex items-center gap-1.5 uppercase">
							<Tag class="w-4 h-4 text-black" />
							Příjem do prodeje:
						</span>
						<span class="font-black text-black">
							{formatDate(eventStore.event.sellStart)} – {formatDate(eventStore.event.sellEnd)}
						</span>
					</div>
					<div class="flex items-center justify-between gap-2 border-t border-neutral-300 pt-2">
						<span class="font-bold text-neutral-600 flex items-center gap-1.5 uppercase">
							<ShoppingBag class="w-4 h-4 text-black" />
							Nákup učebnic:
						</span>
						<span class="font-black text-black">
							{formatDate(eventStore.event.buyStart)} – {formatDate(eventStore.event.buyEnd)}
						</span>
					</div>
				</div>
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
						href="/buy"
						class="flex items-center justify-center gap-2 py-3 px-3 bg-black text-white font-black text-xs uppercase tracking-wider hover:bg-neutral-800 transition-colors border-2 border-black"
					>
						<ShoppingBag class="w-4 h-4 text-white" />
						KOŠÍK
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
