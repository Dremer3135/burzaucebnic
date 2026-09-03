<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, eventStore } from '$lib/stores.svelte';
	import { cart, type CartItem } from '$lib/cart.svelte';
	import { pb, getBookThumbnailUrl, getBookFullImageUrl } from '$lib/pocketbase';
	import {
		scanFrameForDataMatrix,
		drawBoundingBox,
		getVideoTransform,
		CAMERA_CONSTRAINTS,
		type ScanMatch,
		type VideoTransform
	} from '$lib/scanner';
	import { renderDataMatrix } from '$lib/barcodes';
	import {
		Scan,
		ShoppingBag,
		Trash2,
		ArrowLeft,
		Check,
		AlertTriangle,
		X,
		Camera,
		QrCode
	} from '@lucide/svelte';
	import type { Book } from '$lib/types';

	let isScannerOpen = $state(false);
	let isCheckoutModalOpen = $state(false);
	let isCheckingOut = $state(false);
	let checkoutError = $state('');

	let videoElement = $state<HTMLVideoElement | null>(null);
	let canvasElement = $state<HTMLCanvasElement | null>(null);
	let overlayCanvas = $state<HTMLCanvasElement | null>(null);
	let mediaStream = $state<MediaStream | null>(null);
	let scanAnimationId = $state<number | null>(null);

	// Multi-book active scan subscriptions: bookId -> { book, unsub, timer }
	const activeScanSubscriptions = new Map<string, { book: Book; unsub: () => void; timer: any }>();
	const pendingFetches = new Set<string>();

	let currentDetectedCode = $state<string | null>(null);
	let currentBook = $state<Book | null>(null);
	let widgetPosition = $state<{ x: number; y: number } | null>(null);
	let lastCheckedOutItems = $state<CartItem[]>([]);

	let isScanningFrame = false;
	let lastScanTime = 0;

	// Full-screen user ID canvas
	let userIdCanvas = $state<HTMLCanvasElement | null>(null);

	// Full-res preview modal
	let selectedPreviewBook = $state<Book | null>(null);

	$effect(() => {
		if (!auth.user) {
			goto('/');
		}
	});

	onDestroy(() => {
		stopCamera();
		cleanupScanSubscriptions();
	});

	function cleanupScanSubscriptions() {
		for (const [id, item] of activeScanSubscriptions.entries()) {
			clearTimeout(item.timer);
			if (!cart.has(id)) {
				item.unsub();
			}
		}
		activeScanSubscriptions.clear();
		pendingFetches.clear();
		currentBook = null;
		currentDetectedCode = null;
		widgetPosition = null;
	}

	async function openScanner() {
		isScannerOpen = true;
		cleanupScanSubscriptions();
		setTimeout(startCamera, 100);
	}

	function closeScanner() {
		stopCamera();
		cleanupScanSubscriptions();
		isScannerOpen = false;
	}

	async function startCamera() {
		stopCamera();
		try {
			const stream = await navigator.mediaDevices.getUserMedia(CAMERA_CONSTRAINTS);
			mediaStream = stream;
			if (videoElement) {
				videoElement.srcObject = stream;
				await videoElement.play();
				startScanningLoop();
			}
		} catch (err: any) {
			console.error('Camera error', err);
			alert('Nelze spustit kameru. Povolte prosím přístup v prohlížeči.');
			isScannerOpen = false;
		}
	}

	function stopCamera() {
		if (scanAnimationId) {
			cancelAnimationFrame(scanAnimationId);
			scanAnimationId = null;
		}
		if (mediaStream) {
			mediaStream.getTracks().forEach((t) => t.stop());
			mediaStream = null;
		}
	}

	async function startScanningLoop() {
		if (!videoElement || !canvasElement || !overlayCanvas || !isScannerOpen) return;

		const video = videoElement;
		const overlay = overlayCanvas;
		const ctx = overlay.getContext('2d');

		if (overlay.width !== video.clientWidth || overlay.height !== video.clientHeight) {
			overlay.width = video.clientWidth;
			overlay.height = video.clientHeight;
		}

		if (ctx) {
			ctx.clearRect(0, 0, overlay.width, overlay.height);
		}

		const now = performance.now();
		if (!isScanningFrame && now - lastScanTime >= 65 && video.videoWidth > 0 && video.videoHeight > 0) {
			isScanningFrame = true;
			lastScanTime = now;
			try {
				const match = await scanFrameForDataMatrix(canvasElement, video);

				if (match && isScannerOpen) {
					const transform = getVideoTransform(video.videoWidth, video.videoHeight, overlay.width, overlay.height);

					if (ctx) {
						drawBoundingBox(ctx, match.position, transform, '#000000');
					}

					const screenX = match.box.x * transform.scale + transform.offsetX;
					const screenY = match.box.y * transform.scale + transform.offsetY;
					const screenW = match.box.width * transform.scale;
					const screenH = match.box.height * transform.scale;

					let posX = Math.max(16, Math.min(overlay.width - 240, screenX + screenW / 2 - 110));
					let posY = screenY + screenH + 16;
					if (posY + 90 > overlay.height) {
						posY = Math.max(16, screenY - 90);
					}

					widgetPosition = { x: posX, y: posY };
					handleDetectedCode(match.text.trim());
				} else {
					widgetPosition = null;
					currentDetectedCode = null;
				}
			} catch (err) {
				console.error(err);
			} finally {
				isScanningFrame = false;
			}
		}

		scanAnimationId = requestAnimationFrame(startScanningLoop);
	}

	async function handleDetectedCode(code: string) {
		currentDetectedCode = code;

		for (const item of activeScanSubscriptions.values()) {
			if (item.book.code === code) {
				currentBook = item.book;
				clearTimeout(item.timer);
				item.timer = setTimeout(() => {
					if (!cart.has(item.book.id)) {
						item.unsub();
						activeScanSubscriptions.delete(item.book.id);
					}
				}, 10000);
				return;
			}
		}

		if (pendingFetches.has(code)) return;
		pendingFetches.add(code);

		try {
			const book = await pb.collection('books').getFirstListItem<Book>(
				pb.filter('code = {:code}', { code })
			);

			if (currentDetectedCode === code) {
				currentBook = book;
			}

			const unsub = await pb.collection('books').subscribe<Book>(book.id, (e) => {
				if (e.action === 'update') {
					const entry = activeScanSubscriptions.get(book.id);
					if (entry) entry.book = e.record;
					if (currentBook?.id === book.id) {
						currentBook = e.record;
					}
				} else if (e.action === 'delete') {
					if (currentBook?.id === book.id) {
						currentBook = { ...book, status: 'bought' };
					}
				}
			});

			const timer = setTimeout(() => {
				if (!cart.has(book.id)) {
					unsub();
					activeScanSubscriptions.delete(book.id);
				}
			}, 10000);

			activeScanSubscriptions.set(book.id, { book, unsub, timer });
		} catch (err) {
			console.warn(`Book with code ${code} not found in PocketBase`);
		} finally {
			pendingFetches.delete(code);
		}
	}

	function handleAddToCart(book: Book) {
		if (book.status !== 'available') return;
		cart.addBook(book);
		if (navigator.vibrate) navigator.vibrate([80, 50, 80]);
	}

	async function handleCheckout() {
		if (cart.items.length === 0 || cart.hasUnavailable) return;
		isCheckingOut = true;
		checkoutError = '';

		try {
			lastCheckedOutItems = [...cart.items];
			const bookIds = cart.items.map((i) => i.book.id);
			await pb.send('/api/checkout', {
				method: 'POST',
				body: { bookIds }
			});

			cart.clear();
			isCheckoutModalOpen = true;

			setTimeout(() => {
				if (userIdCanvas && auth.user?.id) {
					renderDataMatrix(userIdCanvas, auth.user.id);
				}
			}, 150);
		} catch (err: any) {
			console.error('Checkout failed', err);
			checkoutError = err?.message || 'Chyba při rezervaci knih. Některá z knih již nemusí být dostupná.';
			await cart.refreshItemsStatus();
		} finally {
			isCheckingOut = false;
		}
	}

	async function handleCancelCheckout() {
		try {
			await pb.send('/api/checkout/cancel', { method: 'POST' });
			if (lastCheckedOutItems.length > 0) {
				cart.restoreItems(lastCheckedOutItems);
				lastCheckedOutItems = [];
			}
		} catch (err) {
			console.error(err);
		} finally {
			isCheckoutModalOpen = false;
		}
	}
</script>

<div class="flex-1 max-w-4xl w-full mx-auto p-4 flex flex-col pb-28 bg-white text-black">
	<!-- Page Header -->
	<div class="flex items-center justify-between mb-4 border-b-2 border-black pb-3">
		<div>
			<h1 class="text-2xl font-black uppercase tracking-tight text-black">NÁKUPNÍ KOŠÍK</h1>
			<p class="text-xs font-bold text-neutral-600 uppercase">
				Naskenujte učebnice a přejděte k pokladně
			</p>
		</div>

		{#if cart.count > 0}
			<button
				onclick={() => cart.clear()}
				class="text-xs font-black uppercase text-black hover:bg-black hover:text-white border-2 border-black px-2.5 py-1.5 flex items-center gap-1 transition-colors cursor-pointer"
			>
				<Trash2 class="w-3.5 h-3.5" />
				VYSYPAT
			</button>
		{/if}
	</div>

	<!-- Cart Items List -->
	{#if cart.items.length === 0}
		<div class="flex-1 flex flex-col items-center justify-center p-8 bg-neutral-50 border-2 border-dashed border-black text-center my-6">
			<div class="p-4 bg-white border-2 border-black mb-3">
				<ShoppingBag class="w-8 h-8 text-black" />
			</div>
			<h3 class="text-lg font-black uppercase tracking-tight text-black mb-1">Košík je prázdný</h3>
			<p class="text-xs font-bold text-neutral-600 uppercase max-w-xs mb-6">
				Namiřte kameru na kód Data Matrix na učebnici.
			</p>
			<button
				onclick={openScanner}
				class="inline-flex items-center gap-2 py-3 px-6 bg-black text-white font-black text-sm uppercase tracking-wider hover:bg-neutral-800 active:bg-neutral-900 border-2 border-black transition-colors cursor-pointer"
			>
				<Scan class="w-5 h-5" />
				SKENOVAT UČEBNICI
			</button>
		</div>
	{:else}
		{#if cart.hasUnavailable}
			<div class="mb-4 p-3 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold flex items-center gap-2">
				<AlertTriangle class="w-4 h-4 shrink-0 text-red-600" />
				<span>Některé knihy v košíku byly mezitím rezervovány nebo prodány. Před dokončením je odstraňte.</span>
			</div>
		{/if}

		{#if checkoutError}
			<div class="mb-4 p-3 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold flex items-center gap-2">
				<AlertTriangle class="w-4 h-4 shrink-0 text-red-600" />
				<span>{checkoutError}</span>
			</div>
		{/if}

		<div class="space-y-3 flex-1">
			{#each cart.items as item (item.book.id)}
				<div
					class="p-3 flex gap-3 items-center border-2 border-black transition-all {item.isAvailable
						? 'bg-white text-black'
						: 'bg-neutral-100 border-dashed border-red-600 text-neutral-500'}"
				>
					<!-- Thumbnail -->
					<button
						type="button"
						onclick={() => (selectedPreviewBook = item.book)}
						class="w-16 h-22 shrink-0 border-2 border-black overflow-hidden bg-neutral-100 relative group cursor-pointer"
					>
						{#if item.book.photo}
							<img
								src={getBookThumbnailUrl(item.book)}
								alt={item.book.code}
								class="w-full h-full object-cover"
							/>
						{:else}
							<div class="w-full h-full flex items-center justify-center text-neutral-400">
								<Camera class="w-5 h-5" />
							</div>
						{/if}
						<div class="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center text-[10px] text-white font-black uppercase">
							Lupa
						</div>
					</button>

					<!-- Details -->
					<div class="flex-1 min-w-0">
						<div class="flex items-center gap-2 mb-1">
							<span class="text-sm font-black text-black truncate">{item.book.code}</span>
							{#if !item.isAvailable}
								<span class="text-[10px] bg-red-600 text-white font-black px-1.5 py-0.5 uppercase">
									NEDOSTUPNÉ
								</span>
							{/if}
						</div>
						<div class="text-xl font-black text-black">
							{item.book.price} Kč
						</div>
					</div>

					<!-- Delete from cart button -->
					<button
						onclick={() => cart.removeBook(item.book.id)}
						class="p-2 border-2 border-black bg-white text-black hover:bg-black hover:text-white transition-colors cursor-pointer"
						title="Odebrat z košíku"
					>
						<Trash2 class="w-4 h-4" />
					</button>
				</div>
			{/each}
		</div>

		<!-- Bottom Checkout Bar -->
		<div class="sticky bottom-4 mt-6 bg-white border-4 border-black p-4 flex items-center justify-between gap-4">
			<div>
				<div class="text-xs font-bold text-neutral-600 uppercase">CELKEM ({cart.count} ks):</div>
				<div class="text-3xl font-black text-black">
					{cart.totalPrice} Kč
				</div>
			</div>

			<button
				onclick={handleCheckout}
				disabled={isCheckingOut || cart.hasUnavailable || cart.items.length === 0}
				class="py-3.5 px-6 bg-black text-white font-black text-sm uppercase tracking-wider hover:bg-neutral-800 active:bg-neutral-900 border-2 border-black transition-all disabled:opacity-50 disabled:pointer-events-none cursor-pointer flex items-center gap-2"
			>
				{#if isCheckingOut}
					<span>REZERVUJI...</span>
				{:else}
					<ShoppingBag class="w-5 h-5" />
					<span>K POKLADNĚ</span>
				{/if}
			</button>
		</div>
	{/if}

	<!-- Scan Floating Button -->
	<button
		onclick={openScanner}
		class="fixed bottom-6 right-6 z-30 w-16 h-16 bg-black text-white border-4 border-black hover:bg-neutral-800 active:bg-neutral-900 flex items-center justify-center transition-all cursor-pointer"
		title="Skenovat kód učebnice"
	>
		<Scan class="w-8 h-8 stroke-[2.5]" />
	</button>
</div>

<!-- CAMERA SCANNER VIEW -->
{#if isScannerOpen}
	<div class="fixed inset-0 z-50 bg-black flex flex-col">
		<!-- Top Bar -->
		<div class="absolute top-0 inset-x-0 z-20 flex items-center justify-between p-4 bg-black/80 border-b-2 border-white">
			<button
				onclick={closeScanner}
				class="flex items-center gap-1.5 py-2 px-3 bg-white text-black font-black text-xs uppercase tracking-wider border-2 border-white hover:bg-neutral-200 transition-colors cursor-pointer"
			>
				<ArrowLeft class="w-4 h-4" />
				<span>ZPĚT ({cart.count})</span>
			</button>

			<div class="text-xs font-black uppercase text-black bg-white px-3 py-1.5 border-2 border-white">
				Namiřte na Data Matrix
			</div>
		</div>

		<!-- Video & Overlay Viewport -->
		<div class="relative w-full h-full flex items-center justify-center overflow-hidden">
			<video
				bind:this={videoElement}
				playsinline
				autoplay
				muted
				class="w-full h-full object-cover"
			></video>

			<canvas bind:this={canvasElement} class="hidden"></canvas>
			<canvas bind:this={overlayCanvas} class="absolute inset-0 pointer-events-none w-full h-full"></canvas>

			<!-- FLOATING ITEM WIDGET -->
			{#if widgetPosition && currentBook}
				{@const inCart = cart.has(currentBook.id)}
				{@const isAvail = currentBook.status === 'available'}
				<div
					style="left: {widgetPosition.x}px; top: {widgetPosition.y}px;"
					class="absolute z-30 flex items-center gap-2 p-2.5 bg-white border-4 border-black text-black"
				>
					<button
						onclick={() => handleAddToCart(currentBook!)}
						disabled={inCart || !isAvail}
						class="w-11 h-11 border-2 border-black flex items-center justify-center transition-all cursor-pointer font-black {inCart
							? 'bg-neutral-200 text-black'
							: isAvail
								? 'bg-black text-white hover:bg-neutral-800'
								: 'bg-neutral-100 text-neutral-400'}"
						title={inCart ? 'V košíku' : isAvail ? 'Přidat' : 'Nedostupné'}
					>
						{#if inCart}
							<Check class="w-6 h-6" />
						{:else}
							<ShoppingBag class="w-6 h-6" />
						{/if}
					</button>

					<div class="w-12 h-16 border-2 border-black overflow-hidden bg-neutral-100 shrink-0">
						{#if currentBook.photo}
							<img
								src={getBookThumbnailUrl(currentBook)}
								alt={currentBook.code}
								class="w-full h-full object-cover"
							/>
						{:else}
							<div class="w-full h-full flex items-center justify-center text-xs font-bold">
								Foto
							</div>
						{/if}
					</div>

					<div class="pr-2 min-w-[80px]">
						<div class="text-base font-black text-black">
							{currentBook.price} Kč
						</div>
						<div class="text-[10px] font-black uppercase {isAvail ? (inCart ? 'text-black' : 'text-neutral-700') : 'text-red-600'}">
							{inCart ? 'V KOŠÍKU ✓' : isAvail ? 'K DISPOZICI' : 'NEDOSTUPNÉ'}
						</div>
					</div>
				</div>
			{/if}
		</div>
	</div>
{/if}

<!-- FULLSCREEN BUYER ID DATA MATRIX MODAL -->
{#if isCheckoutModalOpen}
	<div class="fixed inset-0 z-50 bg-black/80 flex flex-col items-center justify-center p-4">
		<div class="max-w-sm w-full bg-white border-4 border-black p-6 text-center text-black">
			<div class="inline-flex p-3 bg-neutral-100 border-2 border-black mb-3">
				<QrCode class="w-8 h-8 text-black" />
			</div>

			<h2 class="text-2xl font-black uppercase tracking-tight text-black mb-1">OBJEDNÁVKA PŘIPRAVENA</h2>
			<p class="text-xs font-bold text-neutral-600 uppercase mb-4">
				Ukažte tento kód pokladnímu. Rezervace platí 15 minut.
			</p>

			<div class="bg-white border-4 border-black p-4 inline-block mx-auto mb-4">
				<canvas bind:this={userIdCanvas} class="max-w-full h-auto"></canvas>
			</div>

			<div class="text-xs font-mono font-black text-black bg-neutral-100 py-2 px-3 border-2 border-black mb-6">
				ID: {auth.user?.id}
			</div>

			<div class="flex gap-2">
				<button
					onclick={handleCancelCheckout}
					class="flex-1 py-3 px-3 bg-white hover:bg-neutral-100 text-black font-black text-xs uppercase tracking-wider border-2 border-black transition-colors cursor-pointer"
				>
					ZRUŠIT REZERVACI
				</button>
				<button
					onclick={() => (isCheckoutModalOpen = false)}
					class="flex-1 py-3 px-3 bg-black hover:bg-neutral-800 text-white font-black text-xs uppercase tracking-wider border-2 border-black transition-colors cursor-pointer"
				>
					HOTOVO
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- FULL-RES IMAGE PREVIEW MODAL -->
{#if selectedPreviewBook}
	<div class="fixed inset-0 z-50 bg-black/90 flex flex-col items-center justify-center p-4">
		<button
			onclick={() => (selectedPreviewBook = null)}
			class="absolute top-4 right-4 p-3 bg-white text-black border-2 border-black hover:bg-neutral-200 transition-colors"
		>
			<X class="w-6 h-6" />
		</button>
		<div class="max-w-md w-full max-h-[85vh] flex flex-col items-center">
			<img
				src={getBookFullImageUrl(selectedPreviewBook)}
				alt={selectedPreviewBook.code}
				class="max-w-full max-h-[70vh] object-contain border-4 border-black bg-white mb-4"
			/>
			<div class="text-center bg-white border-2 border-black p-3 w-full">
				<p class="text-base font-black uppercase text-black">{selectedPreviewBook.code}</p>
				<p class="text-2xl font-black text-black">{selectedPreviewBook.price} Kč</p>
			</div>
		</div>
	</div>
{/if}
