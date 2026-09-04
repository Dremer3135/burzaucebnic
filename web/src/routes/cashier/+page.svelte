<script lang="ts">
	import { onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, eventStore } from '$lib/stores.svelte';
	import { pb, getBookThumbnailUrl } from '$lib/pocketbase';
	import { scanFrameForDataMatrix, CAMERA_CONSTRAINTS } from '$lib/scanner';
	import { renderSpaydQRCode } from '$lib/barcodes';
	import {
		Scan,
		Banknote,
		CreditCard,
		CheckCircle2,
		AlertCircle,
		Receipt,
		RefreshCw,
		User
	} from '@lucide/svelte';
	import type { Book } from '$lib/types';

	interface BuyerCartData {
		buyer: { id: string; name: string; email: string };
		books: Book[];
		totalAmount: number;
		event: any;
	}

	let videoElement = $state<HTMLVideoElement | null>(null);
	let canvasElement = $state<HTMLCanvasElement | null>(null);
	let mediaStream = $state<MediaStream | null>(null);
	let scanAnimationId = $state<number | null>(null);

	let isScanning = $state(true);
	let scannedBuyerId = $state<string | null>(null);
	let isLoadingCart = $state(false);
	let cartData = $state<BuyerCartData | null>(null);

	// Modes: null (cart preview) | 'QR' (waiting for payment) | 'SUCCESS' (completed)
	let paymentMode = $state<'QR' | 'SUCCESS' | null>(null);
	let isProcessingPayment = $state(false);
	let errorMessage = $state('');
	let successMessage = $state('');

	// Active payment details
	let currentPaymentInfo = $state<{
		id: string;
		variableSymbol: number;
		totalAmount: number;
		iban: string;
	} | null>(null);
	let qrCanvas = $state<HTMLCanvasElement | null>(null);

	$effect(() => {
		if (auth.user && !auth.isCashier) {
			goto('/');
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
			const stream = await navigator.mediaDevices.getUserMedia(CAMERA_CONSTRAINTS);
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
			await pb.send('/api/cashier/confirm-cash', {
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

<div class="flex-1 max-w-2xl w-full mx-auto p-4 flex flex-col pb-24 bg-white text-black">
	<!-- Top Navigation Bar -->
	<div class="flex items-center justify-between bg-white border-2 border-black p-1 mb-6 text-black">
		<a
			href="/cashier"
			class="flex-1 py-2.5 px-3 text-center text-xs font-black uppercase tracking-wider transition-all bg-black text-white flex items-center justify-center gap-1.5"
		>
			<Scan class="w-4 h-4" />
			POKLADNÍ SKENER
		</a>
		<a
			href="/cashier/payments"
			class="flex-1 py-2.5 px-3 text-center text-xs font-black uppercase tracking-wider text-black hover:bg-neutral-100 transition-all flex items-center justify-center gap-1.5"
		>
			<Receipt class="w-4 h-4" />
			BANKOVNÍ PLATBY
		</a>
	</div>

	<!-- MAIN CASHIER WORKFLOW -->
	{#if isScanning}
		<div class="flex-1 flex flex-col items-center">
			<div class="text-center mb-4">
				<h1 class="text-2xl font-black uppercase tracking-tight text-black mb-1">SKENOVÁNÍ KÓDU ZÁKAZNÍKA</h1>
				<p class="text-xs font-bold text-neutral-600 uppercase">
					Namiřte kameru na kód na mobilu zákazníka
				</p>
			</div>

			<!-- Camera Viewport -->
			<div class="relative w-full max-w-sm aspect-square bg-black border-4 border-black overflow-hidden flex items-center justify-center">
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
					<div class="w-48 h-48 border-4 border-white relative flex items-center justify-center">
						<span class="text-xs font-black uppercase text-black bg-white px-2 py-0.5 border-2 border-black">
							Zaměřte kód
						</span>
					</div>
				</div>
			</div>

			<!-- Manual Buyer ID Entry -->
			<div class="mt-6 w-full max-w-sm bg-neutral-50 p-4 border-2 border-black text-center">
				<p class="text-xs font-black uppercase text-neutral-600 mb-2">NEBO ZADEJTE ID ZÁKAZNÍKA RUČNĚ:</p>
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
						placeholder="ID kupujícího"
						class="flex-1 bg-white border-2 border-black px-3 py-2 text-xs font-black uppercase text-black"
					/>
					<button
						type="submit"
						class="py-2 px-4 bg-black text-white hover:bg-neutral-800 text-xs font-black uppercase tracking-wider border-2 border-black cursor-pointer"
					>
						NAČÍST
					</button>
				</form>
			</div>
		</div>
	{:else if isLoadingCart}
		<div class="flex-1 flex flex-col items-center justify-center py-20">
			<RefreshCw class="w-8 h-8 animate-spin text-black mb-3" />
			<p class="text-xs font-black uppercase tracking-wider text-black">Načítám košík zákazníka...</p>
		</div>
	{:else if cartData && paymentMode === null}
		<!-- CART SUMMARY & PAYMENT METHOD CHOICE -->
		<div class="bg-white border-4 border-black p-6 text-black">
			<!-- Buyer Info Header -->
			<div class="flex items-center justify-between pb-4 border-b-2 border-black mb-4">
				<div class="flex items-center gap-3">
					<div class="p-2.5 bg-neutral-100 border-2 border-black text-black">
						<User class="w-5 h-5" />
					</div>
					<div>
						<div class="text-base font-black uppercase text-black">
							{cartData.buyer.name || 'Zákazník'}
						</div>
						<div class="text-xs text-neutral-600 font-mono font-bold">
							{cartData.buyer.email} ({cartData.buyer.id})
						</div>
					</div>
				</div>

				<button
					onclick={startScanner}
					class="text-xs font-black uppercase text-black border-2 border-black px-3 py-1.5 hover:bg-neutral-100 cursor-pointer"
				>
					SKENOVAT ZNOVU
				</button>
			</div>

			<!-- Books List -->
			<div class="space-y-2 mb-6 max-h-64 overflow-y-auto pr-1">
				{#each cartData.books as book}
					<div class="flex items-center justify-between p-3 border-2 border-black bg-neutral-50 text-xs">
						<div class="flex items-center gap-2.5 min-w-0">
							<div class="w-10 h-12 border border-black bg-white overflow-hidden shrink-0">
								{#if book.photo}
									<img
										src={getBookThumbnailUrl(book)}
										alt={book.id}
										class="w-full h-full object-cover"
									/>
								{/if}
							</div>
							<span class="font-black uppercase text-black truncate">{book.id}</span>
						</div>
						<div class="font-black text-black text-base shrink-0 ml-2">
							{book.price} Kč
						</div>
					</div>
				{/each}
			</div>

			<!-- Total Price -->
			<div class="flex items-center justify-between p-4 border-4 border-black bg-neutral-100 mb-6">
				<span class="text-sm font-black uppercase text-black">CELKEM K ÚHRADĚ:</span>
				<span class="text-3xl font-black text-black">{cartData.totalAmount} Kč</span>
			</div>

			{#if errorMessage}
				<div class="mb-4 p-3 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold flex items-center gap-2">
					<AlertCircle class="w-4 h-4 shrink-0 text-red-600" />
					<span>{errorMessage}</span>
				</div>
			{/if}

			<!-- Payment Action Buttons -->
			<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
				<button
					onclick={handleGenerateQRPayment}
					disabled={isProcessingPayment}
					class="py-4 px-4 bg-white text-black hover:bg-neutral-100 active:bg-neutral-200 font-black text-sm uppercase tracking-wider border-4 border-black transition-all flex items-center justify-center gap-2 cursor-pointer"
				>
					<CreditCard class="w-5 h-5" />
					<span>ZAPLATIT QR KÓDEM</span>
				</button>

				<button
					onclick={handleConfirmCash}
					disabled={isProcessingPayment}
					class="py-4 px-4 bg-black text-white hover:bg-neutral-800 active:bg-neutral-900 font-black text-sm uppercase tracking-wider border-4 border-black transition-all flex items-center justify-center gap-2 cursor-pointer"
				>
					<Banknote class="w-5 h-5" />
					<span>ZAPLATIT HOTOVĚ</span>
				</button>
			</div>
		</div>
	{:else if paymentMode === 'QR' && currentPaymentInfo}
		<!-- QR PAYMENT DISPLAY -->
		<div class="bg-white border-4 border-black p-6 text-center text-black">
			<h2 class="text-2xl font-black uppercase tracking-tight text-black mb-1">QR PLATBA</h2>
			<p class="text-xs font-bold text-neutral-600 uppercase mb-4">
				Ukažte tento QR kód zákazníkovi k načtení v bance
			</p>

			<!-- QR Code Canvas -->
			<div class="bg-white p-4 border-4 border-black inline-block mx-auto mb-4">
				<canvas bind:this={qrCanvas} class="max-w-full h-auto"></canvas>
			</div>

			<!-- Payment Details -->
			<div class="bg-neutral-50 border-2 border-black p-4 text-xs text-left max-w-sm mx-auto space-y-2 mb-6">
				<div class="flex justify-between border-b border-neutral-300 pb-1">
					<span class="font-bold text-neutral-600 uppercase">ČÁSTKA:</span>
					<span class="font-black text-black text-base">{currentPaymentInfo.totalAmount} Kč</span>
				</div>
				<div class="flex justify-between border-b border-neutral-300 pb-1">
					<span class="font-bold text-neutral-600 uppercase">VARIABILNÍ SYMBOL:</span>
					<span class="font-mono font-black text-black text-sm">
						{currentPaymentInfo.variableSymbol}
					</span>
				</div>
				<div class="flex justify-between border-b border-neutral-300 pb-1">
					<span class="font-bold text-neutral-600 uppercase">ÚČET (IBAN):</span>
					<span class="font-mono font-bold text-black">{currentPaymentInfo.iban}</span>
				</div>
				<div class="flex justify-between">
					<span class="font-bold text-neutral-600 uppercase">ID PLATBY:</span>
					<span class="font-mono text-neutral-600 text-[10px]">{currentPaymentInfo.id}</span>
				</div>
			</div>

			<div class="flex gap-2 max-w-sm mx-auto">
				<a
					href="/cashier/payments"
					class="flex-1 py-3 px-4 bg-white hover:bg-neutral-100 text-black font-black text-xs uppercase tracking-wider border-2 border-black flex items-center justify-center gap-1.5 transition-colors"
				>
					<Receipt class="w-4 h-4" />
					PŘEHLED PLATEB
				</a>
				<button
					onclick={startScanner}
					class="flex-1 py-3 px-4 bg-black hover:bg-neutral-800 text-white font-black text-xs uppercase tracking-wider border-2 border-black transition-colors cursor-pointer"
				>
					DALŠÍ ZÁKAZNÍK
				</button>
			</div>
		</div>
	{:else if paymentMode === 'SUCCESS'}
		<!-- CASH PAYMENT SUCCESS -->
		<div class="bg-white border-4 border-black p-8 text-center text-black">
			<div class="inline-flex p-4 bg-neutral-100 border-2 border-black mb-4">
				<CheckCircle2 class="w-12 h-12 text-black" />
			</div>
			<h2 class="text-2xl font-black uppercase tracking-tight text-black mb-2">PLATBA DOKONČENA!</h2>
			<p class="text-xs font-bold text-neutral-600 uppercase max-w-sm mx-auto mb-6">
				{successMessage}
			</p>

			<button
				onclick={startScanner}
				class="w-full max-w-xs py-4 px-6 bg-black hover:bg-neutral-800 text-white font-black text-sm uppercase tracking-wider border-2 border-black mx-auto cursor-pointer"
			>
				SKENOVAT DALŠÍHO ZÁKAZNÍKA
			</button>
		</div>
	{/if}
</div>
