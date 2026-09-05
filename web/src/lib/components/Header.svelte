<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { auth } from '$lib/stores.svelte';
	import { LogOut, BookOpen, ChevronDown } from '@lucide/svelte';

	let currentPath = $derived($page.url.pathname);
	let isCashier = $derived(auth.isCashier);
	let isDropdownOpen = $state(false);

	let activeModeLabel = $derived.by(() => {
		if (currentPath === '/seeprice') return 'CENA';
		if (currentPath === '/cashier') return 'POKLADNA';
		if (currentPath === '/cashier/payments') return 'PLATBY';
		return 'PRODEJ';
	});
</script>

<header class="sticky top-0 z-40 w-full bg-white border-b-2 border-black px-3 py-2 sm:px-4 sm:py-2.5">
	<div class="max-w-4xl mx-auto flex items-center justify-between gap-2">
		<!-- Left: Title / Logo -->
		<a href="/" class="flex items-center gap-1.5 font-black text-lg text-black tracking-tight uppercase shrink-0">
			<BookOpen class="w-5 h-5 text-black shrink-0 hidden sm:block" />
			<span>BURZA</span>
		</a>

		<!-- Middle: Dropdown mode switch -->
		{#if auth.user}
			<div class="relative">
				<button
					type="button"
					onclick={() => (isDropdownOpen = !isDropdownOpen)}
					class="flex items-center gap-1.5 border-2 border-black px-2.5 py-1.5 bg-white text-black font-black text-xs uppercase tracking-wider hover:bg-neutral-100 cursor-pointer active:scale-95 transition-transform"
					aria-expanded={isDropdownOpen}
					aria-haspopup="true"
				>
					<span>{activeModeLabel}</span>
					<ChevronDown class="w-3.5 h-3.5 transition-transform duration-200 {isDropdownOpen ? 'rotate-180' : ''}" />
				</button>

				{#if isDropdownOpen}
					<!-- Transparent backdrop to close dropdown on tap outside -->
					<div
						class="fixed inset-0 z-40 cursor-default"
						onclick={() => (isDropdownOpen = false)}
						role="presentation"
					></div>

					<div class="absolute left-1/2 -translate-x-1/2 top-full mt-1.5 z-50 bg-white border-2 border-black shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] py-1 min-w-[150px] flex flex-col">
						<a
							href="/sell"
							onclick={() => (isDropdownOpen = false)}
							class="px-3 py-2 text-xs font-black uppercase tracking-wider transition-colors flex items-center justify-between {currentPath === '/sell'
								? 'bg-black text-white'
								: 'text-black hover:bg-neutral-100'}"
						>
							<span>PRODEJ</span>
							{#if currentPath === '/sell'}
								<span class="text-[10px]">•</span>
							{/if}
						</a>
						<a
							href="/seeprice"
							onclick={() => (isDropdownOpen = false)}
							class="px-3 py-2 text-xs font-black uppercase tracking-wider transition-colors flex items-center justify-between {currentPath === '/seeprice'
								? 'bg-black text-white'
								: 'text-black hover:bg-neutral-100'}"
						>
							<span>CENA</span>
							{#if currentPath === '/seeprice'}
								<span class="text-[10px]">•</span>
							{/if}
						</a>

						{#if isCashier}
							<div class="my-1 border-t-2 border-black"></div>
							<a
								href="/cashier"
								onclick={() => (isDropdownOpen = false)}
								class="px-3 py-2 text-xs font-black uppercase tracking-wider transition-colors flex items-center justify-between {currentPath === '/cashier'
									? 'bg-black text-white'
									: 'text-black hover:bg-neutral-100'}"
							>
								<span>POKLADNA</span>
								{#if currentPath === '/cashier'}
									<span class="text-[10px]">•</span>
								{/if}
							</a>
							<a
								href="/cashier/payments"
								onclick={() => (isDropdownOpen = false)}
								class="px-3 py-2 text-xs font-black uppercase tracking-wider transition-colors flex items-center justify-between {currentPath === '/cashier/payments'
									? 'bg-black text-white'
									: 'text-black hover:bg-neutral-100'}"
							>
								<span>PLATBY</span>
								{#if currentPath === '/cashier/payments'}
									<span class="text-[10px]">•</span>
								{/if}
							</a>
						{/if}
					</div>
				{/if}
			</div>
		{/if}

		<!-- Right: Logout -->
		<div class="flex items-center gap-1.5 shrink-0">
			{#if auth.user}
				<button
					onclick={async () => {
						auth.logout();
						await goto('/');
					}}
					class="p-2 border-2 border-black bg-white text-black hover:bg-black hover:text-white transition-colors cursor-pointer flex items-center justify-center gap-1"
					title="Odhlásit se ({auth.user.email})"
					aria-label="Odhlásit se"
				>
					<LogOut class="w-4 h-4" />
					<span class="hidden md:inline text-xs font-bold uppercase tracking-wider">ODHLÁSIT</span>
				</button>
			{/if}
		</div>
	</div>
</header>

