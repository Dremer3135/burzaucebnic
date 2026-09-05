<script lang="ts">
	import { onDestroy } from 'svelte';
	import { auth, cashierPayments, eventStore } from '$lib/stores.svelte';
	import { pb } from '$lib/pocketbase';
	import { renderSpaydQRCode } from '$lib/barcodes';
	import {
		Receipt,
		CheckCircle2,
		Clock,
		Search,
		BookOpen,
		RefreshCw,
		Check,
		AlertCircle,
		X
	} from '@lucide/svelte';
	import type { Payment } from '$lib/types';

	let activeTab = $state<'pending' | 'completed'>('pending');
	let searchQuery = $state('');
	let confirmingPaymentId = $state<string | null>(null);
	let paymentToConfirm = $state<Payment | null>(null);
	let selectedQrPayment = $state<Payment | null>(null);
	let qrModalCanvas = $state<HTMLCanvasElement | null>(null);
	let errorMessage = $state('');
	let modalError = $state('');

	$effect(() => {
		if (auth.isCashier) {
			cashierPayments.init();
		}
	});

	onDestroy(() => {
		cashierPayments.cleanup();
	});

	$effect(() => {
		if (selectedQrPayment && qrModalCanvas) {
			const ev = eventStore.event;
			const iban = ev?.iban || 'CZ6520100000002101234567';
			const payerEmail = selectedQrPayment.expand?.buyer?.email || '';
			renderSpaydQRCode(qrModalCanvas, {
				iban: iban,
				amount: selectedQrPayment.totalAmount,
				vs: selectedQrPayment.variableSymbol,
				paymentId: selectedQrPayment.id,
				payerEmail: payerEmail
			}).catch(console.error);
		}
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
		modalError = '';
		errorMessage = '';
		try {
			await pb.send('/api/cashier/confirm-payment', {
				method: 'POST',
				body: { paymentId: payment.id }
			});
			// Optimistic status update for instant visual feedback
			cashierPayments.payments = cashierPayments.payments.map((p) =>
				p.id === payment.id ? { ...p, status: 'completed' } : p
			);
			paymentToConfirm = null;
		} catch (err: any) {
			console.error('Failed to confirm payment', err);
			const msg = err?.message || 'Chyba při potvrzení platby.';
			modalError = msg;
			errorMessage = msg;
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
					<!-- Upper row: Clickable to show QR payment modal -->
					<div
						role="button"
						tabindex="0"
						onclick={() => (selectedQrPayment = payment)}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') selectedQrPayment = payment; }}
						class="flex items-center justify-between gap-2 pb-2.5 border-b border-black cursor-pointer hover:bg-neutral-50 active:bg-neutral-100 transition-colors p-1.5 -m-1.5 select-none"
						title="Kliknutím zobrazit platební QR kód"
					>
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
									{payment.expand?.buyer?.email || ''}
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
								onclick={() => {
									paymentToConfirm = payment;
									modalError = '';
								}}
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

	<!-- Confirmation Popup for Submitting/Confirming Payments -->
	{#if paymentToConfirm}
		<div class="fixed inset-0 bg-black/60 z-50 flex items-center justify-center p-4">
			<div class="w-full max-w-sm bg-white border-4 border-black p-5 shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] text-center text-black">
				<div class="inline-flex p-3 bg-neutral-100 border-2 border-black mb-3">
					<CheckCircle2 class="w-8 h-8 text-black" />
				</div>
				<h3 class="text-base font-black uppercase mb-1">Potvrdit přijetí platby?</h3>

				<div class="my-3 p-2.5 bg-neutral-50 border-2 border-black text-left font-mono text-xs space-y-1.5">
					<div class="flex justify-between items-baseline border-b border-neutral-200 pb-1">
						<span class="font-sans font-black text-neutral-500 uppercase text-[10px]">Částka:</span>
						<span class="font-sans font-black text-black text-base">{paymentToConfirm.totalAmount} Kč</span>
					</div>
					<div class="flex justify-between items-baseline border-b border-neutral-200 pb-1">
						<span class="font-sans font-black text-neutral-500 uppercase text-[10px]">Var. symbol:</span>
						<span class="font-black text-black text-sm">{paymentToConfirm.variableSymbol}</span>
					</div>
					{#if paymentToConfirm.expand?.buyer}
						<div class="flex justify-between items-baseline border-b border-neutral-200 pb-1">
							<span class="font-sans font-black text-neutral-500 uppercase text-[10px]">Kupující:</span>
							<span class="font-sans font-bold text-black text-[11px] truncate max-w-[170px]">
								{paymentToConfirm.expand.buyer.name || paymentToConfirm.expand.buyer.email}
							</span>
						</div>
					{/if}
					<div class="flex justify-between items-baseline">
						<span class="font-sans font-black text-neutral-500 uppercase text-[10px]">Knihy:</span>
						<span class="font-sans font-bold text-black text-[11px]">
							{paymentToConfirm.books.length} {paymentToConfirm.books.length === 1 ? 'kniha' : paymentToConfirm.books.length >= 2 && paymentToConfirm.books.length <= 4 ? 'knihy' : 'knih'}
						</span>
					</div>
				</div>

				{#if modalError}
					<div class="mb-3 p-2 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold flex items-center gap-1.5 text-left">
						<AlertCircle class="w-4 h-4 shrink-0 text-red-600" />
						<span>{modalError}</span>
					</div>
				{/if}

				<p class="text-xs text-neutral-600 font-bold mb-4">
					Opravdu byla tato platba připsána na účet? Knihy budou označeny jako prodané.
				</p>

				<div class="flex gap-2">
					<button
						type="button"
						onclick={() => (paymentToConfirm = null)}
						disabled={confirmingPaymentId !== null}
						class="flex-1 py-2.5 bg-white hover:bg-neutral-100 text-black text-xs font-black uppercase tracking-wider border-2 border-black cursor-pointer disabled:opacity-50"
					>
						ZRUŠIT
					</button>
					<button
						type="button"
						onclick={() => {
							if (paymentToConfirm) {
								handleConfirmPayment(paymentToConfirm);
							}
						}}
						disabled={confirmingPaymentId !== null}
						class="flex-1 py-2.5 bg-black hover:bg-neutral-800 text-white text-xs font-black uppercase tracking-wider border-2 border-black cursor-pointer flex items-center justify-center gap-1.5 disabled:opacity-50"
					>
						{#if confirmingPaymentId === paymentToConfirm.id}
							<RefreshCw class="w-3.5 h-3.5 animate-spin" />
							<span>POTVRZUJI...</span>
						{:else}
							<span>POTVRDIT</span>
						{/if}
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- QR Payment Modal (Identical layout to checkout, with Close button at bottom) -->
	{#if selectedQrPayment}
		<div
			class="fixed inset-0 bg-black/70 backdrop-blur-xs z-50 flex items-center justify-center p-3 select-none"
			role="dialog"
			tabindex="-1"
			aria-modal="true"
			onkeydown={(e) => { if (e.key === 'Escape') selectedQrPayment = null; }}
		>
			<div
				class="fixed inset-0"
				onclick={() => (selectedQrPayment = null)}
				role="presentation"
			></div>

			<div class="bg-white border-4 border-black p-4 flex flex-col justify-between max-w-sm w-full relative z-10 text-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] max-h-[92vh] overflow-y-auto">
				<!-- Compact Header: Amount & Variable Symbol -->
				<div class="border-b-2 border-black pb-2.5 pt-1">
					<div class="flex items-baseline justify-between">
						<div>
							<span class="text-[10px] font-mono font-bold text-neutral-500 uppercase block">Částka</span>
							<span class="text-3xl font-black text-black leading-none">{selectedQrPayment.totalAmount} Kč</span>
						</div>
						<div class="text-right flex items-center gap-3">
							<div>
								<span class="text-[10px] font-mono font-bold text-neutral-500 uppercase block">Var. symbol</span>
								<span class="font-mono text-xl font-black text-black leading-none">{selectedQrPayment.variableSymbol}</span>
							</div>
							<button
								type="button"
								onclick={() => (selectedQrPayment = null)}
								class="p-1.5 border-2 border-black bg-white hover:bg-neutral-100 text-black cursor-pointer active:scale-95"
								title="Zavřít"
								aria-label="Zavřít"
							>
								<X class="w-4 h-4" />
							</button>
						</div>
					</div>
					{#if selectedQrPayment.expand?.buyer}
						{@const buyer = selectedQrPayment.expand.buyer}
						<div class="text-[11px] font-bold text-neutral-600 truncate mt-1.5 flex items-center gap-1.5">
							<span class="text-neutral-400 font-normal">Kupující:</span>
							<span class="text-black font-mono font-bold truncate">
								{buyer.name ? `${buyer.name} (${buyer.email || ''})` : buyer.email || ''}
							</span>
						</div>
					{/if}
				</div>

				<!-- Centered SPAYD QR Canvas -->
				<div class="flex-1 flex flex-col items-center justify-center py-4 min-h-0">
					<div class="bg-white p-2 border-2 border-black shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]">
						<canvas bind:this={qrModalCanvas} class="w-48 h-48 sm:w-56 sm:h-56 max-h-[42vh] max-w-[42vh] aspect-square block"></canvas>
					</div>
				</div>

				<!-- Bottom Action: Just Close -->
				<div class="pt-1 pb-1">
					<button
						type="button"
						onclick={() => (selectedQrPayment = null)}
						class="w-full py-3.5 px-4 bg-black hover:bg-neutral-800 text-white font-black text-sm uppercase tracking-wider border-2 border-black flex items-center justify-center gap-2 cursor-pointer active:scale-98 transition-transform"
					>
						<X class="w-4 h-4" />
						<span>ZAVŘÍT</span>
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
