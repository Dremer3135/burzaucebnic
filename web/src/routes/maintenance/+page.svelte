<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth, eventStore } from '$lib/stores.svelte';
	import { Wrench, LogOut, RefreshCw } from '@lucide/svelte';

	$effect(() => {
		// If user becomes cashier, redirect to cashier
		if (auth.isCashier) {
			goto('/cashier');
			return;
		}
		// When maintenance break ends, automatically return to main page / market
		if (!eventStore.isLoading && eventStore.event && !eventStore.event.maintenance_break) {
			goto('/');
		}
	});

	async function handleLogout() {
		auth.logout();
		await goto('/');
	}
</script>

<svelte:head>
	<title>Probíhá technická přestávka | Burza učebnic</title>
</svelte:head>

<div class="flex-1 flex flex-col items-center justify-center p-4 bg-white text-black overflow-y-auto">
	<div class="max-w-md w-full bg-white border-2 border-black p-6 sm:p-8 text-center shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]">
		<!-- Icon -->
		<div class="inline-flex p-3.5 bg-neutral-100 text-black border-2 border-black mb-5">
			<Wrench class="w-8 h-8 text-black" />
		</div>

		<!-- Title -->
		<h1 class="text-2xl font-black uppercase tracking-tight text-black mb-2">
			Probíhá technická přestávka
		</h1>

		<!-- Subtitle / Explanation -->
		<p class="text-xs sm:text-sm font-bold text-neutral-700 mb-6 leading-relaxed">
			Na burze učebnic právě probíhá plánovaná technická údržba. Stránky budou brzy opět plně dostupné.
		</p>

		<!-- Live status indicator -->
		<div class="inline-flex items-center gap-2 py-2 px-3 bg-neutral-50 border border-black text-neutral-600 text-xs font-semibold mb-6">
			<RefreshCw class="w-3.5 h-3.5 animate-spin text-black shrink-0" />
			<span>Systém stránku automaticky obnoví po skončení údržby</span>
		</div>

		<!-- Actions -->
		<div class="pt-2 border-t border-neutral-200">
			{#if auth.user}
				<button
					type="button"
					onclick={handleLogout}
					class="inline-flex items-center justify-center gap-2 py-2.5 px-4 bg-white border-2 border-black text-black font-black text-xs uppercase tracking-wider hover:bg-black hover:text-white transition-colors cursor-pointer"
				>
					<LogOut class="w-3.5 h-3.5" />
					<span>Odhlásit se ({auth.user.email})</span>
				</button>
			{:else}
				<a
					href="/"
					class="inline-flex items-center justify-center gap-2 py-2.5 px-4 bg-black text-white font-black text-xs uppercase tracking-wider border-2 border-black hover:bg-neutral-800 transition-colors"
				>
					<span>Přejít na přihlášení</span>
				</a>
			{/if}
		</div>
	</div>
</div>
