<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, cashierPayments } from '$lib/stores.svelte';
	import { pb } from '$lib/pocketbase';
	import {
		Receipt,
		Scan,
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

	$effect(() => {
		if (auth.user && !auth.isCashier) {
			goto('/');
		}
	});

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

<div class="flex-1 max-w-4xl w-full mx-auto p-4 flex flex-col pb-24 bg-white text-black">
	<!-- Top Navigation Bar between Scanner and Payments -->
	<div class="flex items-center justify-between bg-white border-2 border-black p-1 mb-6 text-black">
		<a
			href="/cashier"
			class="flex-1 py-2.5 px-3 text-center text-xs font-black uppercase tracking-wider text-black hover:bg-neutral-100 transition-all flex items-center justify-center gap-1.5"
		>
			<Scan class="w-4 h-4" />
			POKLADNÍ SKENER
		</a>
		<a
			href="/cashier/payments"
			class="flex-1 py-2.5 px-3 text-center text-xs font-black uppercase tracking-wider transition-all bg-black text-white flex items-center justify-center gap-1.5"
		>
			<Receipt class="w-4 h-4" />
			BANKOVNÍ PLATBY
		</a>
	</div>

	<!-- Page Header -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-6 border-b-2 border-black pb-3">
		<div>
			<h1 class="text-2xl font-black uppercase tracking-tight text-black">OVĚŘENÍ BANKOVNÍCH PLATEB</h1>
			<p class="text-xs font-bold text-neutral-600 uppercase">
				Párování plateb podle Variabilního symbolu (VS) s vaším bankovním výpisem
			</p>
		</div>

		<button
			onclick={() => cashierPayments.refresh()}
			class="self-start sm:self-auto p-2.5 bg-white text-black hover:bg-neutral-100 border-2 border-black transition-colors flex items-center gap-1.5 text-xs font-black uppercase cursor-pointer"
			title="Obnovit"
		>
			<RefreshCw class="w-4 h-4 {cashierPayments.isLoading ? 'animate-spin' : ''}" />
			<span>AKTUALIZOVAT</span>
		</button>
	</div>

	{#if errorMessage}
		<div class="mb-4 p-3 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold flex items-center gap-2">
			<AlertCircle class="w-4 h-4 shrink-0 text-red-600" />
			<span>{errorMessage}</span>
		</div>
	{/if}

	<!-- Search & Filter Controls -->
	<div class="flex flex-col sm:flex-row gap-3 items-stretch sm:items-center justify-between mb-4">
		<!-- Tabs -->
		<div class="flex border-2 border-black bg-white p-1 text-xs">
			<button
				onclick={() => (activeTab = 'pending')}
				class="flex items-center gap-1.5 px-3 py-2 transition-all cursor-pointer font-black uppercase {activeTab === 'pending'
					? 'bg-black text-white'
					: 'text-black hover:bg-neutral-100'}"
			>
				<Clock class="w-4 h-4" />
				<span>ČEKAJÍCÍ NA PŘIPSÁNÍ</span>
				{#if pendingCount > 0}
					<span class="ml-1 px-1.5 py-0.5 bg-white text-black font-black text-[10px] border border-black">
						{pendingCount}
					</span>
				{/if}
			</button>

			<button
				onclick={() => (activeTab = 'completed')}
				class="flex items-center gap-1.5 px-3 py-2 transition-all cursor-pointer font-black uppercase {activeTab === 'completed'
					? 'bg-black text-white'
					: 'text-black hover:bg-neutral-100'}"
			>
				<CheckCircle2 class="w-4 h-4" />
				<span>DOKONČENÉ</span>
			</button>
		</div>

		<!-- Search Input -->
		<div class="relative min-w-[240px]">
			<Search class="w-4 h-4 text-black absolute left-3 top-3" />
			<input
				type="text"
				bind:value={searchQuery}
				placeholder="HLEDAT VS, JMÉNO..."
				class="w-full bg-white border-2 border-black pl-9 pr-3 py-2 text-xs font-black uppercase text-black focus:outline-none"
			/>
		</div>
	</div>

	<!-- Payments List -->
	{#if cashierPayments.isLoading && cashierPayments.payments.length === 0}
		<div class="flex-1 flex items-center justify-center py-20">
			<RefreshCw class="w-8 h-8 animate-spin text-black" />
		</div>
	{:else if filteredPayments.length === 0}
		<div class="flex-1 flex flex-col items-center justify-center p-8 bg-neutral-50 border-2 border-dashed border-black text-center my-6">
			<Receipt class="w-10 h-10 text-neutral-400 mb-2" />
			<h3 class="text-base font-black uppercase tracking-tight text-black">Žádné platby v této kategorii</h3>
			<p class="text-xs font-bold text-neutral-600 uppercase mt-1">
				{activeTab === 'pending'
					? 'Všechny platby jsou vyřízeny.'
					: 'Zatím nebyly potvrzeny žádné platby.'}
			</p>
		</div>
	{:else}
		<div class="space-y-3 flex-1">
			{#each filteredPayments as payment (payment.id)}
				<div class="bg-white border-2 border-black p-4 text-black transition-all">
					<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b-2 border-black">
						<div class="flex items-center gap-3">
							<!-- Variable Symbol -->
							<div class="bg-neutral-100 border-2 border-black px-3 py-1.5 text-center">
								<span class="text-[10px] uppercase font-black text-neutral-600 block">VS</span>
								<span class="text-xl font-black text-black font-mono tracking-wide">
									{payment.variableSymbol}
								</span>
							</div>

							<div>
								<div class="flex items-center gap-2">
									<span class="text-sm font-black uppercase text-black">
										{payment.expand?.buyer?.name || 'Kupující'}
									</span>
									<span class="text-xs font-mono font-bold text-neutral-600">
										({payment.expand?.buyer?.email})
									</span>
								</div>
								<div class="text-xs font-bold text-neutral-600 mt-0.5">
									{formatDate(payment.created)} • ID: <code class="font-mono text-black">{payment.id}</code>
								</div>
							</div>
						</div>

						<div class="text-right">
							<div class="text-2xl font-black text-black">
								{payment.totalAmount} Kč
							</div>
							<span class="text-[10px] uppercase font-black px-2 py-0.5 border-2 border-black {payment.method === 'cash' ? 'bg-neutral-200' : 'bg-white'}">
								{payment.method === 'cash' ? 'HOTOVOST' : 'QR PLATBA'}
							</span>
						</div>
					</div>

					<!-- Books included -->
					<div class="pt-3 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
						<div class="text-xs font-bold text-neutral-700 flex items-center gap-1.5 flex-wrap">
							<BookOpen class="w-4 h-4 text-black shrink-0" />
							<span class="uppercase">Položky ({payment.books.length}):</span>
							{#if payment.expand?.books}
								{#each payment.expand.books as b}
									<span class="bg-neutral-100 px-2 py-1 text-black font-mono font-black text-xs border border-black uppercase">
										{b.code} ({b.price} Kč)
									</span>
								{/each}
							{:else}
								<span>{payment.books.length} knih</span>
							{/if}
						</div>

						<!-- Action for pending payments -->
						{#if payment.status === 'pending'}
							<button
								onclick={() => handleConfirmPayment(payment)}
								disabled={confirmingPaymentId === payment.id}
								class="py-2.5 px-4 bg-black text-white hover:bg-neutral-800 active:bg-neutral-900 font-black text-xs uppercase tracking-wider border-2 border-black transition-all flex items-center justify-center gap-1.5 cursor-pointer shrink-0 disabled:opacity-50"
							>
								{#if confirmingPaymentId === payment.id}
									<RefreshCw class="w-4 h-4 animate-spin" />
									<span>POTVRZUJI...</span>
								{:else}
									<Check class="w-4 h-4" />
									<span>POTVRDIT PŘIPSÁNÍ</span>
								{/if}
							</button>
						{:else}
							<div class="flex items-center gap-1 text-black text-xs font-black uppercase">
								<CheckCircle2 class="w-4 h-4 text-black" />
								<span>VYŘÍZENO A PŘEDÁNO</span>
							</div>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
