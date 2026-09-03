<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth, eventStore } from '$lib/stores.svelte';
	import LoginModal from '$lib/components/LoginModal.svelte';
	import { Calendar, ShoppingBag, Tag, Clock, ArrowRight, ShieldCheck } from '@lucide/svelte';

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

<div class="flex-1 flex flex-col items-center justify-center p-4">
	{#if !auth.user}
		<div class="w-full flex justify-center py-6">
			<LoginModal />
		</div>
	{:else}
		<div class="max-w-md w-full bg-slate-800/80 border border-slate-700/80 rounded-2xl p-6 shadow-xl text-center backdrop-blur">
			<div class="inline-flex p-3 bg-amber-500/10 text-amber-400 rounded-2xl mb-4 border border-amber-500/20">
				<Clock class="w-8 h-8" />
			</div>

			<h1 class="text-2xl font-bold text-white mb-2">Burza je momentálně uzavřena</h1>

			{#if eventStore.event}
				<p class="text-sm text-slate-300 mb-6">
					Akce <strong class="text-emerald-400">{eventStore.event.name}</strong> právě neprobíhá.
				</p>

				<div class="bg-slate-900/60 rounded-xl p-4 mb-6 text-left text-xs space-y-2.5 border border-slate-800">
					<div class="flex items-center justify-between">
						<span class="text-slate-400 flex items-center gap-1.5">
							<Tag class="w-3.5 h-3.5 text-emerald-400" />
							Příjem učebnic do prodeje:
						</span>
						<span class="font-medium text-slate-200">
							{formatDate(eventStore.event.sellStart)} – {formatDate(eventStore.event.sellEnd)}
						</span>
					</div>
					<div class="flex items-center justify-between">
						<span class="text-slate-400 flex items-center gap-1.5">
							<ShoppingBag class="w-3.5 h-3.5 text-blue-400" />
							Nákup učebnic:
						</span>
						<span class="font-medium text-slate-200">
							{formatDate(eventStore.event.buyStart)} – {formatDate(eventStore.event.buyEnd)}
						</span>
					</div>
				</div>
			{:else}
				<p class="text-sm text-slate-400 mb-6">
					Momentálně není naplánována žádná aktivní burza učebnic.
				</p>
			{/if}

			<div class="space-y-2">
				<p class="text-xs text-slate-400 mb-2">Můžete přejít přímo do jednotlivých sekcí:</p>
				<div class="grid grid-cols-2 gap-2">
					<a
						href="/sell"
						class="flex items-center justify-center gap-2 py-2.5 px-3 rounded-xl bg-slate-700/60 hover:bg-slate-700 text-slate-200 text-xs font-medium border border-slate-600/60 transition-colors"
					>
						<Tag class="w-4 h-4 text-emerald-400" />
						Moje knihy k prodeji
					</a>
					<a
						href="/buy"
						class="flex items-center justify-center gap-2 py-2.5 px-3 rounded-xl bg-slate-700/60 hover:bg-slate-700 text-slate-200 text-xs font-medium border border-slate-600/60 transition-colors"
					>
						<ShoppingBag class="w-4 h-4 text-blue-400" />
						Můj nákupní košík
					</a>
				</div>
				{#if auth.isCashier}
					<a
						href="/cashier"
						class="w-full flex items-center justify-center gap-2 py-2.5 px-3 rounded-xl bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 text-xs font-medium border border-amber-500/40 transition-colors mt-2"
					>
						<ShieldCheck class="w-4 h-4" />
						Přejít do pokladny
					</a>
				{/if}
			</div>
		</div>
	{/if}
</div>
