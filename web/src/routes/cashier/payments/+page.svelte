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
		User,
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
			// Live SSE will update payments automatically!
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

<div class="flex-1 max-w-4xl w-full mx-auto p-4 flex flex-col pb-20">
	<!-- Top Navigation Bar between Scanner and Payments -->
	<div class="flex items-center justify-between bg-slate-800/80 p-1.5 rounded-2xl border border-slate-700/80 mb-6 backdrop-blur">
		<a
			href="/cashier"
			class="flex-1 py-2 px-3 rounded-xl text-center text-xs font-semibold text-slate-400 hover:text-slate-200 transition-all flex items-center justify-center gap-1.5"
		>
			<Scan class="w-4 h-4" />
			Pokladní skener
		</a>
		<a
			href="/cashier/payments"
			class="flex-1 py-2 px-3 rounded-xl text-center text-xs font-bold transition-all bg-emerald-600 text-white shadow-sm flex items-center justify-center gap-1.5"
		>
			<Receipt class="w-4 h-4" />
			Bankovní platby
		</a>
	</div>

	<!-- Page Header -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-6">
		<div>
			<h1 class="text-xl sm:text-2xl font-bold text-white tracking-tight">Ověření bankovních plateb</h1>
			<p class="text-xs text-slate-400">
				Párování plateb podle Variabilního symbolu (VS) s vaším bankovním výpisem
			</p>
		</div>

		<button
			onclick={() => cashierPayments.refresh()}
			class="self-start sm:self-auto p-2 rounded-xl bg-slate-800 text-slate-300 hover:text-white border border-slate-700 transition-colors flex items-center gap-1.5 text-xs"
			title="Obnovit"
		>
			<RefreshCw class="w-4 h-4 {cashierPayments.isLoading ? 'animate-spin text-emerald-400' : ''}" />
			<span>Aktualizovat</span>
		</button>
	</div>

	{#if errorMessage}
		<div class="mb-4 p-3 bg-rose-500/15 border border-rose-500/30 text-rose-300 text-xs rounded-xl flex items-center gap-2">
			<AlertCircle class="w-4 h-4 shrink-0" />
			<span>{errorMessage}</span>
		</div>
	{/if}

	<!-- Search & Filter Controls -->
	<div class="flex flex-col sm:flex-row gap-3 items-stretch sm:items-center justify-between mb-4">
		<!-- Tabs -->
		<div class="flex bg-slate-800/80 p-1 rounded-xl border border-slate-700/60 text-xs">
			<button
				onclick={() => (activeTab = 'pending')}
				class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg transition-all cursor-pointer {activeTab === 'pending'
					? 'bg-amber-500/20 text-amber-300 font-bold border border-amber-500/40'
					: 'text-slate-400 hover:text-slate-200'}"
			>
				<Clock class="w-3.5 h-3.5" />
				<span>Čekající na připsání</span>
				{#if pendingCount > 0}
					<span class="ml-1 px-1.5 py-0.2 bg-amber-500 text-slate-950 font-bold rounded-full text-[10px]">
						{pendingCount}
					</span>
				{/if}
			</button>

			<button
				onclick={() => (activeTab = 'completed')}
				class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg transition-all cursor-pointer {activeTab === 'completed'
					? 'bg-emerald-500/20 text-emerald-300 font-bold border border-emerald-500/40'
					: 'text-slate-400 hover:text-slate-200'}"
			>
				<CheckCircle2 class="w-3.5 h-3.5" />
				<span>Dokončené</span>
			</button>
		</div>

		<!-- Search Input -->
		<div class="relative min-w-[200px]">
			<Search class="w-4 h-4 text-slate-400 absolute left-3 top-2.5" />
			<input
				type="text"
				bind:value={searchQuery}
				placeholder="Hledat VS, jméno, ID..."
				class="w-full bg-slate-800/90 border border-slate-700/80 rounded-xl pl-9 pr-3 py-1.5 text-xs text-white focus:outline-none focus:border-emerald-500"
			/>
		</div>
	</div>

	<!-- Payments List -->
	{#if cashierPayments.isLoading && cashierPayments.payments.length === 0}
		<div class="flex-1 flex items-center justify-center py-20">
			<RefreshCw class="w-8 h-8 animate-spin text-emerald-400" />
		</div>
	{:else if filteredPayments.length === 0}
		<div class="flex-1 flex flex-col items-center justify-center p-8 bg-slate-800/40 border border-dashed border-slate-700 rounded-3xl text-center my-6">
			<Receipt class="w-10 h-10 text-slate-500 mb-2" />
			<h3 class="text-sm font-semibold text-white">Žádné platby v této kategorii</h3>
			<p class="text-xs text-slate-400 mt-1">
				{activeTab === 'pending'
					? 'Všechny platby jsou vyřízeny.'
					: 'Zatím nebyly potvrzeny žádné platby.'}
			</p>
		</div>
	{:else}
		<div class="space-y-3 flex-1">
			{#each filteredPayments as payment (payment.id)}
				<div class="bg-slate-800/80 border border-slate-700/80 rounded-2xl p-4 shadow-md backdrop-blur transition-all">
					<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-700/60">
						<div class="flex items-center gap-3">
							<!-- Prominent Variable Symbol -->
							<div class="bg-slate-900 border border-slate-700 rounded-xl px-3 py-1 text-center">
								<span class="text-[9px] uppercase tracking-wider text-slate-400 block font-semibold">VS</span>
								<span class="text-base font-black text-amber-400 font-mono tracking-wide">
									{payment.variableSymbol}
								</span>
							</div>

							<div>
								<div class="flex items-center gap-2">
									<span class="text-sm font-bold text-white">
										{payment.expand?.buyer?.name || 'Kupující'}
									</span>
									<span class="text-[11px] font-mono text-slate-400">
										({payment.expand?.buyer?.email})
									</span>
								</div>
								<div class="text-[11px] text-slate-400 mt-0.5">
									{formatDate(payment.created)} • Zpráva pro příjemce: <code class="text-slate-300 font-mono">{payment.id}</code>
								</div>
							</div>
						</div>

						<div class="text-right">
							<div class="text-lg font-black text-emerald-400">
								{payment.totalAmount} Kč
							</div>
							<span class="text-[10px] uppercase font-bold px-2 py-0.5 rounded-full {payment.method === 'cash' ? 'bg-emerald-500/20 text-emerald-300' : 'bg-blue-500/20 text-blue-300'}">
								{payment.method === 'cash' ? 'Hotovost' : 'QR Platba'}
							</span>
						</div>
					</div>

					<!-- Books included -->
					<div class="pt-3 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
						<div class="text-xs text-slate-400 flex items-center gap-1.5 flex-wrap">
							<BookOpen class="w-3.5 h-3.5 text-slate-400 shrink-0" />
							<span>Položky ({payment.books.length}):</span>
							{#if payment.expand?.books}
								{#each payment.expand.books as b}
									<span class="bg-slate-900 px-2 py-0.5 rounded text-slate-300 font-mono text-[11px] border border-slate-800">
										{b.code} ({b.price} Kč)
									</span>
								{/each}
							{:else}
								<span class="text-slate-400">{payment.books.length} knih</span>
							{/if}
						</div>

						<!-- Action for pending payments -->
						{#if payment.status === 'pending'}
							<button
								onclick={() => handleConfirmPayment(payment)}
								disabled={confirmingPaymentId === payment.id}
								class="py-2 px-4 rounded-xl bg-emerald-600 hover:bg-emerald-500 active:scale-95 text-white font-bold text-xs transition-all shadow-md flex items-center justify-center gap-1.5 cursor-pointer shrink-0 disabled:opacity-50"
							>
								{#if confirmingPaymentId === payment.id}
									<RefreshCw class="w-3.5 h-3.5 animate-spin" />
									<span>Potvrzuji...</span>
								{:else}
									<Check class="w-4 h-4" />
									<span>Potvrdit platbu (připsáno)</span>
								{/if}
							</button>
						{:else}
							<div class="flex items-center gap-1 text-emerald-400 text-xs font-semibold">
								<CheckCircle2 class="w-4 h-4" />
								<span>Vyřízeno a předáno</span>
							</div>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
