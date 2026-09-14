<script lang="ts">
	import {
		X,
		Coins,
		Banknote,
		Copy,
		Check,
		Printer,
		Search,
		ChevronDown,
		ChevronUp,
		ArrowRight,
		CreditCard,
		User
	} from '@lucide/svelte';
	import type { SellerReturnSummary } from '$lib/types';
	import {
		CZK_DENOMINATIONS,
		calculateCashDistribution,
		aggregateCashDistributions,
		formatCashBreakdownSummary,
		type AggregatedCashDistribution,
		type CashBreakdown,
		type SellerCashInput
	} from '$lib/cashDistribution';

	let {
		open = $bindable(false),
		mode = 'global',
		sellers = [],
		selectedSeller,
		onPayCash
	}: {
		open?: boolean;
		mode?: 'global' | 'individual';
		sellers?: SellerReturnSummary[];
		selectedSeller?: {
			seller: { id: string; name: string; email: string; payoutToBank: boolean };
			balances: { totalUnpaid: number };
		} | SellerReturnSummary;
		onPayCash?: (sellerId: string) => void;
	} = $props();

	// Global filter state: 'cash_only' vs 'all_unpaid'
	let filterType = $state<'cash_only' | 'all_unpaid'>('cash_only');
	let isSellerListExpanded = $state(false);
	let sellerSearchQuery = $state('');
	let copied = $state(false);
	let copyTimeout: any = null;

	// Normalize selected seller in individual mode
	let individualSellerInfo = $derived.by(() => {
		if (!selectedSeller) return null;
		if ('seller' in selectedSeller) {
			return {
				id: selectedSeller.seller.id,
				name: selectedSeller.seller.name,
				email: selectedSeller.seller.email,
				payoutToBank: selectedSeller.seller.payoutToBank,
				amount: selectedSeller.balances.totalUnpaid
			};
		}
		return {
			id: selectedSeller.id,
			name: selectedSeller.name,
			email: selectedSeller.email,
			payoutToBank: selectedSeller.payoutToBank,
			amount: selectedSeller.totalUnpaid
		};
	});

	// Individual calculation
	let individualBreakdown = $derived.by<CashBreakdown | null>(() => {
		if (mode !== 'individual' || !individualSellerInfo) return null;
		return calculateCashDistribution(individualSellerInfo.amount);
	});

	// Global calculation inputs
	let cashEligibleSellers = $derived.by<SellerCashInput[]>(() => {
		return sellers
			.filter((s) => s.totalUnpaid > 0)
			.filter((s) => (filterType === 'cash_only' ? !s.payoutToBank : true))
			.map((s) => ({
				id: s.id,
				name: s.name,
				email: s.email,
				amount: s.totalUnpaid,
				payoutToBank: s.payoutToBank
			}));
	});

	let cashOnlyCount = $derived(
		sellers.filter((s) => !s.payoutToBank && s.totalUnpaid > 0).length
	);
	let allUnpaidCount = $derived(
		sellers.filter((s) => s.totalUnpaid > 0).length
	);

	let aggregatedDistribution = $derived.by<AggregatedCashDistribution>(() => {
		return aggregateCashDistributions(cashEligibleSellers);
	});

	interface DisplayData {
		totalAmount: number;
		totalPieces: number;
		banknotePieces: number;
		coinPieces: number;
		items: import('$lib/cashDistribution').DenominationCount[];
		sellerCount: number;
	}

	let currentData = $derived.by<DisplayData | null>(() => {
		if (mode === 'individual') {
			if (!individualBreakdown) return null;
			return {
				totalAmount: individualBreakdown.amount,
				totalPieces: individualBreakdown.totalPieces,
				banknotePieces: individualBreakdown.banknotePieces,
				coinPieces: individualBreakdown.coinPieces,
				items: individualBreakdown.items,
				sellerCount: 1
			};
		}
		return {
			totalAmount: aggregatedDistribution.totalAmount,
			totalPieces: aggregatedDistribution.totalPieces,
			banknotePieces: aggregatedDistribution.banknotePieces,
			coinPieces: aggregatedDistribution.coinPieces,
			items: aggregatedDistribution.items,
			sellerCount: aggregatedDistribution.sellerCount
		};
	});

	// Filtered list of sellers in the expandable breakdown table
	let filteredBreakdownSellers = $derived.by(() => {
		if (!sellerSearchQuery.trim()) return aggregatedDistribution.sellerBreakdowns;
		const q = sellerSearchQuery.trim().toLowerCase();
		return aggregatedDistribution.sellerBreakdowns.filter(
			(s) =>
				(s.name && s.name.toLowerCase().includes(q)) ||
				(s.email && s.email.toLowerCase().includes(q)) ||
				s.id.toLowerCase().includes(q)
		);
	});

	// Copy formatted text summary to clipboard
	async function copySummary() {
		try {
			let text = '';
			if (mode === 'individual' && individualSellerInfo && individualBreakdown) {
				const singleAgg: AggregatedCashDistribution = {
					sellerCount: 1,
					totalAmount: individualBreakdown.amount,
					totalPieces: individualBreakdown.totalPieces,
					banknotePieces: individualBreakdown.banknotePieces,
					coinPieces: individualBreakdown.coinPieces,
					items: individualBreakdown.items,
					sellerBreakdowns: [
						{
							...individualSellerInfo,
							breakdown: individualBreakdown
						}
					]
				};
				text = formatCashBreakdownSummary(
					singleAgg,
					`Rozpis hotovosti – ${individualSellerInfo.name || individualSellerInfo.email}`
				);
			} else {
				const title =
					filterType === 'cash_only'
						? 'Rozpis hotovosti – Pouze hotovostní prodejci'
						: 'Rozpis hotovosti – Všichni prodejci k vyplacení';
				text = formatCashBreakdownSummary(aggregatedDistribution, title);
			}

			if (navigator.clipboard && window.isSecureContext) {
				await navigator.clipboard.writeText(text);
			} else {
				// Fallback
				const textArea = document.createElement('textarea');
				textArea.value = text;
				textArea.style.position = 'fixed';
				textArea.style.opacity = '0';
				document.body.appendChild(textArea);
				textArea.select();
				document.execCommand('copy');
				document.body.removeChild(textArea);
			}

			copied = true;
			if (copyTimeout) clearTimeout(copyTimeout);
			copyTimeout = setTimeout(() => {
				copied = false;
			}, 2500);
		} catch (err) {
			console.error('Copy to clipboard failed:', err);
		}
	}

	function handlePrint() {
		if (typeof window !== 'undefined') {
			window.print();
		}
	}
</script>

{#if open}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-5 bg-black/60 backdrop-blur-xs select-none"
		role="dialog"
		aria-modal="true"
		aria-label="Rozpis bankovek a mincí"
	>
		<!-- Backdrop button to close -->
		<button
			type="button"
			class="absolute inset-0 w-full h-full cursor-default"
			onclick={() => (open = false)}
			aria-label="Zavřít"
		></button>

		<!-- Modal Window -->
		<div
			class="relative z-10 w-full max-w-3xl bg-white border-3 border-black shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] flex flex-col max-h-[92vh] overflow-hidden"
		>
			<!-- Top Header -->
			<div class="p-4 bg-neutral-900 text-white border-b-2 border-black flex items-start justify-between gap-3 shrink-0">
				<div class="min-w-0">
					<div class="flex items-center gap-2">
						<div class="p-1.5 bg-amber-400 text-black border border-black shrink-0">
							<Coins class="w-5 h-5" />
						</div>
						<h2 class="text-sm sm:text-base font-black uppercase tracking-tight truncate">
							{#if mode === 'individual'}
								ROZPIS HOTOVOSTI – {individualSellerInfo?.name || individualSellerInfo?.email || 'PRODEJCE'}
							{:else}
								ROZPIS HOTOVOSTI PRO POKLADNU
							{/if}
						</h2>
					</div>
					<p class="text-xs text-neutral-400 mt-1">
						{#if mode === 'individual'}
							Přehled bankovek a mincí potřebných pro vyplacení tohoto prodejce
						{:else}
							Optimální součet bankovek a mincí pro vyplacení prodejců na místě
						{/if}
					</p>
				</div>

				<button
					type="button"
					onclick={() => (open = false)}
					class="p-1.5 border border-neutral-700 bg-neutral-800 hover:bg-neutral-700 active:scale-95 text-white transition-all cursor-pointer shrink-0"
					title="Zavřít"
				>
					<X class="w-5 h-5" />
				</button>
			</div>

			<!-- Modal Body (Scrollable) -->
			<div class="flex-1 overflow-y-auto p-4 sm:p-5 space-y-4 text-black">
				<!-- Global Filter Switcher -->
				{#if mode === 'global'}
					<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 p-2.5 bg-neutral-100 border-2 border-black">
						<span class="text-xs font-black uppercase tracking-wider text-neutral-700">
							VÝBĚR PRODEJCŮ PRO VÝPOČET:
						</span>
						<div class="inline-flex border border-black bg-white p-0.5">
							<button
								type="button"
								onclick={() => (filterType = 'cash_only')}
								class="px-3 py-1 text-xs font-black uppercase tracking-wide cursor-pointer transition-colors {filterType === 'cash_only'
									? 'bg-amber-400 text-black border border-black shadow-[1px_1px_0px_0px_rgba(0,0,0,1)]'
									: 'text-neutral-600 hover:text-black'}"
							>
								Pouze hotovost ({cashOnlyCount})
							</button>
							<button
								type="button"
								onclick={() => (filterType = 'all_unpaid')}
								class="px-3 py-1 text-xs font-black uppercase tracking-wide cursor-pointer transition-colors {filterType === 'all_unpaid'
									? 'bg-amber-400 text-black border border-black shadow-[1px_1px_0px_0px_rgba(0,0,0,1)]'
									: 'text-neutral-600 hover:text-black'}"
								title="Zahrnout i prodejce s bankovním účtem pro případ výplaty na místě"
							>
								Všichni s dluhem ({allUnpaidCount})
							</button>
						</div>
					</div>
				{:else if individualSellerInfo}
					<!-- Individual Seller Banner -->
					<div class="p-3 bg-neutral-50 border-2 border-black flex flex-wrap items-center justify-between gap-2">
						<div class="flex items-center gap-2">
							<User class="w-4 h-4 text-neutral-500" />
							<div>
								<span class="font-black text-xs uppercase block">{individualSellerInfo.name || individualSellerInfo.email}</span>
								<span class="text-[10px] font-mono text-neutral-500">{individualSellerInfo.email} (ID: {individualSellerInfo.id})</span>
							</div>
						</div>

						<div class="inline-flex items-center gap-1.5 px-2 py-0.5 text-[11px] font-black uppercase border border-black {individualSellerInfo.payoutToBank ? 'bg-blue-50 text-blue-900 border-blue-900' : 'bg-amber-50 text-amber-900 border-amber-900'}">
							{#if individualSellerInfo.payoutToBank}
								<CreditCard class="w-3.5 h-3.5" />
								<span>Preferuje účet</span>
							{:else}
								<Banknote class="w-3.5 h-3.5" />
								<span>Preferuje hotovost</span>
							{/if}
						</div>
					</div>
				{/if}

				<!-- Main Summary KPI Strip -->
				{#if currentData}
					<div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
						<!-- Total Amount -->
						<div class="p-3 bg-neutral-900 text-white border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
							<span class="text-[10px] font-mono font-bold uppercase text-neutral-400 block">CELKEM K VYPLACENÍ</span>
							<span class="text-xl sm:text-2xl font-black text-amber-400 block tracking-tight">
								{currentData.totalAmount.toLocaleString('cs-CZ')} Kč
							</span>
							<span class="text-[10px] text-neutral-400 block mt-0.5">
								{#if mode === 'global'}
									{currentData.sellerCount} prodejců
								{:else}
									1 prodejce
								{/if}
							</span>
						</div>

						<!-- Total Pieces -->
						<div class="p-3 bg-white border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
							<span class="text-[10px] font-mono font-bold uppercase text-neutral-500 block">CELKEM KUSŮ</span>
							<span class="text-xl sm:text-2xl font-black text-black block tracking-tight">
								{currentData.totalPieces} ks
							</span>
							<span class="text-[10px] text-neutral-500 block mt-0.5">
								Bankovky + mince
							</span>
						</div>

						<!-- Banknotes count -->
						<div class="p-3 bg-amber-50 border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
							<span class="text-[10px] font-mono font-bold uppercase text-amber-800 block">BANKOVKY</span>
							<span class="text-xl sm:text-2xl font-black text-amber-950 block tracking-tight">
								{currentData.banknotePieces} ks
							</span>
							<span class="text-[10px] text-amber-800 block mt-0.5">
								100 Kč až 5 000 Kč
							</span>
						</div>

						<!-- Coins count -->
						<div class="p-3 bg-neutral-100 border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
							<span class="text-[10px] font-mono font-bold uppercase text-neutral-600 block">MINCE</span>
							<span class="text-xl sm:text-2xl font-black text-neutral-900 block tracking-tight">
								{currentData.coinPieces} ks
							</span>
							<span class="text-[10px] text-neutral-500 block mt-0.5">
								1 Kč až 50 Kč
							</span>
						</div>
					</div>

					<!-- Section: BANKOVKY (5000, 2000, 1000, 500, 200, 100) -->
					<div class="space-y-2">
						<div class="flex items-center justify-between">
							<h3 class="text-xs font-black uppercase tracking-wider text-neutral-800 flex items-center gap-1.5">
								<Banknote class="w-4 h-4 text-emerald-700" />
								<span>BANKOVKY</span>
							</h3>
							<span class="text-[11px] font-mono font-bold text-neutral-500">
								{currentData.banknotePieces} ks
							</span>
						</div>

						<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-2">
							{#each currentData.items.filter((i) => i.denomination.type === 'banknote') as item}
								{@const hasCount = item.count > 0}
								<div
									class="p-2.5 border-2 border-black flex flex-col justify-between transition-all {hasCount
										? `${item.denomination.bgClass} shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]`
										: 'bg-neutral-50/60 opacity-40 border-neutral-300'}"
								>
									<div class="flex items-center justify-between gap-1 mb-1">
										<span class="text-xs font-black {hasCount ? 'text-black' : 'text-neutral-500'}">
											{item.denomination.label}
										</span>
										<span class="text-[9px] font-bold uppercase px-1 py-0.2 bg-black text-white">
											BANKOVKA
										</span>
									</div>

									<div class="my-1 text-center">
										<span class="text-xl sm:text-2xl font-black {hasCount ? 'text-black' : 'text-neutral-400'} block">
											{item.count} <span class="text-xs font-bold font-mono">ks</span>
										</span>
									</div>

									<div class="pt-1.5 border-t border-black/20 text-center">
										<span class="text-[10px] font-mono font-bold text-neutral-600">
											= {item.totalAmount.toLocaleString('cs-CZ')} Kč
										</span>
									</div>
								</div>
							{/each}
						</div>
					</div>

					<!-- Section: MINCE (50, 20, 10, 5, 2, 1) -->
					<div class="space-y-2 pt-1">
						<div class="flex items-center justify-between">
							<h3 class="text-xs font-black uppercase tracking-wider text-neutral-800 flex items-center gap-1.5">
								<Coins class="w-4 h-4 text-amber-600" />
								<span>MINCE</span>
							</h3>
							<span class="text-[11px] font-mono font-bold text-neutral-500">
								{currentData.coinPieces} ks
							</span>
						</div>

						<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-2">
							{#each currentData.items.filter((i) => i.denomination.type === 'coin') as item}
								{@const hasCount = item.count > 0}
								<div
									class="p-2.5 border-2 border-black flex flex-col justify-between transition-all {hasCount
										? `${item.denomination.bgClass} shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]`
										: 'bg-neutral-50/60 opacity-40 border-neutral-300'}"
								>
									<div class="flex items-center justify-between gap-1 mb-1">
										<span class="text-xs font-black {hasCount ? 'text-black' : 'text-neutral-500'}">
											{item.denomination.label}
										</span>
										<span class="text-[9px] font-bold uppercase px-1 py-0.2 bg-neutral-800 text-white">
											MINCE
										</span>
									</div>

									<div class="my-1 text-center">
										<span class="text-xl sm:text-2xl font-black {hasCount ? 'text-black' : 'text-neutral-400'} block">
											{item.count} <span class="text-xs font-bold font-mono">ks</span>
										</span>
									</div>

									<div class="pt-1.5 border-t border-black/20 text-center">
										<span class="text-[10px] font-mono font-bold text-neutral-600">
											= {item.totalAmount.toLocaleString('cs-CZ')} Kč
										</span>
									</div>
								</div>
							{/each}
						</div>
					</div>

					<!-- Individual mode: Compact breakdown chips string -->
					{#if mode === 'individual'}
						<div class="p-3 bg-yellow-50 border-2 border-black space-y-1.5">
							<span class="text-[10px] font-bold uppercase text-neutral-600 block">SOUHRN K VÝPLATĚ:</span>
							<div class="flex flex-wrap items-center gap-1.5">
								{#each currentData.items.filter((i) => i.count > 0) as item}
									<span class="px-2.5 py-1 text-xs font-black border border-black bg-white shadow-[1px_1px_0px_0px_rgba(0,0,0,1)]">
										{item.count}× {item.denomination.label}
									</span>
								{/each}
							</div>
						</div>
					{/if}

					<!-- Global Mode: Expandable Seller-by-Seller Breakdown Table -->
					{#if mode === 'global' && aggregatedDistribution.sellerBreakdowns.length > 0}
						<div class="border-2 border-black bg-white">
							<button
								type="button"
								onclick={() => (isSellerListExpanded = !isSellerListExpanded)}
								class="w-full p-3 bg-neutral-100 hover:bg-neutral-200 flex items-center justify-between text-left font-black text-xs uppercase cursor-pointer border-b border-black"
							>
								<div class="flex items-center gap-2">
									<span>PŘEHLED PODLE JEDNOTLIVÝCH PRODEJCŮ ({aggregatedDistribution.sellerBreakdowns.length})</span>
								</div>
								<div class="flex items-center gap-1 text-neutral-600">
									<span>{isSellerListExpanded ? 'Skrýt' : 'Zobrazit podrobnosti'}</span>
									{#if isSellerListExpanded}
										<ChevronUp class="w-4 h-4" />
									{:else}
										<ChevronDown class="w-4 h-4" />
									{/if}
								</div>
							</button>

							{#if isSellerListExpanded}
								<div class="p-3 space-y-3">
									<!-- Quick Search inside table -->
									<div class="relative">
										<input
											type="text"
											placeholder="Filtrovat prodejce podle jména nebo emailu..."
											bind:value={sellerSearchQuery}
											class="w-full pl-8 pr-3 py-1.5 border border-black text-xs font-bold focus:outline-none focus:bg-yellow-50"
										/>
										<Search class="w-3.5 h-3.5 absolute left-2.5 top-2.5 text-neutral-400" />
									</div>

									<div class="divide-y divide-neutral-200 max-h-60 overflow-y-auto border border-neutral-300">
										{#if filteredBreakdownSellers.length === 0}
											<div class="p-4 text-center text-xs text-neutral-400 font-bold">
												Žádný prodejce neodpovídá hledání.
											</div>
										{:else}
											{#each filteredBreakdownSellers as s}
												<div class="p-2.5 flex flex-col sm:flex-row sm:items-center justify-between gap-2 hover:bg-neutral-50 text-xs">
													<div class="min-w-0 pr-2">
														<div class="flex items-center gap-1.5">
															<span class="font-black text-black truncate">{s.name || s.email}</span>
															{#if s.payoutToBank}
																<span class="px-1 py-0.2 text-[9px] font-bold bg-blue-100 text-blue-900 border border-blue-400">Účet</span>
															{:else}
																<span class="px-1 py-0.2 text-[9px] font-bold bg-amber-100 text-amber-900 border border-amber-400">Hotovost</span>
															{/if}
														</div>
														<p class="text-[10px] font-mono text-neutral-500 truncate">{s.email}</p>
													</div>

													<div class="flex flex-wrap items-center gap-1.5 sm:justify-end shrink-0">
														<span class="font-black text-black sm:text-right pr-2">
															{s.amount.toLocaleString('cs-CZ')} Kč
														</span>
														<div class="flex flex-wrap gap-1">
															{#each s.breakdown.items.filter((i) => i.count > 0) as i}
																<span class="px-1.5 py-0.5 text-[10px] font-mono font-bold bg-neutral-100 border border-neutral-300">
																	{i.count}× {i.denomination.value}
																</span>
															{/each}
														</div>
													</div>
												</div>
											{/each}
										{/if}
									</div>
								</div>
							{/if}
						</div>
					{/if}
				{/if}
			</div>

			<!-- Modal Footer Action Bar -->
			<div class="p-3 sm:p-4 bg-neutral-100 border-t-2 border-black flex flex-wrap items-center justify-between gap-2 shrink-0">
				<!-- Left Actions: Copy / Print -->
				<div class="flex items-center gap-2">
					<button
						type="button"
						onclick={copySummary}
						class="px-3 py-2 border-2 border-black bg-white hover:bg-neutral-200 active:scale-95 text-black font-black text-xs uppercase flex items-center gap-1.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] cursor-pointer"
						title="Zkopírovat textový přehled do schránky"
					>
						{#if copied}
							<Check class="w-4 h-4 text-emerald-600" />
							<span class="text-emerald-700">ZKOPÍROVÁNO!</span>
						{:else}
							<Copy class="w-4 h-4" />
							<span>KOPÍROVAT ROZPIS</span>
						{/if}
					</button>

					<button
						type="button"
						onclick={handlePrint}
						class="px-3 py-2 border-2 border-black bg-white hover:bg-neutral-200 active:scale-95 text-black font-bold text-xs uppercase hidden sm:flex items-center gap-1.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] cursor-pointer"
						title="Vytisknout rozpis"
					>
						<Printer class="w-4 h-4" />
						<span>TISKNOUT</span>
					</button>
				</div>

				<!-- Right Actions: Pay Cash shortcut (individual) or Close -->
				<div class="flex items-center gap-2">
					{#if mode === 'individual' && onPayCash && individualSellerInfo && individualSellerInfo.amount > 0}
						<button
							type="button"
							onclick={() => {
								if (individualSellerInfo) {
									open = false;
									onPayCash(individualSellerInfo.id);
								}
							}}
							class="px-4 py-2 bg-yellow-400 hover:bg-yellow-500 text-black font-black text-xs uppercase tracking-wider border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:scale-95 transition-all flex items-center gap-1.5 cursor-pointer"
						>
							<Banknote class="w-4 h-4" />
							<span>VYPLATIT V HOTOVOSTI</span>
							<ArrowRight class="w-4 h-4" />
						</button>
					{/if}

					<button
						type="button"
						onclick={() => (open = false)}
						class="px-4 py-2 bg-black text-white hover:bg-neutral-800 text-xs font-black uppercase border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:scale-95 cursor-pointer"
					>
						Zavřít
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
