<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, eventStore } from '$lib/stores.svelte';
	import { pb, getBookThumbnailUrl } from '$lib/pocketbase';
	import { scanFrameForDataMatrix } from '$lib/scanner';
	import { renderSpaydQRCode } from '$lib/barcodes';
	import {
		Camera,
		Scan,
		CreditCard,
		Banknote,
		CheckCircle2,
		AlertCircle,
		RefreshCw,
		ArrowRight,
		User,
		Receipt
	} from '@lucide/svelte';

	interface BuyerCartData {
		buyer: { id: string; name?: string; email: string };
		books: Array<{ id: string; code: string; price: number; photo: string }>;
		totalAmount: number;
		event?: any;
	}

	let videoElement = $state<HTMLVideoElement | null>(null);
	let canvasElement = $state<HTMLCanvasElement | null>(null);
	let mediaStream = $state<MediaStream | null>(null);
	let scanAnimationId = $state<number | null>(null);

	let isScanning = $state(true);
	let scannedBuyerId = $state<string | null>(null);
	let cartData = $state<BuyerCartData | null>(null);
	let isLoadingCart = $state(false);
	let errorMessage = $state('');

	// Payment flow: null | 'QR' | 'CASH' | 'SUCCESS'
	let paymentMode = $state<'QR' | 'CASH' | 'SUCCESS' | null>(null);
	let isProcessingPayment = $state(false);
	let successMessage = $state('');

	// QR Canvas
	let qrCanvas = $state<HTMLCanvasElement | null>(null);
	let currentPaymentInfo = $state<{
		id: string;
		variableSymbol: number;
		totalAmount: number;
		iban: string;
	} | null>(null);

	$effect(() => {
		if (auth.user && !auth.isCashier) {
			goto('/');
		}
	});

	onMount(() => {
		if (auth.isCashier) {
			startScanner();
		}
	});

	onDestroy(() => {
		stopScanner();
	});

	async function startScanner() {
		stopScanner();
		isScanning = true;
		errorMessage = '';
		paymentMode = null;
		cartData = null;
		scannedBuyerId = null;

		try {
			const stream = await navigator.mediaDevices.getUserMedia({
				video: { facingMode: 'environment', width: { ideal: 1280 }, height: { ideal: 720 } }
			});
			mediaStream = stream;
			if (videoElement) {
				videoElement.srcObject = stream;
				await videoElement.play();
				runScanLoop();
			}
		} catch (err) {
			console.error(err);
			errorMessage = 'Nelze spustit kameru pro pokladnu.';
		}
	}

	function stopScanner() {
		if (scanAnimationId) {
			cancelAnimationFrame(scanAnimationId);
			scanAnimationId = null;
		}
		if (mediaStream) {
			mediaStream.getTracks().forEach((t) => t.stop());
			mediaStream = null;
		}
		isScanning = false;
	}

	let isScanningFrame = false;
	let lastScanTime = 0;

	async function runScanLoop() {
		if (!videoElement || !canvasElement || !isScanning) return;

		const now = performance.now();
		if (!isScanningFrame && now - lastScanTime >= 65 && videoElement.videoWidth > 0) {
			isScanningFrame = true;
			lastScanTime = now;
			try {
				const match = await scanFrameForDataMatrix(canvasElement, videoElement);
				if (match && match.text) {
					stopScanner();
					scannedBuyerId = match.text.trim();
					if (navigator.vibrate) navigator.vibrate([100, 50, 100]);
					loadBuyerCart(scannedBuyerId);
					return;
				}
			} catch (err) {
				console.error(err);
			} finally {
				isScanningFrame = false;
			}
		}

		scanAnimationId = requestAnimationFrame(runScanLoop);
	}


	async function loadBuyerCart(buyerId: string) {
		isLoadingCart = true;
		errorMessage = '';
		try {
			const data = await pb.send<BuyerCartData>('/api/cashier/buyer-cart', {
				method: 'GET',
				query: { buyerId }
			});
			cartData = data;
			if (!data.books || data.books.length === 0) {
				errorMessage = 'Tento zákazník nemá v košíku žádné rezervované knihy.';
			}
		} catch (err: any) {
			console.error(err);
			errorMessage = err?.message || 'Chyba při načítání nákupu zákazníka.';
		} finally {
			isLoadingCart = false;
		}
	}

	async function handleConfirmCash() {
		if (!cartData || !scannedBuyerId) return;
		isProcessingPayment = true;
		errorMessage = '';
		try {
			const bookIds = cartData.books.map((b) => b.id);
			const res = await pb.send('/api/cashier/confirm-cash', {
				method: 'POST',
				body: {
					buyerId: scannedBuyerId,
					bookIds
				}
			});
			paymentMode = 'SUCCESS';
			successMessage = `Hotovostní platba ve výši ${cartData.totalAmount} Kč byla úspěšně potvrzena. Učebnice byly označeny jako prodané.`;
		} catch (err: any) {
			console.error(err);
			errorMessage = err?.message || 'Chyba při potvrzování hotovostní platby.';
		} finally {
			isProcessingPayment = false;
		}
	}

	async function handleGenerateQRPayment() {
		if (!cartData || !scannedBuyerId) return;
		isProcessingPayment = true;
		errorMessage = '';
		try {
			const bookIds = cartData.books.map((b) => b.id);
			const res = await pb.send<{ success: boolean; payment: any; event: any }>('/api/cashier/create-qr-payment', {
				method: 'POST',
				body: {
					buyerId: scannedBuyerId,
					bookIds
				}
			});

			const p = res.payment;
			const ev = res.event || eventStore.event;
			const iban = ev?.iban || 'CZ6520100000002101234567';

			currentPaymentInfo = {
				id: p.id,
				variableSymbol: p.variableSymbol,
				totalAmount: p.totalAmount,
				iban: iban
			};
			paymentMode = 'QR';

			setTimeout(() => {
				if (qrCanvas && currentPaymentInfo) {
					renderSpaydQRCode(qrCanvas, {
						iban: currentPaymentInfo.iban,
						amount: currentPaymentInfo.totalAmount,
						vs: currentPaymentInfo.variableSymbol,
						paymentId: currentPaymentInfo.id
					});
				}
			}, 100);
		} catch (err: any) {
			console.error(err);
			errorMessage = err?.message || 'Chyba při generování QR platby.';
		} finally {
			isProcessingPayment = false;
		}
	}
</script>

<div class="flex-1 max-w-2xl w-full mx-auto p-4 flex flex-col pb-20">
	<!-- Top Navigation Bar between Scanner and Payments -->
	<div class="flex items-center justify-between bg-slate-800/80 p-1.5 rounded-2xl border border-slate-700/80 mb-6 backdrop-blur">
		<a
			href="/cashier"
			class="flex-1 py-2 px-3 rounded-xl text-center text-xs font-bold transition-all bg-emerald-600 text-white shadow-sm flex items-center justify-center gap-1.5"
		>
			<Scan class="w-4 h-4" />
			Pokladní skener
		</a>
		<a
			href="/cashier/payments"
			class="flex-1 py-2 px-3 rounded-xl text-center text-xs font-semibold text-slate-400 hover:text-slate-200 transition-all flex items-center justify-center gap-1.5"
		>
			<Receipt class="w-4 h-4" />
			Bankovní platby
		</a>
	</div>

	<!-- MAIN CASHIER WORKFLOW -->
	{#if isScanning}
		<div class="flex-1 flex flex-col items-center">
			<div class="text-center mb-4">
				<h1 class="text-xl font-bold text-white mb-1">Skenování kódu zákazníka</h1>
				<p class="text-xs text-slate-400">
					Namiřte kameru na velký kód Data Matrix na mobilu zákazníka
				</p>
			</div>

			<!-- Camera Viewport -->
			<div class="relative w-full max-w-sm aspect-square bg-black rounded-3xl overflow-hidden border-2 border-slate-700 shadow-2xl flex items-center justify-center">
				<video
					bind:this={videoElement}
					playsinline
					autoplay
					muted
					class="w-full h-full object-cover"
				></video>
				<canvas bind:this={canvasElement} class="hidden"></canvas>

				<!-- Target Guide Box -->
				<div class="absolute inset-0 pointer-events-none flex items-center justify-center p-8">
					<div class="w-48 h-48 border-2 border-emerald-400 rounded-2xl relative animate-pulse flex items-center justify-center">
						<div class="absolute inset-0 bg-emerald-500/10 rounded-2xl"></div>
						<div class="absolute -top-1 -left-1 w-6 h-6 border-t-4 border-l-4 border-emerald-400 rounded-tl-lg"></div>
						<div class="absolute -top-1 -right-1 w-6 h-6 border-t-4 border-r-4 border-emerald-400 rounded-tr-lg"></div>
						<div class="absolute -bottom-1 -left-1 w-6 h-6 border-b-4 border-l-4 border-emerald-400 rounded-bl-lg"></div>
						<div class="absolute -bottom-1 -right-1 w-6 h-6 border-b-4 border-r-4 border-emerald-400 rounded-br-lg"></div>
					</div>
				</div>
			</div>

			<!-- Or Manual Buyer ID Entry for Testing -->
			<div class="mt-6 w-full max-w-sm bg-slate-800/60 p-3 rounded-xl border border-slate-700/60 text-center">
				<p class="text-[11px] text-slate-400 mb-2">Nebo zadejte ID zákazníka ručně:</p>
				<form
					onsubmit={(e) => {
						e.preventDefault();
						const form = e.currentTarget as HTMLFormElement;
						const input = form.elements.namedItem('manualId') as HTMLInputElement;
						if (input?.value) {
							stopScanner();
							scannedBuyerId = input.value.trim();
							loadBuyerCart(scannedBuyerId);
						}
					}}
					class="flex gap-2"
				>
					<input
						name="manualId"
						type="text"
						placeholder="Např. ID kupujícího"
						class="flex-1 bg-slate-900 border border-slate-700 rounded-lg px-3 py-1.5 text-xs text-white"
					/>
					<button
						type="submit"
						class="py-1.5 px-3 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-bold"
					>
						Načíst
					</button>
				</form>
			</div>
		</div>
	{:else if isLoadingCart}
		<div class="flex-1 flex flex-col items-center justify-center py-20">
			<RefreshCw class="w-8 h-8 animate-spin text-emerald-400 mb-3" />
			<p class="text-xs text-slate-300">Načítám košík zákazníka...</p>
		</div>
	{:else if cartData && paymentMode === null}
		<!-- CART SUMMARY & PAYMENT METHOD CHOICE -->
		<div class="bg-slate-800/90 border border-slate-700 rounded-3xl p-5 shadow-2xl backdrop-blur">
			<!-- Buyer Info Header -->
			<div class="flex items-center justify-between pb-4 border-b border-slate-700/80 mb-4">
				<div class="flex items-center gap-2.5">
					<div class="p-2 bg-emerald-500/10 text-emerald-400 rounded-xl">
						<User class="w-5 h-5" />
					</div>
					<div>
						<div class="text-sm font-bold text-white">
							{cartData.buyer.name || 'Zákazník'}
						</div>
						<div class="text-[11px] text-slate-400 font-mono">
							{cartData.buyer.email} ({cartData.buyer.id})
						</div>
					</div>
				</div>

				<button
					onclick={startScanner}
					class="text-xs text-slate-400 hover:text-white px-2.5 py-1.5 rounded-lg bg-slate-700/50"
				>
					Skenovat znovu
				</button>
			</div>

			<!-- Books List -->
			<div class="space-y-2 mb-6 max-h-64 overflow-y-auto pr-1">
				{#each cartData.books as book}
					<div class="flex items-center justify-between p-2.5 rounded-xl bg-slate-900/60 border border-slate-800 text-xs">
						<div class="flex items-center gap-2 min-w-0">
							<div class="w-8 h-10 rounded bg-slate-800 overflow-hidden shrink-0">
								{#if book.photo}
									<img
										src={getBookThumbnailUrl(book)}
										alt={book.code}
										class="w-full h-full object-cover"
									/>
								{/if}
							</div>
							<span class="font-medium text-slate-200 truncate">{book.code}</span>
						</div>
						<div class="font-bold text-emerald-400 text-sm shrink-0 ml-2">
							{book.price} Kč
						</div>
					</div>
				{/each}
			</div>

			<!-- Total Price -->
			<div class="flex items-center justify-between p-4 rounded-2xl bg-emerald-950/40 border border-emerald-500/30 mb-6">
				<span class="text-sm text-slate-300 font-medium">Celkem k úhradě:</span>
				<span class="text-2xl font-black text-emerald-400">{cartData.totalAmount} Kč</span>
			</div>

			{#if errorMessage}
				<div class="mb-4 p-3 bg-rose-500/15 border border-rose-500/30 text-rose-300 text-xs rounded-xl flex items-center gap-2">
					<AlertCircle class="w-4 h-4 shrink-0" />
					<span>{errorMessage}</span>
				</div>
			{/if}

			<!-- Payment Action Buttons -->
			<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
				<button
					onclick={handleGenerateQRPayment}
					disabled={isProcessingPayment}
					class="py-3.5 px-4 rounded-xl bg-blue-600 hover:bg-blue-500 active:scale-95 text-white font-bold text-sm transition-all shadow-lg flex items-center justify-center gap-2 cursor-pointer"
				>
					<CreditCard class="w-5 h-5" />
					<span>Zaplatit QR kódem</span>
				</button>

				<button
					onclick={handleConfirmCash}
					disabled={isProcessingPayment}
					class="py-3.5 px-4 rounded-xl bg-emerald-600 hover:bg-emerald-500 active:scale-95 text-white font-bold text-sm transition-all shadow-lg flex items-center justify-center gap-2 cursor-pointer"
				>
					<Banknote class="w-5 h-5" />
					<span>Zaplatit hotově</span>
				</button>
			</div>
		</div>
	{:else if paymentMode === 'QR' && currentPaymentInfo}
		<!-- QR PAYMENT DISPLAY -->
		<div class="bg-slate-800/90 border border-slate-700 rounded-3xl p-6 shadow-2xl backdrop-blur text-center">
			<h2 class="text-lg font-bold text-white mb-1">QR Platba (Převod na účet)</h2>
			<p class="text-xs text-slate-400 mb-4">
				Ukažte tento QR kód zákazníkovi k načtení v bankovní aplikaci
			</p>

			<!-- QR Code Canvas -->
			<div class="bg-white p-3.5 rounded-2xl inline-block shadow-2xl mx-auto mb-4">
				<canvas bind:this={qrCanvas} class="max-w-full h-auto"></canvas>
			</div>

			<!-- Payment Details -->
			<div class="bg-slate-900/80 rounded-xl p-3.5 text-xs text-left max-w-sm mx-auto space-y-1.5 border border-slate-800 mb-6">
				<div class="flex justify-between">
					<span class="text-slate-400">Částka:</span>
					<span class="font-bold text-emerald-400 text-sm">{currentPaymentInfo.totalAmount} Kč</span>
				</div>
				<div class="flex justify-between">
					<span class="text-slate-400">Variabilní symbol:</span>
					<span class="font-mono font-bold text-white bg-slate-800 px-2 py-0.5 rounded">
						{currentPaymentInfo.variableSymbol}
					</span>
				</div>
				<div class="flex justify-between">
					<span class="text-slate-400">Účet (IBAN):</span>
					<span class="font-mono text-slate-300">{currentPaymentInfo.iban}</span>
				</div>
				<div class="flex justify-between">
					<span class="text-slate-400">Zpráva pro příjemce (ID):</span>
					<span class="font-mono text-slate-400 text-[10px]">{currentPaymentInfo.id}</span>
				</div>
			</div>

			<div class="flex gap-2 max-w-sm mx-auto">
				<a
					href="/cashier/payments"
					class="flex-1 py-3 px-4 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-bold text-xs transition-colors flex items-center justify-center gap-1.5 shadow"
				>
					<Receipt class="w-4 h-4" />
					Ověřit v přehledu plateb
				</a>
				<button
					onclick={startScanner}
					class="flex-1 py-3 px-4 rounded-xl bg-slate-700 hover:bg-slate-600 text-slate-200 font-medium text-xs transition-colors cursor-pointer"
				>
					Další zákazník
				</button>
			</div>
		</div>
	{:else if paymentMode === 'SUCCESS'}
		<!-- CASH PAYMENT SUCCESS -->
		<div class="bg-slate-800/90 border border-emerald-500/50 rounded-3xl p-8 shadow-2xl backdrop-blur text-center">
			<div class="inline-flex p-4 bg-emerald-500/15 text-emerald-400 rounded-full mb-4">
				<CheckCircle2 class="w-12 h-12" />
			</div>
			<h2 class="text-xl font-bold text-white mb-2">Platba dokončena!</h2>
			<p class="text-xs text-slate-300 max-w-sm mx-auto mb-6">
				{successMessage}
			</p>

			<button
				onclick={startScanner}
				class="w-full max-w-xs py-3.5 px-6 rounded-xl bg-emerald-600 hover:bg-emerald-500 active:scale-95 text-white font-bold text-sm transition-all shadow-lg shadow-emerald-950 mx-auto cursor-pointer"
			>
				Skenovat dalšího zákazníka
			</button>
		</div>
	{/if}
</div>
