<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
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
		ZoomIn,
		Check,
		Camera
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

	// Bottom Sheet state & DOM elements
	let sheetElement = $state<HTMLElement | null>(null);
	let headerElement = $state<HTMLElement | null>(null);
	let sheetScrollContainer = $state<HTMLElement | null>(null);

	let sheetExpanded = $state(false);
	let isDraggingSheet = $state(false);
	let sheetStartY = 0;
	let sheetDragDeltaY = $state(0);
	let sheetHeight = $state(500);
	let headerHeight = $state(92);

	let collapsedOffset = $derived(Math.max(0, sheetHeight - headerHeight));
	let sheetTranslateY = $derived.by(() => {
		if (isDraggingSheet) {
			if (sheetExpanded) {
				return Math.max(0, Math.min(collapsedOffset, sheetDragDeltaY));
			} else {
				return Math.max(0, Math.min(collapsedOffset, collapsedOffset + sheetDragDeltaY));
			}
		}
		return sheetExpanded ? 0 : collapsedOffset;
	});

	// Swipe gesture tracking for cart items & buyer card
	let pendingItemId: string | null = null;
	let pointerDownX = 0;
	let pointerDownY = 0;
	let swipingItemId = $state<string | null>(null);
	let swipeDeltaX = $state(0);
	let removingItemIds = $state<Set<string>>(new Set());

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
	let isExistingBuyer = $derived(
		!!currentBuyer &&
		!!currentBuyer.id &&
		currentBuyer.email.trim().toLowerCase() === checkoutEmail.trim().toLowerCase()
	);

	// ----------------------------------------------------
	// 5. PAYMENT & FINALIZATION STATE
	// ----------------------------------------------------
	interface PreparedCheckout {
		variableSymbol: number;
		totalAmount: number;
		buyer: BuyerInfo;
		event: AppEvent | null;
	}

	let paymentMode = $state<'PAYMENT' | 'SUCCESS' | null>(null);
	let preparedCheckout = $state<PreparedCheckout | null>(null);
	let activePayment = $state<Payment | null>(null);
	let activeEvent = $state<AppEvent | null>(null);
	let qrCanvas = $state<HTMLCanvasElement | null>(null);
	let isConfirmingPayment = $state(false);
	let paymentError = $state('');
	let confirmAction = $state<'CASH' | 'QR' | null>(null);
	let unsubBooksRealtime: (() => void) | null = null;

	// Pure standard mode gating:
	// Scanner strictly scans and auto-adds ONLY in pure standard camera view
	let canScan = $derived(
		paymentMode === null &&
		!isCheckoutModalOpen &&
		!previewBook &&
		!sheetExpanded &&
		!isDraggingSheet
	);

	// ----------------------------------------------------
	// LIFECYCLE & DIMENSIONS
	// ----------------------------------------------------

	function updateSheetDimensions() {
		if (sheetElement) sheetHeight = sheetElement.offsetHeight || 500;
		if (headerElement) headerHeight = headerElement.offsetHeight || 92;
	}

	$effect(() => {
		if (sheetElement) sheetHeight = sheetElement.offsetHeight || 500;
		if (headerElement) headerHeight = headerElement.offsetHeight || 92;
	});

	onMount(() => {
		updateSheetDimensions();
		window.addEventListener('resize', updateSheetDimensions);
		startCamera();

		// Subscribe to real-time book updates to keep AR overlay in sync
		pb.collection('books').subscribe<Book>('*', (e) => {
			if (e.action === 'update' || e.action === 'create') {
				const cached = codeLookupCache.get(e.record.id);
				if (cached && cached.type === 'book') {
					cached.book = { ...cached.book, ...e.record };
				} else if (!cached) {
					codeLookupCache.set(e.record.id, {
						type: 'book',
						book: e.record
					});
				}
			}
		}).then((unsub) => {
			unsubBooksRealtime = unsub;
		}).catch((err) => {
			console.warn('Realtime books subscription failed', err);
		});
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('resize', updateSheetDimensions);
		}
		if (unsubBooksRealtime) {
			unsubBooksRealtime();
			unsubBooksRealtime = null;
		}
		stopCamera();
	});

	// Re-render SPAYD QR code whenever entering payment mode
	$effect(() => {
		if (paymentMode === 'PAYMENT' && qrCanvas && preparedCheckout) {
			const ev = preparedCheckout.event || activeEvent || eventStore.event;
			const iban = ev?.iban || 'CZ6520100000002101234567';
			const payerEmail = currentBuyer?.email || checkoutEmail.trim() || preparedCheckout.buyer.email || '';
			renderSpaydQRCode(qrCanvas, {
				iban: iban,
				amount: preparedCheckout.totalAmount,
				vs: preparedCheckout.variableSymbol,
				payerEmail: payerEmail
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
			if (typeof cancelAnimationFrame !== 'undefined') {
				cancelAnimationFrame(renderAnimId);
			}
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

		// Pure standard mode: pause detection loop and clear stale boxes when sheets/modals open
		if (!canScan) {
			if (trackedMatches.size > 0) {
				trackedMatches.clear();
			}
			setTimeout(runDetectionLoop, 100);
			return;
		}

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

				// Only render AR polygons when in pure standard scanning mode
				if (canScan) {
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
							const rawName = currentBuyer.name || 'ZÁKAZNÍK';
							const displayName = rawName.length > 10 ? rawName.slice(0, 9) + '…' : rawName;
							drawPricePolygon(ctx, match.position, `✓ ${displayName}`, transform, {
								bg: '#059669',
								border: '#047857',
								text: '#ffffff',
								lightBg: '#d1fae5'
							});
							continue;
						}

						// 3. What if it's a book cached or being added?
						const cached = codeLookupCache.get(code);
						if (cached?.type === 'book' && !suppressedBooks.has(code)) {
							if (cached.book.status === 'available') {
								if (cached.book.accepted) {
									const color = idToColor(cached.book.id);
									drawPricePolygon(ctx, match.position, `${cached.book.price} Kč`, transform, color);
								} else {
									drawPricePolygon(ctx, match.position, 'NESCHVÁLENO', transform, {
										bg: '#ef4444',
										border: '#b91c1c',
										text: '#ffffff',
										lightBg: '#fee2e2'
									});
								}
							} else if (cached.book.status === 'checkout') {
								drawPricePolygon(ctx, match.position, 'ČEKÁ NA PLATBU', transform, {
									bg: '#e5e5e5',
									border: '#000000',
									text: '#000000',
									lightBg: '#f5f5f5'
								});
							} else if (cached.book.status === 'bought') {
								drawPricePolygon(ctx, match.position, 'PRODÁNO', transform, {
									bg: '#f5f5f5',
									border: '#737373',
									text: '#737373',
									lightBg: '#fafafa'
								});
							}
						}
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
			if (book.status === 'available' && book.accepted) {
				if (!suppressedBooks.has(book.id) && !cartBooks.some((b) => b.id === book.id)) {
					// AUTO-ADD TO CART!
					cartBooks = [...cartBooks, book];
					if (navigator.vibrate) navigator.vibrate([50]);
				}
			}
		}
	}

	// ----------------------------------------------------
	// BOTTOM SHEET DRAG & TOGGLE HANDLERS
	// ----------------------------------------------------
	function handleSheetHandlePointerDown(e: PointerEvent) {
		if (e.button !== 0) return;
		const target = e.target as HTMLElement;
		if (target.closest('button')) return;

		updateSheetDimensions();
		isDraggingSheet = true;
		sheetStartY = e.clientY;
		sheetDragDeltaY = 0;
		try {
			(e.currentTarget as HTMLElement)?.setPointerCapture(e.pointerId);
		} catch {}
	}

	function handleSheetHandlePointerMove(e: PointerEvent) {
		if (!isDraggingSheet) return;
		sheetDragDeltaY = e.clientY - sheetStartY;
	}

	function handleSheetHandlePointerUp(e: PointerEvent) {
		if (!isDraggingSheet) return;
		isDraggingSheet = false;
		try {
			(e.currentTarget as HTMLElement)?.releasePointerCapture(e.pointerId);
		} catch {}

		const threshold = 40;
		if (sheetExpanded) {
			if (sheetDragDeltaY > threshold) {
				sheetExpanded = false;
			}
		} else {
			if (sheetDragDeltaY < -threshold) {
				sheetExpanded = true;
			}
		}
		sheetDragDeltaY = 0;
	}

	function handleHeaderClick(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (target.closest('button')) return;
		if (Math.abs(sheetDragDeltaY) < 5) {
			sheetExpanded = !sheetExpanded;
		}
	}

	// Pull-down gesture on scroll container when at scrollTop === 0
	let contentTouchStartY = 0;
	let isContentPulling = false;

	function handleContentPointerDown(e: PointerEvent) {
		if (!sheetScrollContainer || sheetScrollContainer.scrollTop > 0) return;
		const target = e.target as HTMLElement;
		if (target.closest('button') || target.closest('[data-item-row]')) return;

		contentTouchStartY = e.clientY;
		isContentPulling = true;
	}

	function handleContentPointerMove(e: PointerEvent) {
		if (!isContentPulling || !sheetScrollContainer || sheetScrollContainer.scrollTop > 0) return;
		const deltaY = e.clientY - contentTouchStartY;
		if (deltaY > 15 && sheetExpanded) {
			if (swipingItemId) return;
			isDraggingSheet = true;
			sheetDragDeltaY = deltaY;
		}
	}

	function handleContentPointerUp(e: PointerEvent) {
		if (isContentPulling) {
			isContentPulling = false;
			if (isDraggingSheet) {
				handleSheetHandlePointerUp(e);
			}
		}
	}

	// ----------------------------------------------------
	// CART INTERACTIONS (SWIPE-TO-DELETE & UNLINK)
	// ----------------------------------------------------
	function handleItemPointerDown(e: PointerEvent, id: string) {
		if (e.button !== 0) return;
		const target = e.target as HTMLElement;
		if (target.closest('button')) return;

		pendingItemId = id;
		pointerDownX = e.clientX;
		pointerDownY = e.clientY;
		swipingItemId = null;
		swipeDeltaX = 0;
	}

	function handleItemPointerMove(e: PointerEvent, id: string) {
		if (swipingItemId === id) {
			const delta = e.clientX - pointerDownX;
			// Allow smooth drag right up to 220px, with subtle left rubber-band (-25px)
			swipeDeltaX = Math.max(-25, Math.min(220, delta));
			return;
		}

		if (pendingItemId === id) {
			const dx = Math.abs(e.clientX - pointerDownX);
			const dy = Math.abs(e.clientY - pointerDownY);

			// User is scrolling vertically
			if (dy > 8 && dy >= dx) {
				pendingItemId = null;
				return;
			}

			// User is swiping horizontally to delete
			if (dx > 8 && dx > dy) {
				swipingItemId = id;
				swipeDeltaX = Math.max(-25, Math.min(220, e.clientX - pointerDownX));
				try {
					(e.currentTarget as HTMLElement)?.setPointerCapture(e.pointerId);
				} catch {}
			}
		}
	}

	function handleItemPointerUp(e: PointerEvent, id: string, isBuyer = false) {
		if (swipingItemId === id) {
			try {
				(e.currentTarget as HTMLElement)?.releasePointerCapture(e.pointerId);
			} catch {}

			const committed = swipeDeltaX >= 80;
			if (committed) {
				if (isBuyer) {
					triggerUnlinkBuyer();
				} else {
					triggerRemoveBook(id);
				}
			}

			swipingItemId = null;
			swipeDeltaX = 0;
		}
		pendingItemId = null;
	}

	function removeBookFromCart(bookId: string) {
		cartBooks = cartBooks.filter((b) => b.id !== bookId);
		// Suppress immediately: stays suppressed as long as in camera view,
		// and re-arms only if it leaves the camera view for >= 2 seconds!
		suppressedBooks.set(bookId, Date.now());
		if (swipingItemId === bookId) {
			swipingItemId = null;
			swipeDeltaX = 0;
		}
	}

	function triggerRemoveBook(bookId: string) {
		removingItemIds.add(bookId);
		removingItemIds = new Set(removingItemIds);
		setTimeout(() => {
			removeBookFromCart(bookId);
			removingItemIds.delete(bookId);
			removingItemIds = new Set(removingItemIds);
		}, 200);
	}

	function unlinkBuyer() {
		currentBuyer = null;
		if (swipingItemId === 'buyer') {
			swipingItemId = null;
			swipeDeltaX = 0;
		}
	}

	function triggerUnlinkBuyer() {
		removingItemIds.add('buyer');
		removingItemIds = new Set(removingItemIds);
		setTimeout(() => {
			unlinkBuyer();
			removingItemIds.delete('buyer');
			removingItemIds = new Set(removingItemIds);
		}, 200);
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
		sheetExpanded = false;
		isCheckoutModalOpen = true;
	}

	function handleEmailInput() {
		clearTimeout(searchTimeout);

		const query = checkoutEmail.trim().toLowerCase();
		if (currentBuyer && currentBuyer.email.trim().toLowerCase() !== query) {
			currentBuyer = null;
		}

		if (query.length >= 2) {
			searchTimeout = setTimeout(async () => {
				isSearchingUsers = true;
				try {
					const results = await pb.send<Array<{ id: string; email: string; name: string }>>(
						`/api/cashier/user-search?query=${encodeURIComponent(query)}`,
						{ method: 'GET' }
					);
					emailSearchResults = results;

					// If user typed an exact match, automatically recognize them
					const exact = results.find(
						(u) => u.email.trim().toLowerCase() === query
					);
					if (exact && (!currentBuyer || currentBuyer.id !== exact.id)) {
						currentBuyer = exact;
						if (!checkoutName.trim() && exact.name) {
							checkoutName = exact.name;
						}
					}
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
				variableSymbol: number;
				totalAmount: number;
				buyer: BuyerInfo;
				event: AppEvent;
			}>('/api/cashier/prepare-checkout', {
				method: 'POST',
				body: {
					email: email,
					name: checkoutName.trim(),
					buyerId: currentBuyer?.id || '',
					bookIds: cartBooks.map((b) => b.id)
				}
			});

			preparedCheckout = {
				variableSymbol: res.variableSymbol,
				totalAmount: res.totalAmount,
				buyer: res.buyer,
				event: res.event || eventStore.event
			};
			activeEvent = res.event || eventStore.event;
			currentBuyer = res.buyer;

			isCheckoutModalOpen = false;
			sheetExpanded = false;
			paymentMode = 'PAYMENT';
			paymentError = '';
		} catch (err: any) {
			console.error('Checkout preparation failed', err);
			checkoutError = err?.message || 'Chyba při přípravě platby. Zkontrolujte dostupnost knih.';
		} finally {
			isSubmittingCheckout = false;
		}
	}

	// ----------------------------------------------------
	// PAYMENT CONFIRMATION & FINALIZATION
	// ----------------------------------------------------
	async function handleConfirmCash() {
		if (!preparedCheckout) return;
		isConfirmingPayment = true;
		paymentError = '';

		try {
			const res = await pb.send<{ success: boolean; payment: Payment }>('/api/cashier/finalize-checkout', {
				method: 'POST',
				body: {
					method: 'cash',
					buyerId: preparedCheckout.buyer.id,
					bookIds: cartBooks.map((b) => b.id),
					variableSymbol: preparedCheckout.variableSymbol
				}
			});

			activePayment = res.payment;
			paymentMode = 'SUCCESS';
		} catch (err: any) {
			console.error('Failed to confirm cash payment', err);
			paymentError = err?.message || 'Chyba při potvrzení hotovostní platby.';
		} finally {
			isConfirmingPayment = false;
		}
	}

	async function handleConfirmQR() {
		if (!preparedCheckout) return;
		isConfirmingPayment = true;
		paymentError = '';

		try {
			await pb.send('/api/cashier/finalize-checkout', {
				method: 'POST',
				body: {
					method: 'qr',
					buyerId: preparedCheckout.buyer.id,
					bookIds: cartBooks.map((b) => b.id),
					variableSymbol: preparedCheckout.variableSymbol
				}
			});

			resetForNextCustomer();
		} catch (err: any) {
			console.error('Failed to finalize QR payment', err);
			paymentError = err?.message || 'Chyba při dokončení QR platby.';
		} finally {
			isConfirmingPayment = false;
		}
	}

	function resetForNextCustomer() {
		cartBooks = [];
		currentBuyer = null;
		suppressedBooks.clear();
		codeLookupCache.clear();
		preparedCheckout = null;
		activePayment = null;
		paymentMode = null;
		paymentError = '';
		confirmAction = null;
		sheetExpanded = false;
		removingItemIds.clear();
		swipingItemId = null;
		swipeDeltaX = 0;
	}
</script>

<div class="relative flex-1 w-full h-full bg-black overflow-hidden flex flex-col select-none touch-none">
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
			bind:this={sheetElement}
			class="fixed bottom-0 inset-x-0 z-30 max-w-2xl mx-auto w-full bg-white text-black flex flex-col shadow-[0_-8px_24px_rgba(0,0,0,0.2)] overflow-hidden {isDraggingSheet ? '' : 'transition-transform duration-300 ease-out'}"
			style="height: min(80vh, 650px); transform: translateY({isDraggingSheet ? sheetTranslateY + 'px' : sheetExpanded ? '0px' : 'calc(100% - ' + headerHeight + 'px)'});"
		>
			<!-- DRAG / TOGGLE HEADER -->
			<div
				bind:this={headerElement}
				role="button"
				tabindex="0"
				onpointerdown={handleSheetHandlePointerDown}
				onpointermove={handleSheetHandlePointerMove}
				onpointerup={handleSheetHandlePointerUp}
				onpointercancel={handleSheetHandlePointerUp}
				onclick={handleHeaderClick}
				onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') sheetExpanded = !sheetExpanded; }}
				class="w-full flex flex-col items-center pt-2 pb-2.5 px-4 cursor-grab active:cursor-grabbing select-none bg-neutral-50 hover:bg-neutral-100 border-b-2 border-black shrink-0 touch-none"
			>
				<!-- Subtle drag handle bar pill -->
				<div class="w-14 h-1.5 bg-neutral-400 rounded-full mb-2"></div>

				<div class="w-full flex items-center justify-between gap-2">
					<!-- Cart summary & counts -->
					<div class="flex items-center gap-2 min-w-0">
						<span class="text-xs font-black uppercase tracking-tight bg-black text-white px-2 py-0.5 border border-black shrink-0">
							{cartBooks.length} ks
						</span>
						<span class="text-base sm:text-lg font-black text-black truncate">
							{totalAmount} Kč
						</span>
					</div>

					<!-- Buyer Pill or Zaplatit Action -->
					<div class="flex items-center gap-1.5 shrink-0">
						{#if currentBuyer}
							<div
								class="flex items-center gap-1 bg-emerald-100 border border-emerald-800 px-2 py-0.5 text-emerald-900 text-xs font-black truncate max-w-[110px] sm:max-w-[140px]"
								title="{currentBuyer.name} ({currentBuyer.email})"
							>
								<User class="w-3.5 h-3.5 shrink-0 text-emerald-800" />
								<span class="truncate">{currentBuyer.name || currentBuyer.email.split('@')[0]}</span>
							</div>
						{/if}

						{#if !sheetExpanded}
							<button
								type="button"
								onclick={(e) => {
									e.stopPropagation();
									openCheckoutModal();
								}}
								disabled={cartBooks.length === 0}
								class="py-1.5 px-3 font-black text-xs uppercase tracking-wider border-2 border-black transition-all flex items-center gap-1 {cartBooks.length > 0 ? 'bg-black text-white hover:bg-neutral-800 active:scale-95 cursor-pointer' : 'bg-neutral-200 text-neutral-400 border-neutral-300 cursor-not-allowed'}"
							>
								<span>ZAPLATIT</span>
							</button>
						{/if}

					</div>
				</div>
			</div>

			<!-- EXPANDED CONTENT (Scrollable & always mounted for silky smooth sliding) -->
			<div
				bind:this={sheetScrollContainer}
				role="region"
				aria-label="Položky košíku"
				onpointerdown={handleContentPointerDown}
				onpointermove={handleContentPointerMove}
				onpointerup={handleContentPointerUp}
				onpointercancel={handleContentPointerUp}
				class="flex-1 overflow-y-auto p-4 space-y-4"
				inert={!sheetExpanded ? true : undefined}
			>
				<!-- BUYER CARD SECTION (Shown only when assigned) -->
				{#if currentBuyer}
					{@const isBuyerSwiping = swipingItemId === 'buyer'}
					{@const isBuyerRemoving = removingItemIds.has('buyer')}
					<!-- Swipeable Buyer Card Container with collapse animation -->
					<div
						data-item-row
						class="relative overflow-hidden border-2 border-black bg-emerald-50 select-none transition-all duration-200 ease-out {isBuyerRemoving ? 'opacity-0 translate-x-full max-h-0 !my-0 !py-0 !border-0' : 'max-h-24'}"
					>
						<!-- Red track revealed on swipe right -->
						<div
							class="absolute inset-0 bg-red-600 flex items-center px-4 gap-2 text-white font-black text-xs uppercase transition-opacity duration-100"
							style="opacity: {isBuyerSwiping ? Math.min(1, Math.max(0, swipeDeltaX / 50)) : 0};"
						>
							<UserMinus class="w-4 h-4 shrink-0" />
							<span>ODPOJIT KUPUJÍCÍHO</span>
						</div>

						<!-- Foreground sliding card -->
						<div
							role="presentation"
							class="relative bg-emerald-50 p-3 flex items-center justify-between gap-3 cursor-grab active:cursor-grabbing touch-pan-y {isBuyerSwiping ? '' : 'transition-transform duration-200 ease-out'}"
							style="transform: translateX({isBuyerSwiping ? swipeDeltaX : 0}px);"
							onpointerdown={(e) => handleItemPointerDown(e, 'buyer')}
							onpointermove={(e) => handleItemPointerMove(e, 'buyer')}
							onpointerup={(e) => handleItemPointerUp(e, 'buyer', true)}
							onpointercancel={(e) => handleItemPointerUp(e, 'buyer', true)}
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
								<!-- Explicit 1-tap Unlink Button -->
								<button
									type="button"
									onclick={(e) => {
										e.stopPropagation();
										triggerUnlinkBuyer();
									}}
									class="p-1.5 border-2 border-black bg-white hover:bg-red-600 hover:text-white text-black transition-colors cursor-pointer shrink-0 active:scale-95"
									title="Odpojit kupujícího"
								>
									<X class="w-4 h-4" />
								</button>
							</div>
						</div>
					</div>
				{/if}

				<!-- BOOKS LIST SECTION -->
				<div>

					{#if cartBooks.length === 0}
						<p class="text-xs font-bold uppercase text-neutral-400 text-center py-6">
							Košík je prázdný
						</p>
					{:else}
						<div class="space-y-2">
							{#each cartBooks as book (book.id)}
								{@const col = idToColor(book.id)}
								{@const isSwiping = swipingItemId === book.id}
								{@const isRemoving = removingItemIds.has(book.id)}
								<!-- Swipeable Book Card Container with collapse animation -->
								<div
									data-item-row
									class="relative overflow-hidden border-2 border-black bg-white select-none transition-all duration-200 ease-out {isRemoving ? 'opacity-0 translate-x-full max-h-0 !my-0 !py-0 !border-0' : 'max-h-28'}"
								>
									<!-- Red track revealed on swipe right -->
									<div
										class="absolute inset-0 bg-red-600 flex items-center px-4 gap-2 text-white font-black text-xs uppercase transition-opacity duration-100"
										style="opacity: {isSwiping ? Math.min(1, Math.max(0, swipeDeltaX / 50)) : 0};"
									>
										<Trash2 class="w-4 h-4 shrink-0" />
										<span>ODSTRANIT Z KOŠÍKU</span>
									</div>

									<!-- Foreground sliding row -->
									<div
										role="presentation"
										class="relative bg-white p-2.5 flex items-center justify-between gap-3 cursor-grab active:cursor-grabbing touch-pan-y {isSwiping ? '' : 'transition-transform duration-200 ease-out'}"
										style="transform: translateX({isSwiping ? swipeDeltaX : 0}px);"
										onpointerdown={(e) => handleItemPointerDown(e, book.id)}
										onpointermove={(e) => handleItemPointerMove(e, book.id)}
										onpointerup={(e) => handleItemPointerUp(e, book.id)}
										onpointercancel={(e) => handleItemPointerUp(e, book.id)}
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

											<!-- Explicit 1-tap Trash Can Button -->
											<button
												type="button"
												onclick={(e) => {
													e.stopPropagation();
													triggerRemoveBook(book.id);
												}}
												class="p-2 border-2 border-black bg-neutral-100 hover:bg-red-600 hover:text-white transition-colors cursor-pointer shrink-0 active:scale-95"
												title="Smazat knihu z košíku"
											>
												<Trash2 class="w-4 h-4" />
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
						<span>ZAPLATIT</span>
					</button>
				</div>
			</div>
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

					{#if isExistingBuyer}
						<div class="mt-1 text-[10px] font-bold text-emerald-700 uppercase flex items-center gap-1">
							<Check class="w-3 h-3 text-emerald-700" />
							<span>Registrovaný zákazník{currentBuyer?.name ? ` (${currentBuyer.name})` : ''}</span>
						</div>
					{:else if checkoutEmail && emailSearchResults.length === 0 && !isSearchingUsers}
						<div class="mt-1 text-[10px] font-bold text-neutral-500 uppercase">
							Nový zákazník – účet bude v databázi vytvořen automaticky
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
	{#if paymentMode === 'PAYMENT' && preparedCheckout}
		<div class="absolute inset-0 bg-white z-40 p-4 flex flex-col justify-between max-w-sm mx-auto text-black select-none overflow-y-auto">
			<!-- Compact Header: Amount & Variable Symbol -->
			<div class="border-b-2 border-black pb-2.5 pt-1">
				<div class="flex items-baseline justify-between">
					<div>
						<span class="text-[10px] font-mono font-bold text-neutral-500 uppercase block">Částka</span>
						<span class="text-3xl font-black text-black leading-none">{preparedCheckout.totalAmount} Kč</span>
					</div>
					<div class="text-right flex items-center gap-3">
						<div>
							<span class="text-[10px] font-mono font-bold text-neutral-500 uppercase block">Var. symbol</span>
							<span class="font-mono text-xl font-black text-black leading-none">{preparedCheckout.variableSymbol}</span>
						</div>
						<button
							type="button"
							onclick={() => {
								paymentMode = null;
								paymentError = '';
							}}
							class="p-1.5 border-2 border-black bg-white hover:bg-neutral-100 text-black cursor-pointer active:scale-95"
							title="Zpět do košíku"
							aria-label="Zpět do košíku"
						>
							<X class="w-4 h-4" />
						</button>
					</div>
				</div>
				{#if currentBuyer || preparedCheckout.buyer}
					{@const buyer = currentBuyer || preparedCheckout.buyer}
					<div class="text-[11px] font-bold text-neutral-600 truncate mt-1.5 flex items-center gap-1.5">
						<span class="text-neutral-400 font-normal">Kupující:</span>
						<span class="text-black font-mono font-bold truncate">{buyer.name ? `${buyer.name} (${buyer.email})` : buyer.email}</span>
					</div>
				{/if}
			</div>

			<!-- Centered SPAYD QR Canvas -->
			<div class="flex-1 flex flex-col items-center justify-center py-3 min-h-0">
				<div class="bg-white p-2 border-2 border-black shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]">
					<canvas bind:this={qrCanvas} class="w-48 h-48 sm:w-56 sm:h-56 max-h-[42vh] max-w-[42vh] aspect-square block"></canvas>
				</div>
			</div>

			<!-- Bottom Actions & Error -->
			<div class="space-y-2 pt-1 pb-1">
				{#if paymentError}
					<div class="p-2 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold flex items-center gap-2 mb-1">
						<AlertCircle class="w-4 h-4 shrink-0" />
						<span class="truncate">{paymentError}</span>
					</div>
				{/if}

				<!-- Cash Confirmation Button -->
				<button
					type="button"
					onclick={() => (confirmAction = 'CASH')}
					disabled={isConfirmingPayment}
					class="w-full py-3.5 px-4 bg-black hover:bg-neutral-800 text-white font-black text-sm uppercase tracking-wider border-2 border-black flex items-center justify-center gap-2 cursor-pointer active:scale-98 transition-transform disabled:opacity-50"
				>
					{#if isConfirmingPayment && confirmAction === 'CASH'}
						<RefreshCw class="w-4 h-4 animate-spin" />
					{:else}
						<Banknote class="w-4 h-4" />
					{/if}
					<span>Zaplaceno hotově</span>
				</button>

				<!-- Leave pending for QR payment -->
				<button
					type="button"
					onclick={() => (confirmAction = 'QR')}
					disabled={isConfirmingPayment}
					class="w-full py-2.5 px-4 bg-white hover:bg-neutral-100 text-black font-black text-xs uppercase tracking-wider border-2 border-black flex items-center justify-center gap-1.5 cursor-pointer active:scale-98 transition-transform disabled:opacity-50"
				>
					{#if isConfirmingPayment && confirmAction === 'QR'}
						<RefreshCw class="w-4 h-4 animate-spin" />
					{:else}
						<Receipt class="w-4 h-4" />
					{/if}
					<span>Zaplatí přes QR</span>
				</button>
			</div>
		</div>

		<!-- Confirmation Dialog for Payment Actions -->
		{#if confirmAction}
			<div class="fixed inset-0 bg-black/60 z-50 flex items-center justify-center p-4">
				<div class="w-full max-w-sm bg-white border-4 border-black p-5 shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] text-center">
					{#if confirmAction === 'CASH'}
						<div class="inline-flex p-3 bg-neutral-100 border-2 border-black mb-3">
							<Banknote class="w-8 h-8 text-black" />
						</div>
						<h3 class="text-base font-black uppercase mb-1">Potvrdit platbu hotově?</h3>
						<p class="text-xs text-neutral-600 font-bold mb-4">
							Opravdu bylo přijato <span class="text-black font-black">{preparedCheckout.totalAmount} Kč</span> v hotovosti?
						</p>
						<div class="flex gap-2">
							<button
								type="button"
								onclick={() => (confirmAction = null)}
								disabled={isConfirmingPayment}
								class="flex-1 py-2.5 bg-white hover:bg-neutral-100 text-black text-xs font-black uppercase tracking-wider border-2 border-black cursor-pointer"
							>
								ZRUŠIT
							</button>
							<button
								type="button"
								onclick={() => {
									const act = confirmAction;
									confirmAction = null;
									if (act === 'CASH') handleConfirmCash();
								}}
								disabled={isConfirmingPayment}
								class="flex-1 py-2.5 bg-black hover:bg-neutral-800 text-white text-xs font-black uppercase tracking-wider border-2 border-black cursor-pointer flex items-center justify-center gap-1.5"
							>
								{#if isConfirmingPayment}
									<RefreshCw class="w-3.5 h-3.5 animate-spin" />
								{/if}
								<span>POTVRDIT</span>
							</button>
						</div>
					{:else if confirmAction === 'QR'}
						<div class="inline-flex p-3 bg-neutral-100 border-2 border-black mb-3">
							<Receipt class="w-8 h-8 text-black" />
						</div>
						<h3 class="text-base font-black uppercase mb-1">Zaplatí přes QR?</h3>
						<p class="text-xs text-neutral-600 font-bold mb-4">
							Platba zůstane čekající na bankovní převod a můžete obsloužit dalšího zákazníka.
						</p>
						<div class="flex gap-2">
							<button
								type="button"
								onclick={() => (confirmAction = null)}
								disabled={isConfirmingPayment}
								class="flex-1 py-2.5 bg-white hover:bg-neutral-100 text-black text-xs font-black uppercase tracking-wider border-2 border-black cursor-pointer"
							>
								ZRUŠIT
							</button>
							<button
								type="button"
								onclick={() => {
									const act = confirmAction;
									confirmAction = null;
									if (act === 'QR') handleConfirmQR();
								}}
								disabled={isConfirmingPayment}
								class="flex-1 py-2.5 bg-black hover:bg-neutral-800 text-white text-xs font-black uppercase tracking-wider border-2 border-black cursor-pointer flex items-center justify-center gap-1.5"
							>
								{#if isConfirmingPayment}
									<RefreshCw class="w-3.5 h-3.5 animate-spin" />
								{/if}
								<span>POTVRDIT</span>
							</button>
						</div>
					{/if}
				</div>
			</div>
		{/if}
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
