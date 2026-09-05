<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { auth, cashierPayments } from '$lib/stores.svelte';
	import { pb } from '$lib/pocketbase';
	import {
		Receipt,
		CheckCircle2,
		Clock,
		Search,
		BookOpen,
		RefreshCw,
		Check,
		AlertCircle
	} from '@lucide/svelte';
	import type { Payment } from '$lib/types';

	let activeTab = $state<'pending' | 'completed'>('pending');
	let searchQuery = $state('');
	let confirmingPaymentId = $state<string | null>(null);
	let errorMessage = $state('');



	onMount(() => {
		if (auth.isCashier) {
			cashierPayments.init();
		}
	});

	onDestroy(() => {
		cashierPayments.cleanup();
	});

	let filteredPayments = $derived(
		cashierPayments.payments.filter((p) => {
			const matchesTab = p.status === activeTab;
			if (!matchesTab) return false;
			if (!searchQuery.trim()) return true;

			const q = searchQuery.toLowerCase().trim();
			const matchVS = String(p.variableSymbol).includes(q);
			const matchId = p.id.toLowerCase().includes(q);
			const matchBuyer =
				p.expand?.buyer?.name?.toLowerCase().includes(q) ||
				p.expand?.buyer?.email?.toLowerCase().includes(q);
			return matchVS || matchId || matchBuyer;
		})
	);

	let pendingCount = $derived(
		cashierPayments.payments.filter((p) => p.status === 'pending').length
	);

	async function handleConfirmPayment(payment: Payment) {
		confirmingPaymentId = payment.id;
		errorMessage = '';
		try {
			await pb.send('/api/cashier/confirm-payment', {
				method: 'POST',
				body: { paymentId: payment.id }
			});
		} catch (err: any) {
			console.error('Failed to confirm payment', err);
			errorMessage = err?.message || 'Chyba při potvrzení platby.';
		} finally {
			confirmingPaymentId = null;
		}
	}

	function formatDate(dStr: string) {
		return new Date(dStr).toLocaleString('cs-CZ', {
			day: 'numeric',
			month: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}
</script>

<div class="flex-1 max-w-4xl w-full mx-auto p-3 sm:p-4 flex flex-col pb-24 bg-white text-black overflow-y-auto">
	<!-- Page Header -->
	<div class="flex items-center justify-between mb-4 border-b-2 border-black pb-3">
		<h1 class="text-lg sm:text-xl font-black uppercase tracking-tight text-black">
			Bankovní platby
		</h1>

		<button
			onclick={() => cashierPayments.refresh()}
			class="py-1.5 px-3 bg-white text-black hover:bg-neutral-100 border-2 border-black transition-colors cursor-pointer flex items-center gap-1.5 text-xs font-black uppercase"
			title="Aktualizovat platby"
		>
			<RefreshCw class="w-3.5 h-3.5 {cashierPayments.isLoading ? 'animate-spin' : ''}" />
			<span>OBNOVIT</span>
		</button>
	</div>

	{#if errorMessage}
		<div class="mb-3 p-2.5 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold flex items-center gap-2">
			<AlertCircle class="w-4 h-4 shrink-0 text-red-600" />
			<span>{errorMessage}</span>
		</div>
	{/if}

	<!-- Search & Filter Controls -->
	<div class="flex flex-col sm:flex-row gap-2 items-stretch sm:items-center justify-between mb-3">
		<!-- Tabs -->
		<div class="flex border-2 border-black bg-white p-0.5 text-xs">
			<button
				onclick={() => (activeTab = 'pending')}
				class="flex-1 sm:flex-initial flex items-center justify-center gap-1.5 px-3 py-1.5 transition-all cursor-pointer font-black uppercase {activeTab === 'pending'
					? 'bg-black text-white'
					: 'text-black hover:bg-neutral-100'}"
			>
				<Clock class="w-3.5 h-3.5" />
				<span>ČEKAJÍCÍ</span>
				{#if pendingCount > 0}
					<span class="ml-1 px-1.5 py-0.2 {activeTab === 'pending' ? 'bg-white text-black' : 'bg-black text-white'} font-black text-[10px] border border-black">
						{pendingCount}
					</span>
				{/if}
			</button>

			<button
				onclick={() => (activeTab = 'completed')}
				class="flex-1 sm:flex-initial flex items-center justify-center gap-1.5 px-3 py-1.5 transition-all cursor-pointer font-black uppercase {activeTab === 'completed'
					? 'bg-black text-white'
					: 'text-black hover:bg-neutral-100'}"
			>
				<CheckCircle2 class="w-3.5 h-3.5" />
				<span>HOTOVO</span>
			</button>
		</div>

		<!-- Search Input -->
		<div class="relative flex-1 sm:max-w-xs">
			<Search class="w-3.5 h-3.5 text-black absolute left-2.5 top-2.5" />
			<input
				type="text"
				bind:value={searchQuery}
				placeholder="Hledat VS, jméno..."
				class="w-full bg-white border-2 border-black pl-8 pr-3 py-1.5 text-xs font-black uppercase text-black focus:outline-none"
			/>
		</div>
	</div>

	<!-- Payments List -->
	{#if cashierPayments.isLoading && cashierPayments.payments.length === 0}
		<div class="flex-1 flex items-center justify-center py-16">
			<RefreshCw class="w-8 h-8 animate-spin text-black" />
		</div>
	{:else if filteredPayments.length === 0}
		<div class="flex-1 flex flex-col items-center justify-center py-12 text-center">
			<Receipt class="w-8 h-8 text-neutral-300 mb-2" />
			<p class="text-xs font-bold uppercase text-neutral-400">Žádné platby</p>
		</div>
	{:else}
		<div class="space-y-2.5 flex-1">
			{#each filteredPayments as payment (payment.id)}
				<div class="bg-white border-2 border-black p-3 text-black transition-all">
					<div class="flex items-center justify-between gap-2 pb-2.5 border-b border-black">
						<div class="flex items-center gap-2.5 min-w-0">
							<!-- Variable Symbol -->
							<div class="bg-neutral-100 border-2 border-black px-2 py-1 text-center shrink-0">
								<span class="text-[9px] uppercase font-black text-neutral-500 block leading-tight">VS</span>
								<span class="text-base font-black text-black font-mono tracking-wide leading-tight">
									{payment.variableSymbol}
								</span>
							</div>

							<div class="min-w-0">
								<div class="text-xs font-black uppercase text-black truncate">
									{payment.expand?.buyer?.name || 'Kupující'}
								</div>
								<div class="text-[11px] font-mono text-neutral-500 truncate">
									{payment.expand?.buyer?.email || payment.id}
								</div>
							</div>
						</div>

						<div class="text-right shrink-0">
							<div class="text-lg font-black text-black leading-tight">
								{payment.totalAmount} Kč
							</div>
							<span class="text-[9px] uppercase font-black px-1.5 py-0.5 border border-black {payment.method === 'cash' ? 'bg-neutral-200' : 'bg-white'}">
								{payment.method === 'cash' ? 'HOTOVOST' : 'QR PLATBA'}
							</span>
						</div>
					</div>

					<!-- Books included & Action -->
					<div class="pt-2.5 flex items-center justify-between gap-2">
						<div class="text-xs font-bold text-neutral-600 flex items-center gap-1.5 truncate">
							<BookOpen class="w-3.5 h-3.5 text-black shrink-0" />
							<span class="uppercase text-[11px]">{payment.books.length} {payment.books.length === 1 ? 'kniha' : payment.books.length >= 2 && payment.books.length <= 4 ? 'knihy' : 'knih'}</span>
						</div>

						<!-- Action for pending payments -->
						{#if payment.status === 'pending'}
							<button
								onclick={() => handleConfirmPayment(payment)}
								disabled={confirmingPaymentId === payment.id}
								class="py-1.5 px-3 bg-black text-white hover:bg-neutral-800 active:bg-neutral-900 font-black text-xs uppercase tracking-wider border-2 border-black transition-all flex items-center justify-center gap-1 cursor-pointer shrink-0 disabled:opacity-50"
							>
								{#if confirmingPaymentId === payment.id}
									<RefreshCw class="w-3.5 h-3.5 animate-spin" />
									<span>POTVRZUJI...</span>
								{:else}
									<Check class="w-3.5 h-3.5" />
									<span>POTVRDIT</span>
								{/if}
							</button>
						{:else}
							<div class="flex items-center gap-1 text-black text-xs font-black uppercase shrink-0">
								<CheckCircle2 class="w-3.5 h-3.5 text-emerald-600" />
								<span>VYŘÍZENO</span>
							</div>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
