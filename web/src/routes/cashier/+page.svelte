<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, eventStore } from '$lib/stores.svelte';
	import { pb, getBookThumbnailUrl, getBookFullImageUrl } from '$lib/pocketbase';
	import {
		CAMERA_CONSTRAINTS,
		getVideoTransform,
		scanFrameForAllDataMatrices,
		drawPricePolygon,
		idToColor,
		type ScanMatch
	} from '$lib/scanner';
	import { renderSpaydQRCode } from '$lib/barcodes';
	import {
		Scan,
		Receipt,
		Banknote,
		CheckCircle2,
		AlertCircle,
		RefreshCw,
		User,
		UserMinus,
		Trash2,
		X,
		ChevronUp,
		ChevronDown,
		ZoomIn,
		Check,
		Camera,
		Plus
	} from '@lucide/svelte';
	import type { Book, Payment, Event as AppEvent } from '$lib/types';

	interface BuyerInfo {
		id: string;
		name: string;
		email: string;
	}

	// ----------------------------------------------------
	// 1. SCANNER & CAMERA STATE
	// ----------------------------------------------------
	let videoElement = $state<HTMLVideoElement | null>(null);
	let captureCanvas = $state<HTMLCanvasElement | null>(null);
	let overlayCanvas = $state<HTMLCanvasElement | null>(null);

	let mediaStream = $state<MediaStream | null>(null);
	let isScanningLoopActive = false;
	let renderAnimId = $state<number | null>(null);

	let isCameraReady = $state(false);
	let cameraError = $state<string | null>(null);

	// Tracked codes to prevent rendering flicker (code -> { match, lastSeen })
	interface TrackedMatch {
		match: ScanMatch;
		lastSeen: number;
	}
	const trackedMatches = new Map<string, TrackedMatch>();

	// Cache for code lookups (code -> { type: 'book' | 'user' | 'unknown', ... })
	const codeLookupCache = new Map<string, any>();
	const inFlightLookups = new Set<string>();

	// Deleted books suppression: bookId -> lastSeenInFrame timestamp
	const suppressedBooks = new Map<string, number>();

	// ----------------------------------------------------
	// 2. CART & BUYER STATE
	// ----------------------------------------------------
	let cartBooks = $state<Book[]>([]);
	let currentBuyer = $state<BuyerInfo | null>(null);
	let totalAmount = $derived(cartBooks.reduce((sum, b) => sum + b.price, 0));

	// Bottom Sheet toggle
	let sheetExpanded = $state(false);

	// Swipe gesture tracking for cart books (bookId -> deltaX)
	let swipingBookId = $state<string | null>(null);
	let swipeStartX = 0;
	let swipeCurrentDeltaX = $state(0);

	// Swipe gesture tracking for buyer card
	let swipingBuyer = $state(false);
	let buyerStartX = 0;
	let buyerCurrentDeltaX = $state(0);

	// ----------------------------------------------------
	// 3. IMAGE PREVIEW MODAL
	// ----------------------------------------------------
	let previewBook = $state<Book | null>(null);

	// ----------------------------------------------------
	// 4. CHECKOUT DIALOG STATE
	// ----------------------------------------------------
	let isCheckoutModalOpen = $state(false);
	let checkoutEmail = $state('');
	let checkoutName = $state('');
	let emailSearchResults = $state<Array<{ id: string; email: string; name: string }>>([]);
	let isSearchingUsers = $state(false);
	let isSubmittingCheckout = $state(false);
	let checkoutError = $state('');
	let searchTimeout: any = null;

	// ----------------------------------------------------
	// 5. PAYMENT & FINALIZATION STATE
	// ----------------------------------------------------
	let paymentMode = $state<'PAYMENT' | 'SUCCESS' | null>(null);
	let activePayment = $state<Payment | null>(null);
	let activeEvent = $state<AppEvent | null>(null);
	let qrCanvas = $state<HTMLCanvasElement | null>(null);
	let isConfirmingPayment = $state(false);
	let paymentError = $state('');
	let manualCodeInput = $state('');
	let isManualInputOpen = $state(false);

	// ----------------------------------------------------
	// LIFECYCLE & PERMISSIONS
	// ----------------------------------------------------
	$effect(() => {
		if (auth.user && !auth.isCashier) {
			goto('/');
		}
	});

	onMount(() => {
		startCamera();
	});

	onDestroy(() => {
		stopCamera();
	});

	// Re-render SPAYD QR code whenever entering payment mode
	$effect(() => {
		if (paymentMode === 'PAYMENT' && qrCanvas && activePayment) {
			const ev = activeEvent || eventStore.event;
			const iban = ev?.iban || 'CZ6520100000002101234567';
			renderSpaydQRCode(qrCanvas, {
				iban: iban,
				amount: activePayment.totalAmount,
				vs: activePayment.variableSymbol,
				paymentId: activePayment.id
			}).catch(console.error);
		}
	});

	// ----------------------------------------------------
	// CAMERA & SCANNING
	// ----------------------------------------------------
	async function startCamera() {
		stopCamera();
		cameraError = null;
		isCameraReady = false;

		try {
			const stream = await navigator.mediaDevices.getUserMedia(CAMERA_CONSTRAINTS);
			mediaStream = stream;

			if (videoElement) {
				videoElement.srcObject = stream;
				await videoElement.play();
				isCameraReady = true;

				isScanningLoopActive = true;
				runDetectionLoop();
				runRenderLoop();
			}
		} catch (err: any) {
			console.error('Camera initialization error', err);
			cameraError = 'Nelze přistoupit ke kameře pro pokladnu. Zkontrolujte prosím oprávnění.';
		}
	}

	function stopCamera() {
		isScanningLoopActive = false;
		if (renderAnimId !== null) {
			cancelAnimationFrame(renderAnimId);
			renderAnimId = null;
		}
		if (mediaStream) {
			mediaStream.getTracks().forEach((track) => track.stop());
			mediaStream = null;
		}
		trackedMatches.clear();
	}

	let isDetecting = false;
	let lastDetectTime = 0;

	async function runDetectionLoop() {
		if (!isScanningLoopActive) return;

		const now = performance.now();
		if (!isDetecting && videoElement && captureCanvas && videoElement.videoWidth > 0 && now - lastDetectTime >= 60) {
			isDetecting = true;
			lastDetectTime = now;

			try {
				const detected = await scanFrameForAllDataMatrices(captureCanvas, videoElement, 8);
				const seenTime = performance.now();
				const seenCodes = new Set<string>();

				for (const match of detected) {
					const code = match.text.trim();
					if (!code) continue;
					seenCodes.add(code);
					trackedMatches.set(code, { match, lastSeen: seenTime });

					// If code was deleted, it stays suppressed as long as it stays in frame
					if (suppressedBooks.has(code)) {
						suppressedBooks.set(code, Date.now());
						continue;
					}

					// Process detected code (auto-add book or assign buyer)
					handleCodeDetected(code);
				}

				// Re-arm suppression: If a suppressed book is NOT in frame for >= 2 seconds, remove from suppression
				const currentTime = Date.now();
				for (const [bookId, lastSeen] of suppressedBooks.entries()) {
					if (!seenCodes.has(bookId)) {
						if (currentTime - lastSeen >= 2000) {
							suppressedBooks.delete(bookId);
						}
					}
				}

				// Evict tracked matches not seen in last 220ms
				for (const [code, item] of trackedMatches.entries()) {
					if (seenTime - item.lastSeen > 220) {
						trackedMatches.delete(code);
					}
				}
			} catch (err) {
				console.error('Detection frame error', err);
			} finally {
				isDetecting = false;
			}
		}

		if (isScanningLoopActive) {
			setTimeout(runDetectionLoop, 30);
		}
	}

	function runRenderLoop() {
		if (!isScanningLoopActive) return;

		if (overlayCanvas && videoElement && videoElement.videoWidth > 0) {
			const dpr = window.devicePixelRatio || 1;
			const cRect = overlayCanvas.getBoundingClientRect();
			const targetW = Math.round(cRect.width * dpr);
			const targetH = Math.round(cRect.height * dpr);

			if (overlayCanvas.width !== targetW || overlayCanvas.height !== targetH) {
				overlayCanvas.width = targetW;
				overlayCanvas.height = targetH;
			}

			const ctx = overlayCanvas.getContext('2d');
			if (ctx) {
				ctx.save();
				ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
				ctx.clearRect(0, 0, cRect.width, cRect.height);

				const vRect = videoElement.getBoundingClientRect();
				const transform = getVideoTransform(
					videoElement.videoWidth,
					videoElement.videoHeight,
					vRect.width,
					vRect.height,
					vRect.left - cRect.left,
					vRect.top - cRect.top
				);

				// Draw polygons over all tracked matches
				for (const { match } of trackedMatches.values()) {
					const code = match.text.trim();

					// 1. Is it a book already in the cart?
					const bookInCart = cartBooks.find((b) => b.id === code);
					if (bookInCart) {
						const color = idToColor(bookInCart.id);
						drawPricePolygon(ctx, match.position, `${bookInCart.price} Kč`, transform, color);
						continue;
					}

					// 2. Is it the current buyer?
					if (currentBuyer && currentBuyer.id === code) {
						drawPricePolygon(ctx, match.position, `✓ ${currentBuyer.name || 'ZÁKAZNÍK'}`, transform, {
							bg: '#059669',
							border: '#047857',
							text: '#ffffff',
							lightBg: '#d1fae5'
						});
						continue;
					}

					// 3. What if it's an available book cached or being added?
					const cached = codeLookupCache.get(code);
					if (cached?.type === 'book' && cached.book.status === 'available' && !suppressedBooks.has(code)) {
						const color = idToColor(cached.book.id);
						drawPricePolygon(ctx, match.position, `${cached.book.price} Kč`, transform, color);
					}
				}

				ctx.restore();
			}
		}

		renderAnimId = requestAnimationFrame(runRenderLoop);
	}

	async function handleCodeDetected(code: string) {
		// Check if already in cart
		if (cartBooks.some((b) => b.id === code)) return;
		// Check if already assigned as buyer
		if (currentBuyer && currentBuyer.id === code) return;

		let info = codeLookupCache.get(code);
		if (!info) {
			if (inFlightLookups.has(code)) return;
			inFlightLookups.add(code);

			try {
				info = await pb.send<any>(`/api/cashier/lookup-code?code=${encodeURIComponent(code)}`, {
					method: 'GET'
				});
				codeLookupCache.set(code, info);
			} catch (err) {
				console.error('Lookup code error', err);
				return;
			} finally {
				inFlightLookups.delete(code);
			}
		}

		if (info?.type === 'user') {
			if (!currentBuyer || currentBuyer.id !== info.user.id) {
				currentBuyer = info.user;
				if (navigator.vibrate) navigator.vibrate([80, 40, 80]);
			}
		} else if (info?.type === 'book') {
			const book: Book = info.book;
			if (book.status === 'available') {
				if (!suppressedBooks.has(book.id) && !cartBooks.some((b) => b.id === book.id)) {
					// AUTO-ADD TO CART!
					cartBooks = [...cartBooks, book];
					if (navigator.vibrate) navigator.vibrate([50]);
				}
			}
		}
	}

	// ----------------------------------------------------
	// CART INTERACTIONS (SWIPE-TO-DELETE & UNLINK)
	// ----------------------------------------------------
	function removeBookFromCart(bookId: string) {
		cartBooks = cartBooks.filter((b) => b.id !== bookId);
		// Suppress immediately: stays suppressed as long as in camera view,
		// and re-arms only if it leaves the camera view for >= 2 seconds!
		suppressedBooks.set(bookId, Date.now());
		if (swipingBookId === bookId) {
			swipingBookId = null;
			swipeCurrentDeltaX = 0;
		}
	}

	function unlinkBuyer() {
		currentBuyer = null;
		swipingBuyer = false;
		buyerCurrentDeltaX = 0;
	}

	// Touch/Pointer handlers for book row swipe-right
	function handleBookPointerDown(e: PointerEvent, bookId: string) {
		swipingBookId = bookId;
		swipeStartX = e.clientX;
		swipeCurrentDeltaX = 0;
	}

	function handleBookPointerMove(e: PointerEvent, bookId: string) {
		if (swipingBookId !== bookId) return;
		const delta = e.clientX - swipeStartX;
		swipeCurrentDeltaX = Math.max(0, Math.min(180, delta));
	}

	function handleBookPointerUp(e: PointerEvent, bookId: string) {
		if (swipingBookId !== bookId) return;
		if (swipeCurrentDeltaX >= 75) {
			removeBookFromCart(bookId);
		}
		swipingBookId = null;
		swipeCurrentDeltaX = 0;
	}

	// Touch/Pointer handlers for buyer card swipe-right
	function handleBuyerPointerDown(e: PointerEvent) {
		swipingBuyer = true;
		buyerStartX = e.clientX;
		buyerCurrentDeltaX = 0;
	}

	function handleBuyerPointerMove(e: PointerEvent) {
		if (!swipingBuyer) return;
		const delta = e.clientX - buyerStartX;
		buyerCurrentDeltaX = Math.max(0, Math.min(180, delta));
	}

	function handleBuyerPointerUp(e: PointerEvent) {
		if (!swipingBuyer) return;
		if (buyerCurrentDeltaX >= 75) {
			unlinkBuyer();
		}
		swipingBuyer = false;
		buyerCurrentDeltaX = 0;
	}

	// Manual code entry handler
	async function handleManualCodeSubmit(e: SubmitEvent) {
		e.preventDefault();
		const code = manualCodeInput.trim();
		if (!code) return;
		manualCodeInput = '';
		isManualInputOpen = false;
		await handleCodeDetected(code);
	}

	// ----------------------------------------------------
	// CHECKOUT WORKFLOW
	// ----------------------------------------------------
	function openCheckoutModal() {
		if (cartBooks.length === 0) return;
		checkoutEmail = currentBuyer ? currentBuyer.email : '';
		checkoutName = currentBuyer ? currentBuyer.name || '' : '';
		emailSearchResults = [];
		checkoutError = '';
		isCheckoutModalOpen = true;
	}

	function handleEmailInput() {
		clearTimeout(searchTimeout);

		const query = checkoutEmail.trim();
		if (query.length >= 2) {
			searchTimeout = setTimeout(async () => {
				isSearchingUsers = true;
				try {
					const results = await pb.send<Array<{ id: string; email: string; name: string }>>(
						`/api/cashier/user-search?query=${encodeURIComponent(query)}`,
						{ method: 'GET' }
					);
					emailSearchResults = results;
				} catch (err) {
					console.error('Email search error', err);
				} finally {
					isSearchingUsers = false;
				}
			}, 200);
		} else {
			emailSearchResults = [];
		}
	}

	function selectUserSuggestion(user: { id: string; email: string; name: string }) {
		checkoutEmail = user.email;
		checkoutName = user.name || '';
		currentBuyer = user;
		emailSearchResults = [];
	}

	async function handleConfirmCheckout() {
		const email = checkoutEmail.trim();
		if (!email && !currentBuyer?.id) {
			checkoutError = 'Vyplňte prosím email zákazníka.';
			return;
		}

		isSubmittingCheckout = true;
		checkoutError = '';

		try {
			const res = await pb.send<{
				success: boolean;
				payment: Payment;
				buyer: BuyerInfo;
				event: AppEvent;
			}>('/api/cashier/checkout', {
				method: 'POST',
				body: {
					email: email,
					name: checkoutName.trim(),
					buyerId: currentBuyer?.id || '',
					bookIds: cartBooks.map((b) => b.id)
				}
			});

			activePayment = res.payment;
			activeEvent = res.event || eventStore.event;
			currentBuyer = res.buyer;

			isCheckoutModalOpen = false;
			sheetExpanded = false;
			paymentMode = 'PAYMENT';
			paymentError = '';
		} catch (err: any) {
			console.error('Checkout failed', err);
			checkoutError = err?.message || 'Chyba při vytváření platby. Zkontrolujte dostupnost knih.';
		} finally {
			isSubmittingCheckout = false;
		}
	}

	// ----------------------------------------------------
	// PAYMENT CONFIRMATION
	// ----------------------------------------------------
	async function handleConfirmCash() {
		if (!activePayment) return;
		isConfirmingPayment = true;
		paymentError = '';

		try {
			await pb.send('/api/cashier/confirm-payment', {
				method: 'POST',
				body: {
					paymentId: activePayment.id,
					method: 'cash'
				}
			});

			paymentMode = 'SUCCESS';
		} catch (err: any) {
			console.error('Failed to confirm cash payment', err);
			paymentError = err?.message || 'Chyba při potvrzení hotovostní platby.';
		} finally {
			isConfirmingPayment = false;
		}
	}

	function resetForNextCustomer() {
		cartBooks = [];
		currentBuyer = null;
		suppressedBooks.clear();
		codeLookupCache.clear();
		activePayment = null;
		paymentMode = null;
		paymentError = '';
		sheetExpanded = false;
	}
</script>

<div class="relative flex-1 w-full h-full bg-black overflow-hidden flex flex-col select-none touch-none">
	<!-- TOP NAVIGATION BAR -->
	<div class="absolute top-3 inset-x-3 z-20 flex items-center justify-between pointer-events-auto max-w-2xl mx-auto w-full">
		<div class="flex items-center gap-2 bg-white/95 border-2 border-black p-1 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] w-full">
			<a
				href="/cashier"
				class="flex-1 py-1.5 px-3 text-center text-xs font-black uppercase tracking-wider bg-black text-white flex items-center justify-center gap-1.5 transition-colors"
			>
				<Scan class="w-3.5 h-3.5" />
				POKLADNA
			</a>
			<a
				href="/cashier/payments"
				class="flex-1 py-1.5 px-3 text-center text-xs font-black uppercase tracking-wider text-black hover:bg-neutral-100 flex items-center justify-center gap-1.5 transition-colors"
			>
				<Receipt class="w-3.5 h-3.5" />
				BANKOVNÍ PLATBY
			</a>
		</div>
	</div>

	<!-- CAMERA & SCANNER VIEWPORT -->
	<div class="relative flex-1 w-full h-full bg-black overflow-hidden flex items-center justify-center">
		<video
			bind:this={videoElement}
			playsinline
			autoplay
			muted
			class="w-full h-full object-cover"
		></video>

		<!-- Off-screen processing canvas -->
		<canvas bind:this={captureCanvas} class="hidden"></canvas>

		<!-- Real-time High-DPI AR Overlay Canvas -->
		<canvas
			bind:this={overlayCanvas}
			class="absolute inset-0 pointer-events-none w-full h-full z-10"
		></canvas>

		<!-- Camera Guide & Instructions -->
		{#if isCameraReady && !cameraError && paymentMode === null}
			<div class="absolute top-16 inset-x-0 flex justify-center pointer-events-none z-10 px-4">
				<div class="bg-white/95 text-black border-2 border-black px-3 py-1.5 text-[11px] font-black uppercase tracking-wider flex items-center gap-2 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)]">
					<Camera class="w-3.5 h-3.5 text-black shrink-0" />
					<span>Namiřte kameru na knihy nebo kód kupujícího</span>
				</div>
			</div>
		{/if}

		<!-- Manual Input Toggle (Corner Button) -->
		{#if paymentMode === null}
			<button
				onclick={() => (isManualInputOpen = !isManualInputOpen)}
				class="absolute top-16 right-3 z-20 bg-white text-black border-2 border-black p-1.5 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] hover:bg-neutral-100 active:scale-95 transition-transform cursor-pointer"
				title="Zadat kód ručně"
			>
				<Plus class="w-4 h-4" />
			</button>
		{/if}

		<!-- Manual Code Drawer / Popover -->
		{#if isManualInputOpen && paymentMode === null}
			<div class="absolute top-28 right-3 z-30 bg-white border-4 border-black p-3 max-w-xs w-full shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]">
				<div class="flex items-center justify-between pb-2 border-b-2 border-black mb-2">
					<span class="text-xs font-black uppercase">RUČNÍ ZADÁNÍ KÓDU</span>
					<button onclick={() => (isManualInputOpen = false)} class="p-0.5 hover:bg-neutral-100 cursor-pointer">
						<X class="w-4 h-4" />
					</button>
				</div>
				<form onsubmit={handleManualCodeSubmit} class="flex gap-1.5">
					<input
						type="text"
						bind:value={manualCodeInput}
						placeholder="Kód knihy / kupujícího"
						class="flex-1 bg-white border-2 border-black px-2 py-1.5 text-xs font-black uppercase"
					/>
					<button
						type="submit"
						class="px-3 py-1.5 bg-black text-white text-xs font-black uppercase border-2 border-black hover:bg-neutral-800 cursor-pointer"
					>
						PŘIDAT
					</button>
				</form>
			</div>
		{/if}

		<!-- Camera Error -->
		{#if cameraError}
			<div class="absolute inset-0 bg-white flex flex-col items-center justify-center p-6 text-center z-40">
				<div class="p-3 bg-red-100 text-red-700 border-2 border-red-700 mb-4">
					<AlertCircle class="w-8 h-8" />
				</div>
				<h2 class="text-xl font-black uppercase tracking-tight text-black mb-2">Kamera není dostupná</h2>
				<p class="text-sm font-semibold text-neutral-600 max-w-sm mb-6 uppercase">
					{cameraError}
				</p>
				<button
					onclick={startCamera}
					class="inline-flex items-center gap-2 py-3 px-6 bg-black text-white font-black text-xs uppercase tracking-wider hover:bg-neutral-800 border-2 border-black cursor-pointer"
				>
					<RefreshCw class="w-4 h-4" />
					ZKUSIT ZNOVU
				</button>
			</div>
		{/if}
	</div>

	<!-- ---------------------------------------------------------------- -->
	<!-- SLEEK BOTTOM SHEET CART                                          -->
	<!-- ---------------------------------------------------------------- -->
	{#if paymentMode === null}
		<div
			class="fixed bottom-0 inset-x-0 z-30 max-w-2xl mx-auto w-full bg-white border-t-4 border-x-4 border-black text-black transition-all duration-300 flex flex-col shadow-[0_-8px_24px_rgba(0,0,0,0.2)]"
			style={sheetExpanded ? 'max-height: 82vh;' : 'max-height: 105px;'}
		>
			<!-- DRAG / TOGGLE HEADER -->
			<div
				role="button"
				tabindex="0"
				onclick={() => (sheetExpanded = !sheetExpanded)}
				onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') sheetExpanded = !sheetExpanded; }}
				class="w-full flex flex-col items-center pt-2 pb-2.5 px-4 cursor-pointer select-none bg-neutral-50 hover:bg-neutral-100 border-b-2 border-black shrink-0"
			>
				<!-- Subtle drag handle bar -->
				<div class="w-12 h-1.5 bg-neutral-300 rounded-full mb-2"></div>

				<div class="w-full flex items-center justify-between gap-2">
					<!-- Cart summary & Color dots -->
					<div class="flex items-center gap-3 min-w-0">
						<div class="flex items-center gap-2">
							<span class="text-xs font-black uppercase tracking-tight bg-black text-white px-2 py-0.5 border border-black">
								{cartBooks.length} {cartBooks.length === 1 ? 'KNIHA' : cartBooks.length >= 2 && cartBooks.length <= 4 ? 'KNIHY' : 'KNIH'}
							</span>
							<span class="text-lg font-black text-black truncate">
								{totalAmount} Kč
							</span>
						</div>

						<!-- Mini color indicator badges for items in cart -->
						<div class="hidden sm:flex items-center gap-1 overflow-hidden">
							{#each cartBooks.slice(0, 6) as b (b.id)}
								{@const col = idToColor(b.id)}
								<div
									class="w-3.5 h-3.5 border border-black shrink-0"
									style="background-color: {col.bg};"
									title={b.id}
								></div>
							{/each}
						</div>
					</div>

					<!-- Buyer Pill or Zaplatit Action -->
					<div class="flex items-center gap-2 shrink-0">
						{#if currentBuyer}
							<div
								class="flex items-center gap-1.5 bg-emerald-100 border-2 border-emerald-800 px-2.5 py-1 text-emerald-900 text-xs font-black truncate max-w-[140px]"
								title="{currentBuyer.name} ({currentBuyer.email})"
							>
								<User class="w-3.5 h-3.5 shrink-0 text-emerald-800" />
								<span class="truncate">{currentBuyer.name || currentBuyer.email.split('@')[0]}</span>
							</div>
						{/if}

						<button
							type="button"
							onclick={(e) => {
								e.stopPropagation();
								openCheckoutModal();
							}}
							disabled={cartBooks.length === 0}
							class="py-2 px-4 font-black text-xs uppercase tracking-wider border-2 border-black transition-all flex items-center gap-1.5 {cartBooks.length > 0 ? 'bg-black text-white hover:bg-neutral-800 active:scale-95 cursor-pointer' : 'bg-neutral-200 text-neutral-400 border-neutral-300 cursor-not-allowed'}"
						>
							<span>ZAPLATIT</span>
						</button>

						<button
							type="button"
							class="p-1 hover:bg-neutral-200 text-black cursor-pointer border border-transparent"
							aria-label={sheetExpanded ? 'Zabalit košík' : 'Rozbalit košík'}
						>
							{#if sheetExpanded}
								<ChevronDown class="w-5 h-5" />
							{:else}
								<ChevronUp class="w-5 h-5" />
							{/if}
						</button>
					</div>
				</div>
			</div>

			<!-- EXPANDED CONTENT (Scrollable) -->
			{#if sheetExpanded}
				<div class="flex-1 overflow-y-auto p-4 space-y-4">
					<!-- BUYER CARD SECTION -->
					<div>
						<div class="text-[10px] font-black uppercase tracking-wider text-neutral-500 mb-1.5">
							KUPUJÍCÍ
						</div>
						{#if currentBuyer}
							<!-- Swipeable Buyer Card -->
							<div class="relative overflow-hidden border-2 border-black bg-emerald-50 select-none">
								<!-- Background track revealed on swipe-right -->
								<div class="absolute inset-0 bg-red-600 flex items-center px-4 gap-2 text-white font-black text-xs uppercase">
									<UserMinus class="w-4 h-4" />
									<span>ODPOJIT KUPUJÍCÍHO</span>
								</div>

								<!-- Foreground card that slides -->
								<div
									role="presentation"
									class="relative bg-emerald-50 p-3 flex items-center justify-between gap-3 transition-transform duration-75 cursor-grab active:cursor-grabbing"
									style="transform: translateX({swipingBuyer ? buyerCurrentDeltaX : 0}px);"
									onpointerdown={handleBuyerPointerDown}
									onpointermove={handleBuyerPointerMove}
									onpointerup={handleBuyerPointerUp}
									onpointercancel={handleBuyerPointerUp}
								>
									<div class="flex items-center gap-2.5 min-w-0">
										<div class="w-8 h-8 bg-emerald-600 text-white border-2 border-black flex items-center justify-center shrink-0">
											<User class="w-4 h-4" />
										</div>
										<div class="min-w-0">
											<div class="text-xs font-black uppercase text-black truncate">
												{currentBuyer.name || 'Zákazník'}
											</div>
											<div class="text-[11px] font-mono font-bold text-emerald-800 truncate">
												{currentBuyer.email}
											</div>
										</div>
									</div>

									<div class="flex items-center gap-2 shrink-0">
										<span class="hidden sm:inline text-[9px] font-bold text-neutral-500 uppercase">
											POTAŽENÍM DOPRAVA ODPOJIT
										</span>
										<button
											type="button"
											onclick={unlinkBuyer}
											class="p-1 border border-black bg-white hover:bg-neutral-100 text-black cursor-pointer"
											title="Odpojit kupujícího"
										>
											<X class="w-3.5 h-3.5" />
										</button>
									</div>
								</div>
							</div>
						{:else}
							<!-- No buyer hint -->
							<div class="p-2.5 border-2 border-dashed border-neutral-300 bg-neutral-50 text-neutral-600 text-xs flex items-center justify-between">
								<span class="text-[11px] font-bold uppercase">
									Kupující nepřiřazen (lze naskenovat kód z mobilu nebo zadat při platbě)
								</span>
							</div>
						{/if}
					</div>

					<!-- BOOKS LIST SECTION -->
					<div>
						<div class="flex items-center justify-between text-[10px] font-black uppercase tracking-wider text-neutral-500 mb-1.5">
							<span>POLOŽKY V KOŠÍKU ({cartBooks.length})</span>
							<span>POTAŽENÍM DOPRAVA ODSTRANIT</span>
						</div>

						{#if cartBooks.length === 0}
							<div class="p-8 border-2 border-dashed border-neutral-300 text-center text-neutral-500 text-xs font-bold uppercase">
								Košík je prázdný.<br />
								Namiřte kameru na knihy pro automatické přidání.
							</div>
						{:else}
							<div class="space-y-2">
								{#each cartBooks as book (book.id)}
									{@const col = idToColor(book.id)}
									<!-- Swipeable Book Card -->
									<div class="relative overflow-hidden border-2 border-black bg-white select-none">
										<!-- Red track revealed on swipe right -->
										<div class="absolute inset-0 bg-red-600 flex items-center px-4 gap-2 text-white font-black text-xs uppercase">
											<Trash2 class="w-4 h-4" />
											<span>ODSTRANIT Z KOŠÍKU</span>
										</div>

										<!-- Foreground row -->
										<div
											role="presentation"
											class="relative bg-white p-2.5 flex items-center justify-between gap-3 transition-transform duration-75 cursor-grab active:cursor-grabbing"
											style="transform: translateX({swipingBookId === book.id ? swipeCurrentDeltaX : 0}px);"
											onpointerdown={(e) => handleBookPointerDown(e, book.id)}
											onpointermove={(e) => handleBookPointerMove(e, book.id)}
											onpointerup={(e) => handleBookPointerUp(e, book.id)}
											onpointercancel={(e) => handleBookPointerUp(e, book.id)}
										>
											<div class="flex items-center gap-3 min-w-0">
												<!-- Hash-derived Color Stripe Badge -->
												<div
													class="w-3.5 h-12 border border-black shrink-0"
													style="background-color: {col.bg};"
												></div>

												<!-- Cover Thumbnail (Tap to enlarge!) -->
												<button
													type="button"
													onclick={(e) => {
														e.stopPropagation();
														previewBook = book;
													}}
													class="relative w-10 h-12 border border-black bg-neutral-100 overflow-hidden shrink-0 group cursor-pointer"
													title="Kliknutím zvětšit náhled obálky"
												>
													{#if book.photo}
														<img
															src={getBookThumbnailUrl(book)}
															alt={book.id}
															class="w-full h-full object-cover"
														/>
														<div class="absolute inset-0 bg-black/30 opacity-0 group-hover:opacity-100 flex items-center justify-center transition-opacity">
															<ZoomIn class="w-3.5 h-3.5 text-white" />
														</div>
													{/if}
												</button>

												<!-- Book Details -->
												<div class="min-w-0">
													<div class="flex items-center gap-1.5">
														<span
															class="px-1.5 py-0.2 border text-[10px] font-black uppercase text-white"
															style="background-color: {col.bg}; border-color: {col.border};"
														>
															{book.id}
														</span>
													</div>
													<div class="text-[11px] font-bold text-neutral-500 uppercase mt-0.5">
														STAV: {book.status}
													</div>
												</div>
											</div>

											<div class="flex items-center gap-3 shrink-0">
												<div class="text-base font-black text-black">
													{book.price} Kč
												</div>

												<!-- Fallback Delete Button -->
												<button
													type="button"
													onclick={() => removeBookFromCart(book.id)}
													class="p-1.5 border border-black bg-neutral-100 hover:bg-red-100 hover:text-red-700 transition-colors cursor-pointer"
													title="Smazat knihu"
												>
													<Trash2 class="w-3.5 h-3.5" />
												</button>
											</div>
										</div>
									</div>
								{/each}
							</div>
						{/if}
					</div>

					<!-- Expanded Bottom Action -->
					<div class="pt-3 border-t-2 border-black flex items-center justify-between gap-3">
						<div>
							<div class="text-[10px] font-bold uppercase text-neutral-500">CELKEM:</div>
							<div class="text-2xl font-black text-black">{totalAmount} Kč</div>
						</div>

						<button
							type="button"
							onclick={openCheckoutModal}
							disabled={cartBooks.length === 0}
							class="py-3 px-6 bg-black text-white hover:bg-neutral-800 font-black text-xs uppercase tracking-wider border-2 border-black flex items-center gap-2 cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
						>
							<Banknote class="w-4 h-4" />
							<span>PŘEJÍT K PLATBĚ ({totalAmount} Kč)</span>
						</button>
					</div>
				</div>
			{/if}
		</div>
	{/if}

	<!-- ---------------------------------------------------------------- -->
	<!-- COVER IMAGE FULL PREVIEW MODAL (720p)                            -->
	<!-- ---------------------------------------------------------------- -->
	{#if previewBook}
		{@const col = idToColor(previewBook.id)}
		<div
			class="fixed inset-0 bg-black/85 backdrop-blur-xs flex items-center justify-center p-4 z-50 select-text"
			role="dialog"
			aria-modal="true"
		>
			<div class="bg-white border-4 border-black p-4 max-w-md w-full relative text-black shadow-[6px_6px_0px_0px_rgba(0,0,0,1)]">
				<button
					onclick={() => (previewBook = null)}
					class="absolute top-3 right-3 p-1.5 border-2 border-black bg-white hover:bg-neutral-100 text-black cursor-pointer z-10"
					aria-label="Zavřít"
				>
					<X class="w-5 h-5" />
				</button>

				<h3 class="text-sm font-black uppercase tracking-tight mb-2 pr-8">
					NÁHLED UČEBNICE ({previewBook.id})
				</h3>

				<div class="border-2 border-black bg-neutral-100 w-full aspect-3/4 max-h-[60vh] overflow-hidden flex items-center justify-center mb-3">
					{#if previewBook.photo}
						<img
							src={getBookFullImageUrl(previewBook)}
							alt={previewBook.id}
							class="w-full h-full object-contain"
						/>
					{:else}
						<span class="text-xs font-bold text-neutral-400 uppercase">Fotografie není k dispozici</span>
					{/if}
				</div>

				<div class="flex items-center justify-between p-2 bg-neutral-50 border-2 border-black text-xs font-black">
					<span
						class="px-2 py-0.5 text-white border"
						style="background-color: {col.bg}; border-color: {col.border};"
					>
						{previewBook.id}
					</span>
					<span class="text-base">{previewBook.price} Kč</span>
				</div>

				<button
					onclick={() => (previewBook = null)}
					class="w-full mt-3 py-2.5 bg-black text-white hover:bg-neutral-800 text-xs font-black uppercase tracking-wider border-2 border-black cursor-pointer"
				>
					ZAVŘÍT NÁHLED
				</button>
			</div>
		</div>
	{/if}

	<!-- ---------------------------------------------------------------- -->
	<!-- CHECKOUT DIALOG MODAL                                            -->
	<!-- ---------------------------------------------------------------- -->
	{#if isCheckoutModalOpen}
		<div
			class="fixed inset-0 bg-black/85 backdrop-blur-xs flex items-center justify-center p-4 z-50 select-text"
			role="dialog"
			aria-modal="true"
		>
			<div class="bg-white border-4 border-black p-6 max-w-md w-full relative text-black shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]">
				<button
					onclick={() => (isCheckoutModalOpen = false)}
					class="absolute top-3 right-3 p-1.5 border-2 border-black bg-white hover:bg-neutral-100 text-black cursor-pointer"
					aria-label="Zavřít"
				>
					<X class="w-5 h-5" />
				</button>

				<h2 class="text-lg font-black uppercase tracking-tight mb-1">DOKONČENÍ NÁKUPU</h2>
				<p class="text-xs font-bold text-neutral-600 uppercase mb-4">
					Zadejte email zákazníka pro přiřazení nákupu
				</p>

				<!-- Summary Box -->
				<div class="bg-neutral-100 border-2 border-black p-3 mb-4 flex items-center justify-between">
					<span class="text-xs font-black uppercase">POLOŽEK: {cartBooks.length} KNIH</span>
					<span class="text-lg font-black">{totalAmount} Kč</span>
				</div>

				<!-- Email Input with live search -->
				<div class="relative mb-3">
					<label for="buyer-email" class="block text-[10px] font-black uppercase tracking-wider text-neutral-700 mb-1">
						EMAIL KUPUJÍCÍHO
					</label>
					<input
						id="buyer-email"
						type="email"
						bind:value={checkoutEmail}
						oninput={handleEmailInput}
						placeholder="např. jan.novak@skola.cz"
						class="w-full bg-white border-2 border-black px-3 py-2 text-xs font-black text-black outline-none focus:border-black"
					/>

					<!-- Autocomplete dropdown suggestions -->
					{#if emailSearchResults.length > 0}
						<div class="absolute left-0 right-0 top-full mt-1 bg-white border-2 border-black shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] z-30 max-h-48 overflow-y-auto">
							{#each emailSearchResults as u}
								<button
									type="button"
									onclick={() => selectUserSuggestion(u)}
									class="w-full p-2 text-left hover:bg-neutral-100 border-b border-neutral-200 last:border-0 flex items-center justify-between cursor-pointer"
								>
									<div>
										<div class="text-xs font-black text-black">{u.email}</div>
										{#if u.name}
											<div class="text-[10px] text-neutral-600 font-bold">{u.name}</div>
										{/if}
									</div>
									<Check class="w-3.5 h-3.5 text-black" />
								</button>
							{/each}
						</div>
					{/if}

					{#if checkoutEmail && emailSearchResults.length === 0 && !isSearchingUsers}
						<div class="mt-1 text-[10px] font-bold text-neutral-500 uppercase">
							Nový zákazník – účet bude v PocketBase vytvořen automaticky
						</div>
					{/if}
				</div>

				<!-- Optional Name Input -->
				<div class="mb-4">
					<label for="buyer-name" class="block text-[10px] font-black uppercase tracking-wider text-neutral-700 mb-1">
						JMÉNO KUPUJÍCÍHO (VOLITELNÉ)
					</label>
					<input
						id="buyer-name"
						type="text"
						bind:value={checkoutName}
						placeholder="např. Jan Novák"
						class="w-full bg-white border-2 border-black px-3 py-2 text-xs font-black text-black outline-none"
					/>
				</div>

				{#if checkoutError}
					<div class="p-2.5 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold mb-4 flex items-center gap-2">
						<AlertCircle class="w-4 h-4 shrink-0" />
						<span>{checkoutError}</span>
					</div>
				{/if}

				<div class="flex gap-2">
					<button
						type="button"
						onclick={() => (isCheckoutModalOpen = false)}
						class="flex-1 py-3 bg-white hover:bg-neutral-100 text-black text-xs font-black uppercase tracking-wider border-2 border-black cursor-pointer"
					>
						ZPĚT
					</button>
					<button
						type="button"
						onclick={handleConfirmCheckout}
						disabled={isSubmittingCheckout}
						class="flex-1 py-3 bg-black hover:bg-neutral-800 text-white text-xs font-black uppercase tracking-wider border-2 border-black flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50"
					>
						{#if isSubmittingCheckout}
							<RefreshCw class="w-4 h-4 animate-spin" />
						{/if}
						<span>POKRAČOVAT K PLATBĚ</span>
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- ---------------------------------------------------------------- -->
	<!-- PAYMENT FINALIZATION VIEW (SPAYD QR & CASH CONFIRMATION)          -->
	<!-- ---------------------------------------------------------------- -->
	{#if paymentMode === 'PAYMENT' && activePayment}
		<div class="absolute inset-0 bg-white z-40 p-4 sm:p-6 overflow-y-auto flex flex-col items-center justify-center text-black">
			<div class="w-full max-w-md bg-white border-4 border-black p-6 shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] text-center">
				<h2 class="text-2xl font-black uppercase tracking-tight mb-1">PLATBA NÁKUPU</h2>
				<p class="text-xs font-bold text-neutral-600 uppercase mb-4">
					Zákazník může zaplatit QR kódem v bance nebo hotově
				</p>

				<!-- Big Czech SPAYD QR Canvas -->
				<div class="bg-white p-4 border-4 border-black inline-block mx-auto mb-4">
					<canvas bind:this={qrCanvas} class="max-w-full h-auto"></canvas>
				</div>

				<!-- Total Price & VS Details -->
				<div class="bg-neutral-50 border-2 border-black p-3.5 text-xs text-left mb-6 space-y-1.5 font-mono">
					<div class="flex justify-between border-b border-neutral-300 pb-1">
						<span class="font-sans font-black text-neutral-600 uppercase text-[11px]">ČÁSTKA:</span>
						<span class="font-sans font-black text-black text-xl">{activePayment.totalAmount} Kč</span>
					</div>
					<div class="flex justify-between border-b border-neutral-300 pb-1">
						<span class="font-sans font-black text-neutral-600 uppercase text-[11px]">VARIABILNÍ SYMBOL:</span>
						<span class="font-black text-black text-base">{activePayment.variableSymbol}</span>
					</div>
					<div class="flex justify-between border-b border-neutral-300 pb-1">
						<span class="font-sans font-black text-neutral-600 uppercase text-[11px]">ÚČET (IBAN):</span>
						<span class="font-bold text-black text-[11px]">{activeEvent?.iban || 'CZ6520100000002101234567'}</span>
					</div>
					{#if currentBuyer}
						<div class="flex justify-between">
							<span class="font-sans font-black text-neutral-600 uppercase text-[11px]">KUPUJÍCÍ:</span>
							<span class="font-sans font-bold text-black text-[11px] truncate max-w-[200px]">{currentBuyer.email}</span>
						</div>
					{/if}
				</div>

				{#if paymentError}
					<div class="p-2.5 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold mb-4 flex items-center gap-2">
						<AlertCircle class="w-4 h-4 shrink-0" />
						<span>{paymentError}</span>
					</div>
				{/if}

				<!-- Action buttons -->
				<div class="space-y-2">
					<!-- Prominent Cash Confirmation Button -->
					<button
						type="button"
						onclick={handleConfirmCash}
						disabled={isConfirmingPayment}
						class="w-full py-4 px-6 bg-black hover:bg-neutral-800 text-white font-black text-sm uppercase tracking-wider border-4 border-black flex items-center justify-center gap-2 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] cursor-pointer active:scale-98 transition-transform"
					>
						{#if isConfirmingPayment}
							<RefreshCw class="w-5 h-5 animate-spin" />
						{:else}
							<Banknote class="w-5 h-5" />
						{/if}
						<span>ZAPLACENO HOTOVĚ ({activePayment.totalAmount} Kč)</span>
					</button>

					<!-- Leave pending for bank payment -->
					<button
						type="button"
						onclick={resetForNextCustomer}
						class="w-full py-2.5 px-4 bg-white hover:bg-neutral-100 text-black font-black text-xs uppercase tracking-wider border-2 border-black flex items-center justify-center gap-1.5 cursor-pointer"
					>
						<Receipt class="w-4 h-4" />
						<span>PONECHAT JAKO ČEKAJÍCÍ QR PLATBU (DALŠÍ ZÁKAZNÍK)</span>
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- ---------------------------------------------------------------- -->
	<!-- SUCCESS VIEW                                                     -->
	<!-- ---------------------------------------------------------------- -->
	{#if paymentMode === 'SUCCESS'}
		<div class="absolute inset-0 bg-white z-40 p-6 overflow-y-auto flex flex-col items-center justify-center text-black">
			<div class="w-full max-w-md bg-white border-4 border-black p-8 shadow-[8px_8px_0px_0px_rgba(0,0,0,1)] text-center">
				<div class="inline-flex p-4 bg-emerald-100 border-4 border-black mb-4">
					<CheckCircle2 class="w-12 h-12 text-emerald-800" />
				</div>

				<h2 class="text-2xl font-black uppercase tracking-tight mb-2">PLATBA DOKONČENA!</h2>
				<p class="text-xs font-bold text-neutral-600 uppercase max-w-xs mx-auto mb-6">
					Platba byla úspěšně zaznamenána. Učebnice byly označeny jako prodané.
				</p>

				<button
					type="button"
					onclick={resetForNextCustomer}
					class="w-full py-4 px-6 bg-black hover:bg-neutral-800 text-white font-black text-sm uppercase tracking-wider border-2 border-black cursor-pointer shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]"
				>
					DALŠÍ ZÁKAZNÍK
				</button>
			</div>
		</div>
	{/if}
</div>
