<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { pb, getBookThumbnailUrl } from '$lib/pocketbase';
	import type {
		SellerReturnSummary,
		SellerReturnDetails,
		FioPayoutSyncResult
	} from '$lib/types';
	import { idToColor } from '$lib/scanner';
	import ReturnScannerModal from '$lib/components/ReturnScannerModal.svelte';
	import {
		Search,
		CreditCard,
		Banknote,
		RotateCcw,
		Download,
		RefreshCw,
		CheckCircle2,
		AlertTriangle,
		X,
		BookOpen,
		Clock,
		ExternalLink
	} from '@lucide/svelte';

	// Sellers list
	let sellers = $state<SellerReturnSummary[]>([]);
	let isLoadingSellers = $state(true);
	let sellerSearchQuery = $state('');

	// Selected seller
	let selectedSellerId = $state<string | null>(null);
	let sellerDetails = $state<SellerReturnDetails | null>(null);
	let isLoadingDetails = $state(false);

	// Modals
	let isScannerOpen = $state(false);
	let isCashPayoutModalOpen = $state(false);
	let isCashPayoutSubmitting = $state(false);
	let cashPayoutConfirmedWarning = $state(false);

	let isBulkXmlModalOpen = $state(false);
	let isDownloadingXml = $state(false);

	let isSyncingFio = $state(false);
	let syncResultModal = $state<FioPayoutSyncResult | null>(null);

	// Feedback toast
	let toastMessage = $state<string | null>(null);
	let toastType = $state<'success' | 'error'>('success');
	let toastTimeout: any = null;

	// Cooldown timer for Fio sync
	let cooldownSeconds = $state(0);
	let cooldownInterval: any = null;

	// Filtered sellers
	let filteredSellers = $derived(
		sellers.filter((s) => {
			if (!sellerSearchQuery.trim()) return true;
			const q = sellerSearchQuery.trim().toLowerCase();
			return (
				(s.name && s.name.toLowerCase().includes(q)) ||
				(s.email && s.email.toLowerCase().includes(q)) ||
				s.id.toLowerCase().includes(q)
			);
		})
	);

	// Bulk XML statistics
	let bankSellersWithUnpaid = $derived(
		sellers.filter((s) => s.payoutToBank && s.iban && s.totalUnpaid > 0)
	);
	let bulkXmlTotalAmount = $derived(
		Math.round(bankSellersWithUnpaid.reduce((acc, s) => acc + s.totalUnpaid, 0) * 100) / 100
	);

	onMount(async () => {
		await loadSellers();
		initCooldownTimer();
	});

	onDestroy(() => {
		if (cooldownInterval) clearInterval(cooldownInterval);
		if (toastTimeout) clearTimeout(toastTimeout);
	});

	function initCooldownTimer() {
		const storedCooldown = localStorage.getItem('fio_payout_cooldown_until');
		if (storedCooldown) {
			const target = parseInt(storedCooldown, 10);
			const diff = Math.ceil((target - Date.now()) / 1000);
			if (diff > 0) {
				cooldownSeconds = diff;
			}
		}

		cooldownInterval = setInterval(() => {
			if (cooldownSeconds > 0) {
				cooldownSeconds--;
			}
		}, 1000);
	}

	function startCooldown(seconds: number) {
		cooldownSeconds = seconds;
		localStorage.setItem('fio_payout_cooldown_until', String(Date.now() + seconds * 1000));
	}

	function showToast(msg: string, type: 'success' | 'error' = 'success') {
		toastMessage = msg;
		toastType = type;
		if (toastTimeout) clearTimeout(toastTimeout);
		toastTimeout = setTimeout(() => {
			toastMessage = null;
		}, 4000);
	}

	async function loadSellers() {
		isLoadingSellers = true;
		try {
			const res = await pb.send<SellerReturnSummary[]>('/api/cashier/returns/sellers', {
				method: 'GET'
			});
			sellers = res || [];

			// If a seller is already selected, refresh their details
			if (selectedSellerId) {
				await loadSellerDetails(selectedSellerId);
			}
		} catch (err: any) {
			console.error('Error loading sellers:', err);
			showToast('Chyba při načítání seznamu prodejců: ' + (err?.message || ''), 'error');
		} finally {
			isLoadingSellers = false;
		}
	}

	async function selectSeller(id: string) {
		selectedSellerId = id;
		await loadSellerDetails(id);
	}

	async function loadSellerDetails(id: string) {
		isLoadingDetails = true;
		try {
			const res = await pb.send<SellerReturnDetails>(
				`/api/cashier/returns/seller-details?id=${encodeURIComponent(id)}`,
				{ method: 'GET' }
			);
			sellerDetails = res;
		} catch (err: any) {
			console.error('Error loading seller details:', err);
			showToast('Chyba při načítání detailu prodejce.', 'error');
		} finally {
			isLoadingDetails = false;
		}
	}

	function openCashPayoutModal() {
		if (!sellerDetails || sellerDetails.balances.totalUnpaid <= 0) return;
		cashPayoutConfirmedWarning = false;
		isCashPayoutModalOpen = true;
	}

	async function confirmCashPayout() {
		if (!sellerDetails || isCashPayoutSubmitting) return;
		isCashPayoutSubmitting = true;

		try {
			const res = await pb.send<{ success: boolean; totalUnpaid: number; amount: number }>(
				'/api/cashier/returns/pay-cash',
				{
					method: 'POST',
					body: {
						sellerId: sellerDetails.seller.id,
						amount: sellerDetails.balances.totalUnpaid
					}
				}
			);

			if (res && res.success) {
				showToast(`Vyplaceno ${res.amount || sellerDetails.balances.totalUnpaid} Kč v hotovosti.`, 'success');
				isCashPayoutModalOpen = false;
				await loadSellers();
			}
		} catch (err: any) {
			console.error('Cash payout error:', err);
			showToast(err?.message || 'Chyba při provádění výplaty.', 'error');
		} finally {
			isCashPayoutSubmitting = false;
		}
	}

	async function downloadFioXml(sellerId?: string) {
		isDownloadingXml = true;
		try {
			const url = sellerId
				? `/api/cashier/payouts/fio-xml?sellerId=${encodeURIComponent(sellerId)}`
				: `/api/cashier/payouts/fio-xml`;

			const res = await fetch(url, {
				headers: {
					Authorization: pb.authStore.token
				}
			});

			if (!res.ok) {
				const errJson = await res.json().catch(() => null);
				throw new Error(errJson?.message || 'Chyba při stahování Fio XML.');
			}

			const blob = await res.blob();
			const disposition = res.headers.get('content-disposition');
			let filename = sellerId ? 'fio_vyplata.xml' : 'fio_vyplaty_burza.xml';
			if (disposition && disposition.includes('filename=')) {
				const match = disposition.match(/filename="?([^"]+)"?/);
				if (match && match[1]) filename = match[1];
			}

			const objectUrl = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = objectUrl;
			a.download = filename;
			document.body.appendChild(a);
			a.click();
			document.body.removeChild(a);
			URL.revokeObjectURL(objectUrl);

			showToast(`XML soubor byl úspěšně stažen (${filename}).`, 'success');
			isBulkXmlModalOpen = false;
		} catch (err: any) {
			console.error('Download XML error:', err);
			showToast(err?.message || 'Chyba při stahování XML.', 'error');
		} finally {
			isDownloadingXml = false;
		}
	}

	async function syncFioPayouts() {
		if (isSyncingFio || cooldownSeconds > 0) return;
		isSyncingFio = true;

		try {
			const res = await pb.send<FioPayoutSyncResult>('/api/cashier/sync-fio-payouts', {
				method: 'POST'
			});

			startCooldown(res.cooldownSeconds || 30);
			syncResultModal = res;
			await loadSellers();
		} catch (err: any) {
			console.error('Fio payout sync error:', err);
			if (err?.status === 429 && err?.data?.retryAfterSeconds) {
				startCooldown(err.data.retryAfterSeconds);
			}
			showToast(err?.message || 'Chyba při synchronizaci s Fio bankou.', 'error');
		} finally {
			isSyncingFio = false;
		}
	}

	function handleBooksReturnedSuccess(returnedBookIds: string[]) {
		showToast(`Úspěšně označeno ${returnedBookIds.length} knih jako vrácené.`, 'success');
		loadSellers();
	}
</script>

<div class="max-w-6xl mx-auto p-3 sm:p-5 space-y-4">
	<!-- Top Bar Actions -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-4 bg-white border-2 border-black shadow-[3px_3px_0px_0px_rgba(0,0,0,1)]">
		<div>
			<h1 class="text-base sm:text-lg font-black uppercase tracking-tight text-black flex items-center gap-2">
				<RotateCcw class="w-5 h-5 text-black" />
				<span>VRACENÍ KNIH A VÝPLATY PRODEJCŮM</span>
			</h1>
			<p class="text-xs text-neutral-600 mt-0.5">
				Vracení neprodaných knih prodejcům a vypořádání tržeb v hotovosti i bankou
			</p>
		</div>

		<!-- Action Buttons: Bulk Fio XML & Sync -->
		<div class="flex items-center gap-2 shrink-0">
			<!-- FIO XML DÁVKA -->
			<button
				type="button"
				onclick={() => (isBulkXmlModalOpen = true)}
				class="px-3 py-2 border-2 border-black bg-white hover:bg-neutral-100 text-black font-black text-xs uppercase tracking-wider flex items-center gap-1.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:scale-95 transition-all cursor-pointer"
			>
				<Download class="w-4 h-4" />
				<span>FIO XML DÁVKA</span>
				{#if bankSellersWithUnpaid.length > 0}
					<span class="px-1.5 py-0.2 bg-black text-white text-[10px] font-bold">
						{bankSellersWithUnpaid.length}
					</span>
				{/if}
			</button>

			<!-- FIO SYNCHRONIZACE VÝPLAT -->
			<button
				type="button"
				onclick={syncFioPayouts}
				disabled={isSyncingFio || cooldownSeconds > 0}
				class="px-3 py-2 border-2 border-black bg-yellow-300 hover:bg-yellow-400 disabled:opacity-50 disabled:cursor-not-allowed text-black font-black text-xs uppercase tracking-wider flex items-center gap-1.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:scale-95 transition-all cursor-pointer"
				title={cooldownSeconds > 0 ? `Limit Fio API: počkejte ${cooldownSeconds} s` : 'Zkontrolovat odchozí platby ve Fio bance'}
			>
				{#if isSyncingFio}
					<RefreshCw class="w-4 h-4 animate-spin" />
					<span>SYNCHRONIZUJI...</span>
				{:else}
					<RefreshCw class="w-4 h-4" />
					<span>
						FIO SYNCHRONIZACE
						{#if cooldownSeconds > 0}
							({cooldownSeconds}s)
						{/if}
					</span>
				{/if}
			</button>
		</div>
	</div>

	<!-- Main Grid: Seller Search / List (Left) + Selected Seller View (Right) -->
	<div class="grid grid-cols-1 lg:grid-cols-12 gap-4">
		<!-- Left Column: Search & Seller List (5 cols on lg) -->
		<div class="lg:col-span-4 space-y-3">
			<div class="p-3 bg-white border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] space-y-2">
				<div class="relative">
					<input
						type="text"
						placeholder="Hledat podle jména, e-mailu..."
						bind:value={sellerSearchQuery}
						class="w-full pl-8 pr-3 py-2 border-2 border-black text-xs font-bold focus:outline-none focus:bg-yellow-50"
					/>
					<Search class="w-4 h-4 absolute left-2.5 top-2.5 text-neutral-400" />
					{#if sellerSearchQuery}
						<button
							type="button"
							onclick={() => (sellerSearchQuery = '')}
							class="absolute right-2 top-2 p-0.5 text-neutral-400 hover:text-black"
						>
							<X class="w-3.5 h-3.5" />
						</button>
					{/if}
				</div>

				<div class="flex items-center justify-between text-[11px] font-mono text-neutral-500 px-1">
					<span>Počet prodejců: {filteredSellers.length}</span>
					<button
						type="button"
						onclick={loadSellers}
						class="hover:text-black font-bold uppercase cursor-pointer"
					>
						Obnovit
					</button>
				</div>
			</div>

			<!-- Sellers List -->
			<div class="overflow-y-auto max-h-[calc(100vh-280px)] space-y-2 pr-1">
				{#if isLoadingSellers}
					<div class="p-8 text-center bg-white border-2 border-black text-xs font-bold text-neutral-500 uppercase">
						<RefreshCw class="w-4 h-4 animate-spin mx-auto mb-2" />
						Načítání prodejců...
					</div>
				{:else if filteredSellers.length === 0}
					<div class="p-6 text-center bg-white border-2 border-black text-xs font-bold text-neutral-400 uppercase">
						Žádní prodejci neodpovídají hledání
					</div>
				{:else}
					{#each filteredSellers as s (s.id)}
						{@const isSelected = selectedSellerId === s.id}
						{@const isFullyPaid = s.totalEarned > 0 && s.totalUnpaid === 0}
						{@const hasUnpaid = s.totalUnpaid > 0}

						<button
							type="button"
							onclick={() => selectSeller(s.id)}
							class="w-full p-3 border-2 border-black text-left transition-all cursor-pointer shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:scale-[0.99] {isSelected
								? 'bg-black text-white'
								: 'bg-white text-black hover:bg-neutral-50'}"
						>
							<div class="flex items-start justify-between gap-2">
								<div class="min-w-0 flex-1">
									<p class="text-xs font-black uppercase tracking-tight truncate">
										{s.name || s.email}
									</p>
									<p class="text-[10px] font-mono truncate {isSelected ? 'text-neutral-300' : 'text-neutral-500'}">
										{s.email}
									</p>
								</div>

								<!-- Preference icon: Bank or Cash -->
								<div
									class="p-1 border shrink-0 {isSelected
										? 'border-white bg-neutral-800 text-white'
										: 'border-black bg-neutral-100 text-black'}"
									title={s.payoutToBank ? `Bankovní účet: ${s.iban}` : 'V hotovosti na místě'}
								>
									{#if s.payoutToBank}
										<CreditCard class="w-3.5 h-3.5" />
									{:else}
										<Banknote class="w-3.5 h-3.5" />
									{/if}
								</div>
							</div>

							<!-- Balances Chip & Unsold Count -->
							<div class="mt-2 pt-2 border-t flex items-center justify-between gap-1.5 text-[11px] {isSelected ? 'border-neutral-700' : 'border-neutral-200'}">
								<!-- Payout status -->
								<div>
									{#if s.totalEarned === 0}
										<span class="font-mono text-[10px] opacity-75">Bez prodejů (0 Kč)</span>
									{:else if hasUnpaid}
										<span class="font-bold {isSelected ? 'text-amber-300' : 'text-amber-700'}">
											Zbývá: {s.totalUnpaid} Kč
										</span>
									{:else if isFullyPaid}
										<span class="font-bold {isSelected ? 'text-emerald-400' : 'text-emerald-700'}">
											✓ Vyplaceno ({s.totalPaid} Kč)
										</span>
									{/if}
								</div>

								<!-- Unsold Books badge -->
								{#if s.unsoldCount > 0}
									<span class="px-1.5 py-0.5 text-[10px] font-black uppercase border {isSelected ? 'bg-white text-black border-white' : 'bg-neutral-100 text-black border-black'}">
										{s.unsoldCount} k vrácení
									</span>
								{/if}
							</div>
						</button>
					{/each}
				{/if}
			</div>
		</div>

		<!-- Right Column: Selected Seller Details (7 cols on lg) -->
		<div class="lg:col-span-8">
			{#if !selectedSellerId}
				<div class="p-12 text-center bg-white border-2 border-black shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] text-neutral-400">
					<BookOpen class="w-8 h-8 mx-auto mb-2 opacity-40" />
					<p class="font-black text-sm uppercase">Vyberte prodejce ze seznamu</p>
					<p class="text-xs mt-1">Zobrazí se přehled neprodaných knih, prodaných knih a finanční bilance</p>
				</div>
			{:else if isLoadingDetails}
				<div class="p-12 text-center bg-white border-2 border-black shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] text-neutral-500">
					<RefreshCw class="w-6 h-6 animate-spin mx-auto mb-2" />
					<p class="font-black text-xs uppercase">Načítání detailu prodejce...</p>
				</div>
			{:else if sellerDetails}
				{@const s = sellerDetails.seller}
				{@const b = sellerDetails.balances}

				<div class="space-y-4">
					<!-- Seller Header Card with Balances & Payout Actions -->
					<div class="p-4 bg-white border-2 border-black shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] space-y-3">
						<div class="flex flex-col sm:flex-row sm:items-start justify-between gap-3">
							<div>
								<div class="flex items-center gap-2">
									<h2 class="text-base font-black uppercase tracking-tight text-black">
										{s.name || s.email}
									</h2>
									<span class="px-2 py-0.5 text-[10px] font-mono font-bold bg-neutral-100 border border-black">
										ID: {s.id}
									</span>
								</div>
								<p class="text-xs font-mono text-neutral-500">{s.email}</p>
							</div>

							<!-- Payout Preference Chip -->
							<div class="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-black uppercase border-2 border-black {s.payoutToBank ? 'bg-blue-50 text-blue-900 border-blue-900' : 'bg-amber-50 text-amber-900 border-amber-900'}">
								{#if s.payoutToBank}
									<CreditCard class="w-4 h-4 shrink-0" />
									<span class="truncate">Účet: <strong class="font-mono text-[11px]">{s.iban || 'Chybí IBAN'}</strong></span>
								{:else}
									<Banknote class="w-4 h-4 shrink-0" />
									<span>V hotovosti na místě</span>
								{/if}
							</div>
						</div>

						<!-- Financial Balance Overview -->
						<div class="grid grid-cols-3 gap-2 p-3 bg-neutral-100 border border-neutral-300 text-center">
							<div class="p-1">
								<span class="text-[10px] font-bold text-neutral-500 uppercase block">Celkem prodáno</span>
								<span class="text-base font-black text-black">{b.totalEarned} Kč</span>
								<span class="text-[10px] text-neutral-400 font-mono block">({b.soldCount} ks)</span>
							</div>

							<div class="p-1 border-x border-neutral-300">
								<span class="text-[10px] font-bold text-neutral-500 uppercase block">Již vyplaceno</span>
								<span class="text-base font-black text-neutral-800">{b.totalPaid} Kč</span>
								<div class="text-[9px] text-neutral-500 font-mono">
									<span>H: {b.paidInCash} Kč</span> | <span>B: {b.paidInBank} Kč</span>
								</div>
							</div>

							<div class="p-1">
								<span class="text-[10px] font-black uppercase block {b.totalUnpaid > 0 ? 'text-amber-600' : 'text-neutral-500'}">
									Zbývá vyplatit
								</span>
								<span class="text-base font-black {b.totalUnpaid > 0 ? 'text-amber-600' : 'text-emerald-700'}">
									{b.totalUnpaid} Kč
								</span>
								<span class="text-[10px] font-bold uppercase block {b.totalUnpaid === 0 ? 'text-emerald-700' : 'text-amber-600'}">
									{b.totalUnpaid === 0 && b.totalEarned > 0 ? 'Plně vyplaceno' : b.totalUnpaid > 0 ? 'K vyplacení' : '0 Kč'}
								</span>
							</div>
						</div>

						<!-- Action Buttons: Pay Cash / Single Fio XML -->
						<div class="flex flex-wrap items-center gap-2 pt-1">
							<!-- Vyplatit v hotovosti -->
							<button
								type="button"
								onclick={openCashPayoutModal}
								disabled={b.totalUnpaid <= 0}
								class="px-4 py-2.5 bg-yellow-400 hover:bg-yellow-500 disabled:opacity-40 disabled:cursor-not-allowed text-black font-black text-xs uppercase tracking-wider border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:scale-95 transition-all flex items-center gap-1.5 cursor-pointer"
							>
								<Banknote class="w-4 h-4" />
								<span>VYPLATIT V HOTOVOSTI ({b.totalUnpaid} KČ)</span>
							</button>

							<!-- Stáhnout Fio XML pro tohoto prodejce -->
							{#if s.payoutToBank && s.iban}
								<button
									type="button"
									onclick={() => downloadFioXml(s.id)}
									disabled={b.totalUnpaid <= 0 || isDownloadingXml}
									class="px-3 py-2.5 bg-white hover:bg-neutral-100 disabled:opacity-40 disabled:cursor-not-allowed text-black font-bold text-xs uppercase tracking-wider border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:scale-95 transition-all flex items-center gap-1.5 cursor-pointer"
									title="Stáhnout příkaz k úhradě pro tohoto jednoho prodejce"
								>
									<Download class="w-4 h-4" />
									<span>FIO XML PRO PRODEJCE</span>
								</button>
							{/if}
						</div>
					</div>

					<!-- UNPRODANE KNIHY (AT THE TOP!) -->
					<div class="p-4 bg-white border-2 border-black shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] space-y-3">
						<div class="flex items-center justify-between border-b-2 border-black pb-2">
							<div class="flex items-center gap-2">
								<h3 class="text-xs sm:text-sm font-black uppercase tracking-tight text-black">
									NEPRODANÉ KNIHY ({sellerDetails.unsoldBooks.length} ks)
								</h3>
								<span class="text-[10px] font-mono px-2 py-0.5 bg-neutral-100 border border-black font-bold">
									K VYZVEDNUTÍ
								</span>
							</div>

							<!-- Button: VRÁTIT KNIHY -->
							{#if sellerDetails.unsoldBooks.length > 0}
								<button
									type="button"
									onclick={() => (isScannerOpen = true)}
									class="px-3 py-1.5 bg-emerald-600 hover:bg-emerald-700 text-white font-black text-xs uppercase tracking-wider border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:scale-95 transition-all flex items-center gap-1.5 cursor-pointer"
								>
									<RotateCcw class="w-4 h-4" />
									<span>VRÁTIT KNIHY ({sellerDetails.unsoldBooks.length})</span>
								</button>
							{/if}
						</div>

						{#if sellerDetails.unsoldBooks.length === 0}
							<p class="text-xs text-neutral-400 font-bold uppercase py-2">
								Žádné zbývající neprodané knihy k vrácení
							</p>
						{:else}
							<div class="grid grid-cols-1 sm:grid-cols-2 gap-2 max-h-72 overflow-y-auto pr-1">
								{#each sellerDetails.unsoldBooks as book (book.id)}
									{@const col = idToColor(book.id)}
									<div class="p-2 border-2 border-black bg-neutral-50 flex items-center justify-between gap-2">
										<div class="flex items-center gap-2.5 min-w-0">
											<div class="w-2.5 h-10 border border-black shrink-0" style="background-color: {col.bg};"></div>
											<div class="w-8 h-10 border border-black bg-neutral-200 overflow-hidden shrink-0">
												{#if book.photo}
													<img src={getBookThumbnailUrl(book)} alt={book.id} class="w-full h-full object-cover" />
												{/if}
											</div>
											<div class="min-w-0">
												<span class="font-mono text-xs font-black">{book.id}</span>
												<div class="text-[11px] font-black text-black">{book.price} Kč</div>
											</div>
										</div>
										<span class="text-[10px] font-black uppercase px-1.5 py-0.5 border border-emerald-600 bg-emerald-50 text-emerald-700 shrink-0">
											K vyzvednutí
										</span>
									</div>
								{/each}
							</div>
						{/if}
					</div>

					<!-- PRODANE KNIHY -->
					<div class="p-4 bg-white border-2 border-black shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] space-y-3">
						<div class="flex items-center justify-between border-b-2 border-black pb-2">
							<h3 class="text-xs sm:text-sm font-black uppercase tracking-tight text-black">
								PRODANÉ KNIHY ({sellerDetails.soldBooks.length} ks - celkem {b.totalEarned} Kč)
							</h3>
							<span class="text-[10px] font-mono px-2 py-0.5 bg-neutral-100 border border-black font-bold">
								PRODÁNO
							</span>
						</div>

						{#if sellerDetails.soldBooks.length === 0}
							<p class="text-xs text-neutral-400 font-bold uppercase py-2">
								Zatím žádné prodané knihy
							</p>
						{:else}
							<div class="grid grid-cols-1 sm:grid-cols-2 gap-2 max-h-72 overflow-y-auto pr-1">
								{#each sellerDetails.soldBooks as book (book.id)}
									{@const col = idToColor(book.id)}
									<div class="p-2 border-2 border-black bg-white flex items-center justify-between gap-2">
										<div class="flex items-center gap-2.5 min-w-0">
											<div class="w-2.5 h-10 border border-black shrink-0" style="background-color: {col.bg};"></div>
											<div class="w-8 h-10 border border-black bg-neutral-200 overflow-hidden shrink-0">
												{#if book.photo}
													<img src={getBookThumbnailUrl(book)} alt={book.id} class="w-full h-full object-cover" />
												{/if}
											</div>
											<div class="min-w-0">
												<span class="font-mono text-xs font-bold">{book.id}</span>
												<div class="text-[11px] font-black text-black">{book.price} Kč</div>
											</div>
										</div>
										<div class="text-right shrink-0">
											<span class="text-[10px] font-black uppercase px-1.5 py-0.5 border border-neutral-400 bg-neutral-100 text-neutral-700">
												PRODÁNO
											</span>
											{#if book.buyer}
												<p class="text-[9px] text-neutral-400 font-mono mt-0.5 truncate max-w-[100px]">
													{book.buyer.name || book.buyer.email}
												</p>
											{/if}
										</div>
									</div>
								{/each}
							</div>
						{/if}
					</div>

					<!-- VRACENE KNIHY (Already returned) -->
					{#if sellerDetails.returnedBooks && sellerDetails.returnedBooks.length > 0}
						<div class="p-4 bg-white border-2 border-black shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] space-y-3">
							<div class="flex items-center justify-between border-b-2 border-black pb-2">
								<h3 class="text-xs sm:text-sm font-black uppercase tracking-tight text-neutral-700">
									VRÁCENÉ KNIHY ({sellerDetails.returnedBooks.length} ks)
								</h3>
								<span class="text-[10px] font-mono px-2 py-0.5 bg-red-50 text-red-700 border border-red-700 font-bold">
									VRÁCENO PRODEJCI
								</span>
							</div>

							<div class="grid grid-cols-1 sm:grid-cols-2 gap-2 max-h-48 overflow-y-auto pr-1">
								{#each sellerDetails.returnedBooks as book (book.id)}
									{@const col = idToColor(book.id)}
									<div class="p-2 border border-neutral-300 bg-neutral-50 opacity-75 flex items-center justify-between gap-2">
										<div class="flex items-center gap-2 min-w-0">
											<div class="w-2.5 h-8 border border-black shrink-0" style="background-color: {col.bg};"></div>
											<span class="font-mono text-xs line-through text-neutral-600">{book.id}</span>
											<span class="text-xs text-neutral-500">{book.price} Kč</span>
										</div>
										<span class="text-[9px] font-black uppercase px-1.5 py-0.5 bg-red-100 text-red-700 border border-red-300">
											Vráceno
										</span>
									</div>
								{/each}
							</div>
						</div>
					{/if}

					<!-- HISTORIE VÝPLAT (money_returns Ledger) -->
					<div class="p-4 bg-white border-2 border-black shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] space-y-3">
						<h3 class="text-xs sm:text-sm font-black uppercase tracking-tight text-black border-b-2 border-black pb-2">
							HISTORIE VÝPLAT ZŮSTATKU
						</h3>

						{#if sellerDetails.moneyReturns.length === 0}
							<p class="text-xs text-neutral-400 font-bold uppercase py-2">
								Zatím nebyly provedeny žádné výplaty
							</p>
						{:else}
							<div class="space-y-2">
								{#each sellerDetails.moneyReturns as r (r.id)}
									<div class="p-2.5 border-2 border-black bg-neutral-50 flex items-center justify-between gap-3 text-xs">
										<div class="flex items-center gap-2.5 min-w-0">
											<div class="p-1.5 border border-black shrink-0 {r.method === 'bank' ? 'bg-blue-100 text-blue-800' : 'bg-yellow-200 text-yellow-900'}">
												{#if r.method === 'bank'}
													<CreditCard class="w-4 h-4" />
												{:else}
													<Banknote class="w-4 h-4" />
												{/if}
											</div>

											<div class="min-w-0">
												<div class="flex items-center gap-1.5">
													<span class="font-black uppercase text-black">
														{r.method === 'bank' ? 'Bankovní převod' : 'V hotovosti'}
													</span>
													{#if r.variableSymbol}
														<span class="text-[10px] font-mono text-neutral-500 font-bold">
															(VS: {r.variableSymbol})
														</span>
													{/if}
												</div>

												<div class="text-[10px] text-neutral-500 font-mono">
													<span>{r.created.replace('T', ' ').slice(0, 16)}</span>
													{#if r.cashier}
														<span>• Pokladník: {r.cashier.name || r.cashier.email}</span>
													{/if}
													{#if r.fio_transaction_id}
														<span>• Fio ID: {r.fio_transaction_id}</span>
													{/if}
												</div>
											</div>
										</div>

										<div class="text-right shrink-0">
											<span class="text-sm font-black text-emerald-700">
												+{r.amount} Kč
											</span>
										</div>
									</div>
								{/each}
							</div>
						{/if}
					</div>
				</div>
			{/if}
		</div>
	</div>
</div>

<!-- Return Scanner Modal -->
{#if sellerDetails}
	<ReturnScannerModal
		bind:open={isScannerOpen}
		seller={sellerDetails.seller}
		unsoldBooks={sellerDetails.unsoldBooks}
		soldBooks={sellerDetails.soldBooks}
		returnedBooks={sellerDetails.returnedBooks}
		onsuccess={handleBooksReturnedSuccess}
	/>
{/if}

<!-- Cash Payout Confirmation Modal (Standard or Strong Warning) -->
{#if isCashPayoutModalOpen && sellerDetails}
	{@const s = sellerDetails.seller}
	{@const b = sellerDetails.balances}
	{@const prefersBank = s.payoutToBank}

	<div class="fixed inset-0 z-50 flex items-center justify-center p-4">
		<button
			type="button"
			class="fixed inset-0 bg-black/70 cursor-default border-none p-0 w-full h-full"
			aria-label="Zavřít"
			onclick={() => (isCashPayoutModalOpen = false)}
		></button>
		<div
			class="relative z-10 bg-white border-4 border-black w-full max-w-md p-5 shadow-[6px_6px_0px_0px_rgba(0,0,0,1)]"
			role="dialog"
			aria-modal="true"
			tabindex="-1"
		>
			{#if prefersBank}
				<!-- STRONG WARNING MODAL (Seller preferred Bank account!) -->
				<div class="flex items-center gap-2 text-amber-600 mb-3 font-black text-sm uppercase">
					<AlertTriangle class="w-6 h-6 shrink-0" />
					<span>POZOR: PRODEJCE MÁ NASTAVENÝ BANKOVNÍ ÚČET</span>
				</div>

				<div class="p-3 bg-amber-50 border-2 border-amber-500 text-amber-900 text-xs font-medium space-y-2 mb-4">
					<p>
						Prodejce má v systému vyplněn bankovní účet:
						<strong class="font-mono font-black text-black">{s.iban}</strong>.
					</p>
					<p class="font-bold">
						Opravdu mu chcete vyplatit <span class="underline text-black font-black">{b.totalUnpaid} Kč V HOTOVOSTI</span> na místě?
					</p>
					<p class="text-[11px] text-amber-800">
						Pokud potvrdíte, bude vytvořen záznam o vyplacení v hotovosti a částka <strong>již nebude</strong> zahrnuta do příkazu k úhradě pro banku.
					</p>
				</div>

				<label class="flex items-start gap-2 text-xs font-bold text-black mb-5 cursor-pointer select-none">
					<input
						type="checkbox"
						bind:checked={cashPayoutConfirmedWarning}
						class="mt-0.5 w-4 h-4 border-2 border-black"
					/>
					<span>Rozumím a chci vyplatit peníze na místě v hotovosti.</span>
				</label>
			{:else}
				<!-- STANDARD CONFIRMATION (Seller preferred cash) -->
				<div class="flex items-center gap-2 text-black mb-3 font-black text-sm uppercase">
					<Banknote class="w-6 h-6 text-emerald-700" />
					<span>POTVRZENÍ VÝPLATY V HOTOVOSTI</span>
				</div>

				<p class="text-xs font-medium text-neutral-700 mb-5">
					Potvrdit předání <strong class="text-black font-black text-sm">{b.totalUnpaid} Kč</strong> v hotovosti prodejci
					<strong class="text-black font-black">{s.name || s.email}</strong>?
				</p>
			{/if}

			<div class="flex gap-2">
				<button
					type="button"
					onclick={() => (isCashPayoutModalOpen = false)}
					disabled={isCashPayoutSubmitting}
					class="flex-1 py-2.5 bg-neutral-200 hover:bg-neutral-300 font-bold text-xs uppercase border-2 border-black cursor-pointer"
				>
					Zrušit
				</button>
				<button
					type="button"
					onclick={confirmCashPayout}
					disabled={isCashPayoutSubmitting || (prefersBank && !cashPayoutConfirmedWarning)}
					class="flex-1 py-2.5 bg-yellow-400 hover:bg-yellow-500 disabled:opacity-40 disabled:cursor-not-allowed text-black font-black text-xs uppercase border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:scale-95 cursor-pointer flex items-center justify-center gap-1.5"
				>
					{#if isCashPayoutSubmitting}
						<RefreshCw class="w-3.5 h-3.5 animate-spin" />
						<span>Ukládám...</span>
					{:else}
						<CheckCircle2 class="w-4 h-4" />
						<span>Potvrdit výplatu</span>
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Bulk XML Summary Modal -->
{#if isBulkXmlModalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4">
		<button
			type="button"
			class="fixed inset-0 bg-black/70 cursor-default border-none p-0 w-full h-full"
			aria-label="Zavřít"
			onclick={() => (isBulkXmlModalOpen = false)}
		></button>
		<div
			class="relative z-10 bg-white border-4 border-black w-full max-w-md p-5 shadow-[6px_6px_0px_0px_rgba(0,0,0,1)]"
			role="dialog"
			aria-modal="true"
			tabindex="-1"
		>
			<div class="flex items-center gap-2 text-black mb-3 font-black text-sm uppercase">
				<Download class="w-5 h-5" />
				<span>FIO XML DÁVKA K VÝPLATĚ NA ÚČET</span>
			</div>

			<div class="p-4 bg-neutral-100 border-2 border-black space-y-2 mb-4 text-xs">
				<div class="flex justify-between font-bold">
					<span>Počet příjemců k vyplacení:</span>
					<span class="font-black text-sm">{bankSellersWithUnpaid.length}</span>
				</div>
				<div class="flex justify-between font-bold border-t border-neutral-300 pt-2">
					<span>Celková částka k vyplacení:</span>
					<span class="font-black text-base text-emerald-700">{bulkXmlTotalAmount} Kč</span>
				</div>
			</div>

			<p class="text-[11px] text-neutral-600 mb-5">
				Tento soubor XML nahrajete v Internetbankingu Fio banky v sekci <strong>Příkazy &gt; Import plateb</strong> a autorizujete.
				Po odeslání peněz klikněte v aplikaci na tlačítko <strong>FIO SYNCHRONIZACE</strong> pro spárování.
			</p>

			<div class="flex gap-2">
				<button
					type="button"
					onclick={() => (isBulkXmlModalOpen = false)}
					disabled={isDownloadingXml}
					class="flex-1 py-2.5 bg-neutral-200 hover:bg-neutral-300 font-bold text-xs uppercase border-2 border-black cursor-pointer"
				>
					Zavřít
				</button>
				<button
					type="button"
					onclick={() => downloadFioXml()}
					disabled={bankSellersWithUnpaid.length === 0 || isDownloadingXml}
					class="flex-1 py-2.5 bg-black hover:bg-neutral-800 disabled:opacity-40 disabled:cursor-not-allowed text-white font-black text-xs uppercase border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:scale-95 cursor-pointer flex items-center justify-center gap-1.5"
				>
					{#if isDownloadingXml}
						<RefreshCw class="w-3.5 h-3.5 animate-spin" />
						<span>Stahuji...</span>
					{:else}
						<Download class="w-4 h-4" />
						<span>Stáhnout XML dávku</span>
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Fio Sync Result Modal -->
{#if syncResultModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4">
		<button
			type="button"
			class="fixed inset-0 bg-black/70 cursor-default border-none p-0 w-full h-full"
			aria-label="Zavřít"
			onclick={() => (syncResultModal = null)}
		></button>
		<div
			class="relative z-10 bg-white border-4 border-black w-full max-w-md p-5 shadow-[6px_6px_0px_0px_rgba(0,0,0,1)]"
			role="dialog"
			aria-modal="true"
			tabindex="-1"
		>
			<div class="flex items-center gap-2 text-black mb-3 font-black text-sm uppercase">
				<CheckCircle2 class="w-5 h-5 text-emerald-600" />
				<span>VÝSLEDEK FIO SYNCHRONIZACE</span>
			</div>

			<p class="text-xs font-medium text-neutral-800 mb-3">
				{syncResultModal.message}
			</p>

			{#if syncResultModal.matchedPayouts && syncResultModal.matchedPayouts.length > 0}
				<div class="max-h-48 overflow-y-auto space-y-1.5 mb-4 pr-1">
					{#each syncResultModal.matchedPayouts as p}
						<div class="p-2 bg-neutral-50 border border-neutral-300 text-xs flex items-center justify-between">
							<div class="min-w-0 pr-2">
								<p class="font-bold truncate">{p.sellerName || p.sellerEmail}</p>
								<p class="text-[10px] font-mono text-neutral-500">ID: {p.sellerId}</p>
							</div>
							<span class="font-black text-emerald-700 shrink-0">
								+{p.amount} Kč
							</span>
						</div>
					{/each}
				</div>
			{/if}

			<button
				type="button"
				onclick={() => (syncResultModal = null)}
				class="w-full py-2.5 bg-black hover:bg-neutral-800 text-white font-black text-xs uppercase border-2 border-black cursor-pointer"
			>
				Rozumím
			</button>
		</div>
	</div>
{/if}

<!-- Feedback Toast Banner -->
{#if toastMessage}
	<div
		class="fixed bottom-4 right-4 z-50 px-4 py-3 border-2 border-black shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] text-xs font-bold uppercase tracking-wider flex items-center gap-2 {toastType === 'success' ? 'bg-emerald-300 text-black' : 'bg-red-400 text-white'}"
	>
		{#if toastType === 'success'}
			<CheckCircle2 class="w-4 h-4" />
		{:else}
			<AlertTriangle class="w-4 h-4" />
		{/if}
		<span>{toastMessage}</span>
	</div>
{/if}
