<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { auth } from '$lib/stores.svelte';
	import { LogOut, BookOpen, ShieldAlert } from '@lucide/svelte';

	let currentPath = $derived($page.url.pathname);
	let isCashier = $derived(auth.isCashier);
</script>

<header class="sticky top-0 z-40 w-full bg-white border-b-2 border-black px-4 py-3">
	<div class="max-w-4xl mx-auto flex items-center justify-between gap-3">
		<!-- Left: Title / Logo -->
		<a href="/" class="flex items-center gap-2 font-black text-lg sm:text-xl text-black tracking-tight uppercase">
			<BookOpen class="w-6 h-6 text-black shrink-0" />
			<span>BURZA</span>
		</a>

		<!-- Middle: Big sharp mode switch -->
		{#if auth.user}
			<div class="flex items-center gap-1 border-2 border-black p-0.5 bg-white">
				<a
					href="/sell"
					class="px-3 py-1.5 font-black text-xs uppercase tracking-wider transition-colors {currentPath === '/sell'
						? 'bg-black text-white'
						: 'bg-white text-black hover:bg-neutral-200'}"
				>
					PRODAT
				</a>
				<a
					href="/seeprice"
					class="px-3 py-1.5 font-black text-xs uppercase tracking-wider transition-colors {currentPath === '/seeprice'
						? 'bg-black text-white'
						: 'bg-white text-black hover:bg-neutral-200'}"
				>
					<span class="sm:hidden">CENA</span>
					<span class="hidden sm:inline">ZJISTIT CENU</span>
				</a>
			</div>
		{/if}

		<!-- Right: Cashier link & Logout -->
		<div class="flex items-center gap-2">
			{#if isCashier}
				<a
					href="/cashier"
					class="font-black text-xs uppercase tracking-wider px-3 py-1.5 border-2 border-black {currentPath.startsWith('/cashier') ? 'bg-black text-white' : 'bg-white text-black hover:bg-neutral-200'} transition-colors flex items-center gap-1.5"
				>
					<ShieldAlert class="w-4 h-4" />
					<span>POKLADNA</span>
				</a>
			{/if}

			{#if auth.user}
				<button
					onclick={async () => {
						auth.logout();
						await goto('/');
					}}
					class="font-bold text-xs uppercase tracking-wider px-2.5 py-1.5 border-2 border-black bg-white text-black hover:bg-black hover:text-white transition-colors cursor-pointer flex items-center gap-1"
					title="Odhlásit se ({auth.user.email})"
				>
					<LogOut class="w-4 h-4" />
					<span class="hidden sm:inline">ODHLÁSIT</span>
				</button>
			{/if}
		</div>
	</div>
</header>

