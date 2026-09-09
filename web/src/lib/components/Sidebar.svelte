<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores.svelte';
	import { X, LogOut, BookOpen, Banknote, ChevronRight, CheckCircle2 } from '@lucide/svelte';

	let { open = $bindable(false), onopenPayout }: { open?: boolean; onopenPayout?: () => void } = $props();

	function handleClose() {
		open = false;
	}

	async function handleLogout() {
		open = false;
		auth.logout();
		await goto('/');
	}

	function handleOpenPayout() {
		open = false;
		if (onopenPayout) {
			onopenPayout();
		}
	}

	async function handleOpenTutorial() {
		open = false;
		await goto('/tutorial');
	}

	function handleKeydown(event: KeyboardEvent) {
		if (open && event.key === 'Escape') {
			open = false;
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<!-- Backdrop -->
	<div
		class="fixed inset-0 z-50 bg-black/60 backdrop-blur-xs transition-opacity"
		onclick={handleClose}
		role="presentation"
	></div>

	<!-- Sidebar Panel -->
	<div
		class="fixed top-0 right-0 bottom-0 z-50 w-80 max-w-[85vw] bg-white border-l-4 border-black shadow-[-8px_0px_0px_0px_rgba(0,0,0,1)] flex flex-col justify-between text-black transition-transform duration-200"
		role="dialog"
		aria-modal="true"
		aria-label="Postranní menu"
	>
		<!-- Top Section -->
		<div>
			<!-- Header with Close Button -->
			<div class="p-4 border-b-2 border-black flex items-center justify-between bg-neutral-50">
				<div class="flex items-center gap-2 min-w-0">
					<div class="w-8 h-8 bg-black text-white font-black text-xs flex items-center justify-center border border-black shrink-0">
						{auth.user?.name ? auth.user.name[0].toUpperCase() : auth.user?.email ? auth.user.email[0].toUpperCase() : 'U'}
					</div>
					<div class="min-w-0">
						<p class="text-xs font-black uppercase tracking-tight text-black truncate">
							{auth.user?.name || auth.user?.email || 'Uživatel'}
						</p>
						{#if auth.isCashier}
							<span class="inline-block text-[9px] font-black uppercase tracking-wider px-1.5 py-0.5 bg-yellow-300 text-black border border-black mt-0.5">
								POKLADNÍK
							</span>
						{:else}
							<p class="text-[10px] text-neutral-500 font-mono truncate">
								{auth.user?.email}
							</p>
						{/if}
					</div>
				</div>

				<button
					onclick={handleClose}
					class="p-1.5 border-2 border-black bg-white hover:bg-neutral-200 text-black cursor-pointer transition-colors shrink-0"
					title="Zavřít menu"
					aria-label="Zavřít menu"
				>
					<X class="w-4 h-4" />
				</button>
			</div>

			<!-- Payout Status Overview Banner -->
			<div class="p-3 bg-neutral-100 border-b-2 border-black">
				<div class="text-[10px] font-black uppercase tracking-wider text-neutral-500 mb-1">
					Aktuální výplata peněz:
				</div>
				<div class="flex items-center gap-1.5 text-xs font-bold text-black">
					{#if auth.user?.payoutToBank}
						<CheckCircle2 class="w-3.5 h-3.5 text-emerald-600 shrink-0" />
						<span class="truncate">
							Na účet: <span class="font-mono text-[11px]">{auth.user.iban ? auth.user.iban.replace(/(.{4})/g, '$1 ').trim() : 'Nezadáno'}</span>
						</span>
					{:else}
						<span class="w-2 h-2 rounded-full bg-amber-500 shrink-0"></span>
						<span>V hotovosti u pokladny</span>
					{/if}
				</div>
			</div>

			<!-- Menu Items Navigation -->
			<nav class="p-3 space-y-2">
				<!-- 1. Způsob výplaty -->
				<button
					onclick={handleOpenPayout}
					class="w-full p-3 border-2 border-black bg-white hover:bg-neutral-50 active:scale-[0.99] transition-all text-left flex items-center justify-between cursor-pointer group shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]"
				>
					<div class="flex items-center gap-3 min-w-0">
						<div class="p-2 border border-black bg-neutral-100 group-hover:bg-black group-hover:text-white transition-colors shrink-0">
							<Banknote class="w-4 h-4" />
						</div>
						<div class="min-w-0">
							<p class="text-xs font-black uppercase tracking-wider text-black">
								Způsob výplaty
							</p>
							<p class="text-[11px] text-neutral-500 font-medium truncate">
								{auth.user?.payoutToBank ? 'Bankovní účet (IBAN)' : 'V hotovosti'}
							</p>
						</div>
					</div>
					<ChevronRight class="w-4 h-4 text-black shrink-0" />
				</button>

				<!-- 2. Návod k použití -->
				<button
					onclick={handleOpenTutorial}
					class="w-full p-3 border-2 border-black bg-white hover:bg-neutral-50 active:scale-[0.99] transition-all text-left flex items-center justify-between cursor-pointer group shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]"
				>
					<div class="flex items-center gap-3 min-w-0">
						<div class="p-2 border border-black bg-neutral-100 group-hover:bg-black group-hover:text-white transition-colors shrink-0">
							<BookOpen class="w-4 h-4" />
						</div>
						<div class="min-w-0">
							<p class="text-xs font-black uppercase tracking-wider text-black">
								Návod k použití
							</p>
							<p class="text-[11px] text-neutral-500 font-medium">
								Jak prodávat a nakupovat
							</p>
						</div>
					</div>
					<ChevronRight class="w-4 h-4 text-black shrink-0" />
				</button>

				<!-- 3. Odhlásit se -->
				<button
					onclick={handleLogout}
					class="w-full p-3 border-2 border-black bg-white hover:bg-red-50 hover:border-red-600 hover:text-red-700 active:scale-[0.99] transition-all text-left flex items-center justify-between cursor-pointer group shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]"
				>
					<div class="flex items-center gap-3 min-w-0">
						<div class="p-2 border border-black bg-neutral-100 group-hover:bg-red-600 group-hover:text-white group-hover:border-red-600 transition-colors shrink-0">
							<LogOut class="w-4 h-4" />
						</div>
						<div class="min-w-0">
							<p class="text-xs font-black uppercase tracking-wider">
								Odhlásit se
							</p>
							<p class="text-[11px] text-neutral-500 font-medium">
								Ukončit přihlášení
							</p>
						</div>
					</div>
					<ChevronRight class="w-4 h-4 shrink-0" />
				</button>
			</nav>
		</div>

		<!-- Footer Section -->
		<div class="p-4 border-t-2 border-black bg-neutral-50 text-[11px]">
			<div class="flex items-center gap-2 mb-2">
				<a href="/terms" onclick={handleClose} class="font-bold text-neutral-600 hover:text-black uppercase tracking-wider transition-colors">
					Podmínky
				</a>
				<span class="text-neutral-400">•</span>
				<a href="/privacy" onclick={handleClose} class="font-bold text-neutral-600 hover:text-black uppercase tracking-wider transition-colors">
					Soukromí
				</a>
			</div>
			<p class="text-neutral-400 text-[10px] uppercase tracking-wider font-mono">
				Burza učebnic by SKRAT
			</p>
		</div>
	</div>
{/if}
