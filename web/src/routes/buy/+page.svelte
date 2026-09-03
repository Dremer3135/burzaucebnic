<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, eventStore } from '$lib/stores.svelte';
	import { cart, type CartItem } from '$lib/cart.svelte';
	import { pb, getBookThumbnailUrl, getBookFullImageUrl } from '$lib/pocketbase';
	import { scanFrameForDataMatrix, drawBoundingBox, getVideoTransform, type ScanMatch, type VideoTransform } from '$lib/scanner';
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
		ExternalLink,
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
			const stream = await navigator.mediaDevices.getUserMedia({
				video: {
					facingMode: 'environment',
					width: { ideal: 1280 },
					height: { ideal: 720 }
				},
				audio: false
			});
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

		// Sync overlay canvas dimensions to video element client dimensions
		if (overlay.width !== video.clientWidth || overlay.height !== video.clientHeight) {
			overlay.width = video.clientWidth;
			overlay.height = video.clientHeight;
		}

		if (ctx) {
			ctx.clearRect(0, 0, overlay.width, overlay.height);
		}

		const now = performance.now();
		// Throttle scanning frame processing to ~15 fps to preserve mobile battery and prevent UI stutter
		if (!isScanningFrame && now - lastScanTime >= 65 && video.videoWidth > 0 && video.videoHeight > 0) {
			isScanningFrame = true;
			lastScanTime = now;
			try {
				const match = await scanFrameForDataMatrix(canvasElement, video);

				if (match && isScannerOpen) {
					// Use exact object-fit: cover transform for pinpoint accuracy on any mobile aspect ratio
					const transform = getVideoTransform(video.videoWidth, video.videoHeight, overlay.width, overlay.height);

					if (ctx) {
						drawBoundingBox(ctx, match.position, transform, '#10b981');
					}

					// Calculate widget position relative to viewport
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
					// Code not in frame: immediately remove floating widget overlay
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

		// Check if we already have an active subscription for this book
		for (const item of activeScanSubscriptions.values()) {
			if (item.book.code === code) {
				currentBook = item.book;
				// Reset 10-second expiration timer
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

			// Active real-time subscription for scanned book
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

			// User requirement: "When the user doesnt add the book to their cart, and the book disappears for more than 10 seconds from being scanned, the subscription will be terminated."
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

			// On successful checkout, clear cart and open fullscreen user ID Data Matrix modal
			cart.clear();
			isCheckoutModalOpen = true;

			// Draw Data Matrix on canvas
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
			// Restore previously checked out items back to cart
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

<div class="flex-1 max-w-4xl w-full mx-auto p-4 flex flex-col pb-24">
	<!-- Page Header -->
	<div class="flex items-center justify-between mb-4">
		<div>
			<h1 class="text-xl sm:text-2xl font-bold text-white tracking-tight">Nákupní košík</h1>
			<p class="text-xs text-slate-400">
				Naskenujte učebnice pomocí kamery a přejděte k pokladně
			</p>
		</div>

		{#if cart.count > 0}
			<button
				onclick={() => cart.clear()}
				class="text-xs text-slate-400 hover:text-rose-400 flex items-center gap-1 p-1.5 transition-colors cursor-pointer"
			>
				<Trash2 class="w-3.5 h-3.5" />
				Vysypat
			</button>
		{/if}
	</div>

	<!-- Cart Items List -->
	{#if cart.items.length === 0}
		<div class="flex-1 flex flex-col items-center justify-center p-8 bg-slate-800/40 border border-dashed border-slate-700 rounded-3xl text-center my-6">
			<div class="p-4 bg-blue-500/10 text-blue-400 rounded-full mb-3">
				<ShoppingBag class="w-8 h-8" />
			</div>
			<h3 class="text-base font-semibold text-white mb-1">Váš nákupní košík je prázdný</h3>
			<p class="text-xs text-slate-400 max-w-xs mb-5">
				Klepněte na tlačítko skeneru vpravo dole a namiřte fotoaparát na kód Data Matrix na učebnici.
			</p>
			<button
				onclick={openScanner}
				class="inline-flex items-center gap-2 py-2.5 px-4 bg-emerald-600 hover:bg-emerald-500 text-white font-medium rounded-xl text-xs transition-colors shadow-lg shadow-emerald-950 cursor-pointer"
			>
				<Scan class="w-4 h-4" />
				Skenovat učebnici
			</button>
		</div>
	{:else}
		{#if cart.hasUnavailable}
			<div class="mb-4 p-3 bg-amber-500/15 border border-amber-500/30 text-amber-300 text-xs rounded-xl flex items-center gap-2">
				<AlertTriangle class="w-4 h-4 shrink-0" />
				<span>Některé knihy v košíku byly mezitím rezervovány nebo prodány. Před dokončením je prosím odstraňte.</span>
			</div>
		{/if}

		{#if checkoutError}
			<div class="mb-4 p-3 bg-rose-500/15 border border-rose-500/30 text-rose-300 text-xs rounded-xl flex items-center gap-2">
				<AlertTriangle class="w-4 h-4 shrink-0" />
				<span>{checkoutError}</span>
			</div>
		{/if}

		<div class="space-y-2.5 flex-1">
			{#each cart.items as item (item.book.id)}
				<div
					class="rounded-2xl p-3 flex gap-3 items-center border transition-all {item.isAvailable
						? 'bg-slate-800/80 border-slate-700/80 text-white'
						: 'bg-slate-900/80 border-rose-500/40 opacity-60 text-slate-400'}"
				>
					<!-- Thumbnail (click opens full-res) -->
					<button
						type="button"
						onclick={() => (selectedPreviewBook = item.book)}
						class="w-16 h-22 shrink-0 rounded-xl overflow-hidden bg-slate-950 border border-slate-700 relative group cursor-pointer"
					>
						{#if item.book.photo}
							<img
								src={getBookThumbnailUrl(item.book)}
								alt={item.book.code}
								class="w-full h-full object-cover group-hover:scale-105 transition-transform"
							/>
						{:else}
							<div class="w-full h-full flex items-center justify-center text-slate-600">
								<Camera class="w-5 h-5" />
							</div>
						{/if}
						<div class="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center text-[10px] text-white">
							Lupa
						</div>
					</button>

					<!-- Details -->
					<div class="flex-1 min-w-0">
						<div class="flex items-center gap-2 mb-1">
							<span class="text-sm font-semibold truncate text-slate-200">{item.book.code}</span>
							{#if !item.isAvailable}
								<span class="text-[10px] bg-rose-500/20 text-rose-300 border border-rose-500/40 px-1.5 py-0.5 rounded font-medium">
									Nedostupné
								</span>
							{/if}
						</div>
						<div class="text-base font-bold text-emerald-400">
							{item.book.price} Kč
						</div>
					</div>

					<!-- Delete from cart button -->
					<button
						onclick={() => cart.removeBook(item.book.id)}
						class="p-2 rounded-xl text-slate-400 hover:text-rose-400 hover:bg-slate-700/50 transition-colors"
						title="Odebrat z košíku"
					>
						<Trash2 class="w-4 h-4" />
					</button>
				</div>
			{/each}
		</div>

		<!-- Bottom Checkout Bar -->
		<div class="sticky bottom-4 mt-6 bg-slate-800/95 border border-slate-700 rounded-2xl p-4 shadow-2xl backdrop-blur flex items-center justify-between gap-4">
			<div>
				<div class="text-xs text-slate-400">Celková cena ({cart.count} {cart.count === 1 ? 'kniha' : cart.count < 5 ? 'knihy' : 'knih'}):</div>
				<div class="text-2xl font-black text-emerald-400">
					{cart.totalPrice} Kč
				</div>
			</div>

			<button
				onclick={handleCheckout}
				disabled={isCheckingOut || cart.hasUnavailable || cart.items.length === 0}
				class="py-3 px-6 rounded-xl bg-emerald-600 hover:bg-emerald-500 active:scale-95 text-white font-bold text-sm transition-all shadow-lg shadow-emerald-950 disabled:opacity-50 disabled:pointer-events-none cursor-pointer flex items-center gap-2"
			>
				{#if isCheckingOut}
					<span class="animate-spin w-4 h-4 border-2 border-white border-t-transparent rounded-full"></span>
					<span>Rezervuji...</span>
				{:else}
					<ShoppingBag class="w-4 h-4" />
					<span>Přejít k pokladně</span>
				{/if}
			</button>
		</div>
	{/if}

	<!-- Scan Button in Lower Right Corner -->
	<button
		onclick={openScanner}
		class="fixed bottom-6 right-6 z-30 w-14 h-14 rounded-full bg-blue-600 hover:bg-blue-500 active:scale-95 text-white font-bold shadow-xl shadow-blue-950/60 flex items-center justify-center transition-all cursor-pointer"
		title="Skenovat kód učebnice"
	>
		<Scan class="w-7 h-7 stroke-[2]" />
	</button>
</div>

<!-- CAMERA SCANNER VIEW -->
{#if isScannerOpen}
	<div class="fixed inset-0 z-50 bg-black flex flex-col">
		<!-- Top Bar with Back Button -->
		<div class="absolute top-0 inset-x-0 z-20 flex items-center justify-between p-4 bg-gradient-to-b from-black/80 to-transparent">
			<button
				onclick={closeScanner}
				class="flex items-center gap-1.5 py-2 px-3 rounded-xl bg-slate-900/80 text-white hover:bg-slate-800 backdrop-blur border border-slate-700/60 text-xs font-semibold shadow cursor-pointer"
			>
				<ArrowLeft class="w-4 h-4" />
				<span>Zpět do košíku ({cart.count})</span>
			</button>

			<div class="text-xs text-white/80 bg-slate-900/80 backdrop-blur px-3 py-1.5 rounded-full border border-slate-700/60">
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

			<!-- FLOATING ITEM WIDGET NEXT TO BOUNDING BOX -->
			{#if widgetPosition && currentBook}
				{@const inCart = cart.has(currentBook.id)}
				{@const isAvail = currentBook.status === 'available'}
				<div
					style="left: {widgetPosition.x}px; top: {widgetPosition.y}px;"
					class="absolute z-30 flex items-center gap-2 p-2 bg-slate-900/95 border border-emerald-500/80 rounded-2xl shadow-2xl backdrop-blur animate-in fade-in zoom-in-95 duration-150"
				>
					<!-- Add to Cart Icon Button on the left -->
					<button
						onclick={() => handleAddToCart(currentBook!)}
						disabled={inCart || !isAvail}
						class="w-10 h-10 rounded-xl flex items-center justify-center transition-all cursor-pointer {inCart
							? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/40'
							: isAvail
								? 'bg-emerald-600 hover:bg-emerald-500 active:scale-90 text-white shadow-md'
								: 'bg-slate-800 text-slate-500 border border-slate-700'}"
						title={inCart ? 'V košíku' : isAvail ? 'Přidat do košíku' : 'Nedostupné'}
					>
						{#if inCart}
							<Check class="w-5 h-5" />
						{:else}
							<ShoppingBag class="w-5 h-5" />
						{/if}
					</button>

					<!-- ~100px Thumbnail -->
					<div class="w-12 h-16 rounded-lg overflow-hidden bg-slate-950 shrink-0 border border-slate-800">
						{#if currentBook.photo}
							<img
								src={getBookThumbnailUrl(currentBook)}
								alt={currentBook.code}
								class="w-full h-full object-cover"
							/>
						{:else}
							<div class="w-full h-full flex items-center justify-center text-slate-600 text-xs">
								Foto
							</div>
						{/if}
					</div>

					<!-- Price & Status -->
					<div class="pr-2 min-w-[70px]">
						<div class="text-sm font-bold text-emerald-400">
							{currentBook.price} Kč
						</div>
						<div class="text-[10px] font-medium {isAvail ? (inCart ? 'text-emerald-400' : 'text-slate-300') : 'text-rose-400'}">
							{inCart ? 'V košíku ✓' : isAvail ? 'K dispozici' : 'Nedostupné'}
						</div>
					</div>
				</div>
			{/if}
		</div>
	</div>
{/if}

<!-- FULLSCREEN BUYER ID DATA MATRIX MODAL (POST-CHECKOUT) -->
{#if isCheckoutModalOpen}
	<div class="fixed inset-0 z-50 bg-slate-950/95 flex flex-col items-center justify-center p-4 backdrop-blur-md">
		<div class="max-w-sm w-full bg-slate-900 border-2 border-emerald-500 rounded-3xl p-6 text-center shadow-2xl">
			<div class="inline-flex p-3 bg-emerald-500/10 text-emerald-400 rounded-2xl mb-3 border border-emerald-500/20">
				<QrCode class="w-8 h-8" />
			</div>

			<h2 class="text-xl font-bold text-white mb-1">Objednávka připravena</h2>
			<p class="text-xs text-slate-300 mb-5">
				Ukažte tento kód pokladnímu. Knihy jsou pro vás rezervovány na 15 minut.
			</p>

			<!-- Fullscreen / Large Data Matrix Container -->
			<div class="bg-white p-4 rounded-2xl inline-block shadow-xl mx-auto mb-4">
				<canvas bind:this={userIdCanvas} class="max-w-full h-auto"></canvas>
			</div>

			<div class="text-[11px] font-mono text-slate-400 bg-slate-950 py-1.5 px-3 rounded-lg border border-slate-800 mb-6">
				ID kupujícího: {auth.user?.id}
			</div>

			<div class="flex gap-2">
				<button
					onclick={handleCancelCheckout}
					class="flex-1 py-2.5 px-3 rounded-xl bg-slate-800 hover:bg-slate-700 text-rose-300 text-xs font-medium border border-slate-700 transition-colors cursor-pointer"
				>
					Zrušit rezervaci
				</button>
				<button
					onclick={() => (isCheckoutModalOpen = false)}
					class="flex-1 py-2.5 px-3 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-bold transition-colors cursor-pointer shadow-md"
				>
					Rozumím
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- FULL-RES IMAGE PREVIEW MODAL -->
{#if selectedPreviewBook}
	<div class="fixed inset-0 z-50 bg-black/90 flex flex-col items-center justify-center p-4 backdrop-blur-sm">
		<button
			onclick={() => (selectedPreviewBook = null)}
			class="absolute top-4 right-4 p-2.5 rounded-full bg-slate-800/80 text-white hover:bg-slate-700 transition-colors"
		>
			<X class="w-6 h-6" />
		</button>
		<div class="max-w-md w-full max-h-[85vh] flex flex-col items-center">
			<img
				src={getBookFullImageUrl(selectedPreviewBook)}
				alt={selectedPreviewBook.code}
				class="max-w-full max-h-[70vh] rounded-2xl object-contain border border-slate-700 shadow-2xl mb-4"
			/>
			<div class="text-center">
				<p class="text-base font-bold text-white">{selectedPreviewBook.code}</p>
				<p class="text-emerald-400 font-bold text-lg">{selectedPreviewBook.price} Kč</p>
			</div>
		</div>
	</div>
{/if}
