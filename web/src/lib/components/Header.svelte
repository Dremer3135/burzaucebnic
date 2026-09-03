<script lang="ts">
	import { page } from '$app/stores';
	import { auth } from '$lib/stores.svelte';
	import { LogOut, BookOpen, ShieldAlert } from '@lucide/svelte';

	let currentPath = $derived($page.url.pathname);
	let isCashier = $derived(auth.isCashier);
</script>

<header class="sticky top-0 z-40 w-full bg-slate-900/90 backdrop-blur border-b border-slate-800 px-4 py-2.5">
	<div class="max-w-4xl mx-auto flex items-center justify-between gap-2">
		<!-- Left: Title / Logo -->
		<a href="/" class="flex items-center gap-2 font-bold text-base sm:text-lg text-emerald-400 tracking-tight">
			<BookOpen class="w-5 h-5 text-emerald-400 shrink-0" />
			<span>Burza Učebnic</span>
		</a>

		<!-- Middle: Small subtle mode switch button -->
		{#if auth.user}
			<div class="flex items-center bg-slate-800/80 p-0.5 rounded-lg border border-slate-700/60 text-xs">
				<a
					href="/buy"
					class="px-2 py-1 rounded-md transition-colors {currentPath === '/buy'
						? 'bg-emerald-600 text-white font-medium shadow-sm'
						: 'text-slate-400 hover:text-slate-200'}"
				>
					Koupit
				</a>
				<a
					href="/sell"
					class="px-2 py-1 rounded-md transition-colors {currentPath === '/sell'
						? 'bg-emerald-600 text-white font-medium shadow-sm'
						: 'text-slate-400 hover:text-slate-200'}"
				>
					Prodat
				</a>
			</div>
		{/if}

		<!-- Right: Cashier link & User profile/logout -->
		<div class="flex items-center gap-2">
			{#if isCashier}
				<a
					href="/cashier"
					class="flex items-center gap-1 text-xs px-2 py-1 rounded bg-amber-500/20 text-amber-300 border border-amber-500/30 hover:bg-amber-500/30 transition-colors"
					title="Pokladní zóna"
				>
					<ShieldAlert class="w-3.5 h-3.5" />
					<span class="hidden sm:inline">Pokladna</span>
				</a>
			{/if}

			{#if auth.user}
				<button
					onclick={() => auth.logout()}
					class="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-slate-800 transition-colors cursor-pointer"
					title="Odhlásit se ({auth.user.email})"
				>
					<LogOut class="w-4 h-4" />
				</button>
			{/if}
		</div>
	</div>
</header>

