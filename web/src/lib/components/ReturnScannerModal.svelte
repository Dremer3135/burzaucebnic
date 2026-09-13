<script lang="ts">
	import { onDestroy, onMount, tick, untrack } from 'svelte';
	import { pb, getBookThumbnailUrl } from '$lib/pocketbase';
	import type { Book, BookStatus } from '$lib/types';
	import {
		CAMERA_CONSTRAINTS,
		getVideoTransform,
		drawPricePolygon,
		idToColor,
		scanFrameForAllDataMatrices,
		type ScanMatch
	} from '$lib/scanner';
	import { X, Search, Check, AlertTriangle, Trash2, Camera, RefreshCw } from '@lucide/svelte';

	interface ReturnBookItem {
		id: string;
		price: number;
		photo: string;
		status: BookStatus;
		accepted: boolean;
		collectionId: string;
		collectionName: string;
	}

	let {
		open = $bindable(false),
		seller,
		unsoldBooks = [],
		soldBooks = [],
		returnedBooks = [],
		onsuccess
	}: {
		open?: boolean;
		seller: { id: string; name: string; email: string };
		unsoldBooks?: ReturnBookItem[];
		soldBooks?: ReturnBookItem[];
		returnedBooks?: ReturnBookItem[];
		onsuccess?: (returnedIds: string[]) => void;
	} = $props();

	// Video & canvas references (isolated from effect subscriptions via untrack)
	let videoElement = $state<HTMLVideoElement | null>(null);
	let captureCanvas = $state<HTMLCanvasElement | null>(null);
	let overlayCanvas = $state<HTMLCanvasElement | null>(null);

	let mediaStream: MediaStream | null = null;
	let isCameraReady = $state(false);
	let cameraError = $state<string | null>(null);
	let isScanningLoopActive = false;
	let isDetecting = false;
	let lastDetectTime = 0;
	let renderAnimId = 0;
	let cameraSessionId = 0;

	// State
	let scannedBooks = $state<ReturnBookItem[]>([]);
	const suppressedBooks = new Map<string, number>();
	let trackedMatches = new Map<string, { match: ScanMatch; lastSeen: number }>();

	// Manual search drawer
	let isSearchOpen = $state(false);
	let searchQuery = $state('');

	// Confirmation modal
	let isConfirmOpen = $state(false);
	let isSubmitting = $state(false);
	let errorMessage = $state<string | null>(null);

	// Swipe-to-delete state
	let swipingItemId = $state<string | null>(null);
	let pendingItemId = $state<string | null>(null);
	let pointerDownX = 0;
	let pointerDownY = 0;
	let swipeDeltaX = $state(0);
	let removingItemIds = $state<Set<string>>(new Set());

	// Unsold, sold and returned books lookup maps
	let unsoldMap = $derived(new Map(unsoldBooks.map((b) => [b.id, b])));
	let soldMap = $derived(new Map(soldBooks.map((b) => [b.id, b])));
	let returnedMap = $derived(new Map(returnedBooks.map((b) => [b.id, b])));

	// Filtered books for search drawer
	let availableForSearch = $derived(
		unsoldBooks
			.filter((b) => !scannedBooks.some((sb) => sb.id === b.id))
			.filter((b) => !searchQuery.trim() || b.id.toLowerCase().includes(searchQuery.trim().toLowerCase()))
	);

	let prevOpen = false;

	$effect(() => {
		const isOpen = open;
		if (isOpen === prevOpen) return;
		prevOpen = isOpen;

		untrack(() => {
			if (isOpen) {
				handleOpenModal();
			} else {
				handleCloseModal();
			}
		});
	});

	onDestroy(() => {
		stopCamera();
	});

	async function handleOpenModal() {
		scannedBooks = [];
		suppressedBooks.clear();
		trackedMatches.clear();
		errorMessage = null;
		cameraError = null;
		isConfirmOpen = false;
		isSearchOpen = false;
		await tick();
		await startCamera();
	}

	function handleCloseModal() {
		stopCamera();
	}

	async function startCamera() {
		const sessionId = ++cameraSessionId;
		stopCameraInternal();
		cameraError = null;

		try {
			await tick();
			let stream: MediaStream;
			try {
				stream = await navigator.mediaDevices.getUserMedia(CAMERA_CONSTRAINTS);
			} catch (firstErr) {
				console.warn('Primary camera constraints failed, attempting fallback:', firstErr);
				try {
					stream = await navigator.mediaDevices.getUserMedia({
						video: { facingMode: 'environment' },
						audio: false
					});
				} catch (secondErr) {
					console.warn('Secondary camera constraints failed, attempting basic video:', secondErr);
					stream = await navigator.mediaDevices.getUserMedia({
						video: true,
						audio: false
					});
				}
			}

			// If modal was closed or a newer session began while waiting for getUserMedia
			if (sessionId !== cameraSessionId || !open) {
				stream.getTracks().forEach((t) => t.stop());
				return;
			}

			mediaStream = stream;

			if (!videoElement) {
				await tick();
			}

			if (videoElement) {
				videoElement.srcObject = stream;
				try {
					await videoElement.play();
				} catch (playErr: any) {
					if (playErr.name !== 'AbortError') {
						throw playErr;
					}
				}

				if (sessionId !== cameraSessionId || !open) {
					stopCameraInternal();
					return;
				}

				isCameraReady = true;
				isScanningLoopActive = true;
				runDetectionLoop();
				runRenderLoop();
			}
		} catch (err: any) {
			if (sessionId !== cameraSessionId || !open) return;
			console.error('Failed to start return scanner camera:', err);
			cameraError = 'Nelze přistoupit ke kameře. Zkontrolujte prosím oprávnění v prohlížeči.';
		}
	}

	function stopCameraInternal() {
		isScanningLoopActive = false;
		if (renderAnimId) {
			cancelAnimationFrame(renderAnimId);
			renderAnimId = 0;
		}
		if (mediaStream) {
			mediaStream.getTracks().forEach((t) => t.stop());
			mediaStream = null;
		}
		if (videoElement) {
			videoElement.srcObject = null;
		}
		isCameraReady = false;
	}

	function stopCamera() {
		cameraSessionId++; // Invalidate any active or in-flight camera initialization
		stopCameraInternal();
	}

	async function runDetectionLoop() {
		if (!isScanningLoopActive) return;

		const now = performance.now();
		if (!isDetecting && now - lastDetectTime >= 40) {
			isDetecting = true;
			lastDetectTime = now;

			try {
				if (captureCanvas && videoElement && videoElement.videoWidth > 0) {
					const matches = await scanFrameForAllDataMatrices(captureCanvas, videoElement, 8);
					if (!isScanningLoopActive) return;

					const currentTime = Date.now();

					// Re-arm suppressed books if not seen in view for >= 2 seconds
					for (const [bookId, lastSeen] of suppressedBooks.entries()) {
						if (currentTime - lastSeen > 2000) {
							suppressedBooks.delete(bookId);
						}
					}

					for (const match of matches) {
						const code = match.text.trim();
						trackedMatches.set(code, { match, lastSeen: currentTime });
						handleScannedCode(code);
					}

					for (const [code, tracked] of trackedMatches.entries()) {
						if (currentTime - tracked.lastSeen > 800) {
							trackedMatches.delete(code);
						}
					}
				}
			} catch (err) {
				console.error('Return scanner detection error:', err);
			} finally {
				isDetecting = false;
			}
		}

		if (isScanningLoopActive) {
			setTimeout(runDetectionLoop, 25);
		}
	}

	function handleScannedCode(code: string) {
		if (suppressedBooks.has(code)) {
			suppressedBooks.set(code, Date.now());
			return;
		}

		const targetBook = unsoldMap.get(code);
		if (targetBook) {
			if (!scannedBooks.some((b) => b.id === code)) {
				scannedBooks = [...scannedBooks, targetBook];
				if (navigator.vibrate) navigator.vibrate([50]);
			}
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
				ctx.scale(dpr, dpr);
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

				for (const { match } of trackedMatches.values()) {
					const code = match.text.trim();
					const book = unsoldMap.get(code);

					if (book) {
						const isAlreadyInTray = scannedBooks.some((b) => b.id === code);
						if (isAlreadyInTray) {
							drawPricePolygon(ctx, match.position, `✓ V KOŠÍKU (${book.price} Kč)`, transform, {
								bg: '#15803d',
								border: '#166534',
								text: '#ffffff',
								lightBg: '#dcfce7'
							});
						} else {
							drawPricePolygon(ctx, match.position, `VRÁTIT (${book.price} Kč)`, transform, {
								bg: '#16a34a',
								border: '#15803d',
								text: '#ffffff',
								lightBg: '#dcfce7'
							});
						}
					} else if (returnedMap.has(code)) {
						// Already returned to this seller
						drawPricePolygon(ctx, match.position, 'VRÁCENO', transform, {
							bg: '#fee2e2',
							border: '#dc2626',
							text: '#dc2626',
							lightBg: '#fef2f2'
						});
					} else if (soldMap.has(code)) {
						// Already sold
						drawPricePolygon(ctx, match.position, 'JIŽ PRODÁNO', transform, {
							bg: '#737373',
							border: '#525252',
							text: '#ffffff',
							lightBg: '#fafafa'
						});
					} else {
						// Not belonging to this seller
						drawPricePolygon(ctx, match.position, 'CIZÍ KNIHA', transform, {
							bg: '#dc2626',
							border: '#b91c1c',
							text: '#ffffff',
							lightBg: '#fee2e2'
						});
					}
				}

				ctx.restore();
			}
		}

		renderAnimId = requestAnimationFrame(runRenderLoop);
	}

	// ----------------------------------------------------
	// SWIPE INTERACTIONS (IDENTICAL PHYSICS TO CASHIER)
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
			swipeDeltaX = Math.max(-25, Math.min(220, delta));
			return;
		}

		if (pendingItemId === id) {
			const dx = Math.abs(e.clientX - pointerDownX);
			const dy = Math.abs(e.clientY - pointerDownY);

			// Vertical scroll discrimination
			if (dy > 8 && dy >= dx) {
				pendingItemId = null;
				return;
			}

			// Horizontal swipe to delete
			if (dx > 8 && dx > dy) {
				swipingItemId = id;
				swipeDeltaX = Math.max(-25, Math.min(220, e.clientX - pointerDownX));
				try {
					(e.currentTarget as HTMLElement)?.setPointerCapture(e.pointerId);
				} catch {}
			}
		}
	}

	function handleItemPointerUp(e: PointerEvent, id: string) {
		if (swipingItemId === id) {
			try {
				(e.currentTarget as HTMLElement)?.releasePointerCapture(e.pointerId);
			} catch {}

			const committed = swipeDeltaX >= 80;
			if (committed) {
				triggerRemoveBook(id);
			}
			swipingItemId = null;
			swipeDeltaX = 0;
		}
		pendingItemId = null;
	}

	function removeBookFromTray(bookId: string) {
		scannedBooks = scannedBooks.filter((b) => b.id !== bookId);
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
			removeBookFromTray(bookId);
			removingItemIds.delete(bookId);
			removingItemIds = new Set(removingItemIds);
		}, 200);
	}

	function addAllRemaining() {
		const newBooks = unsoldBooks.filter((b) => !scannedBooks.some((sb) => sb.id === b.id));
		scannedBooks = [...scannedBooks, ...newBooks];
		if (navigator.vibrate) navigator.vibrate([100]);
	}

	function addBookManually(book: ReturnBookItem) {
		if (!scannedBooks.some((b) => b.id === book.id)) {
			scannedBooks = [...scannedBooks, book];
			suppressedBooks.delete(book.id);
			if (navigator.vibrate) navigator.vibrate([50]);
		}
	}

	async function confirmReturn() {
		if (scannedBooks.length === 0 || isSubmitting) return;
		isSubmitting = true;
		errorMessage = null;

		try {
			const bookIds = scannedBooks.map((b) => b.id);
			const res = await pb.send<{ success: boolean; count: number; returnedBookIds: string[] }>(
				'/api/cashier/return-books',
				{
					method: 'POST',
					body: { bookIds }
				}
			);

			if (res && res.success) {
				if (onsuccess) {
					onsuccess(res.returnedBookIds || bookIds);
				}
				open = false;
			}
		} catch (err: any) {
			console.error('Return books error:', err);
			errorMessage = err?.message || 'Chyba při označování knih jako vrácených.';
		} finally {
			isSubmitting = false;
		}
	}
</script>

{#if open}
	<div
		class="fixed inset-0 z-50 bg-black flex flex-col select-none overflow-hidden"
		role="dialog"
		aria-modal="true"
		aria-label="Skener pro vracení knih"
	>
		<!-- Top Bar -->
		<div class="relative z-20 flex items-center justify-between p-3 bg-neutral-900 text-white border-b border-neutral-800">
			<div class="min-w-0 pr-2">
				<div class="flex items-center gap-1.5 text-xs font-black uppercase tracking-wider text-neutral-400">
					<span>VRÁTIT KNIHY</span>
					<span>•</span>
					<span class="text-white truncate">{seller.name || seller.email}</span>
				</div>
				<p class="text-[11px] text-neutral-400 font-mono truncate">
					{scannedBooks.length} / {unsoldBooks.length} vybráno k vrácení
				</p>
			</div>

			<div class="flex items-center gap-2 shrink-0">
				<button
					type="button"
					onclick={() => (isSearchOpen = true)}
					class="p-2 border border-neutral-700 bg-neutral-800 hover:bg-neutral-700 active:scale-95 text-white font-bold text-xs uppercase flex items-center gap-1.5 cursor-pointer"
					title="Hledat podle ID"
				>
					<Search class="w-4 h-4" />
					<span class="hidden sm:inline">Hledat kód</span>
				</button>

				<button
					type="button"
					onclick={() => (open = false)}
					class="p-2 border border-neutral-700 bg-neutral-800 hover:bg-neutral-700 active:scale-95 text-white cursor-pointer"
					title="Zavřít"
				>
					<X class="w-4 h-4" />
				</button>
			</div>
		</div>

		<!-- Camera & AR Overlay Container -->
		<div class="relative flex-1 bg-black overflow-hidden flex items-center justify-center touch-none">
			<video
				bind:this={videoElement}
				playsinline
				autoplay
				muted
				class="w-full h-full object-cover"
			></video>

			<canvas bind:this={captureCanvas} class="hidden"></canvas>
			<canvas bind:this={overlayCanvas} class="absolute inset-0 w-full h-full pointer-events-none z-10"></canvas>

			<!-- Crosshair / Scan hint -->
			{#if !cameraError}
				<div class="absolute inset-0 pointer-events-none flex flex-col items-center justify-center p-4">
					<div class="w-64 h-64 border-2 border-dashed border-white/40 rounded-lg flex items-center justify-center">
						<span class="text-[11px] font-black uppercase tracking-wider text-white/70 bg-black/50 px-2 py-1">
							NAMIŘTE NA NÁLEPKU
						</span>
					</div>
				</div>
			{:else}
				<div class="absolute inset-0 bg-white flex flex-col items-center justify-center p-6 text-center z-30 pointer-events-auto">
					<div class="p-3 bg-red-100 text-red-700 border-2 border-red-700 mb-3">
						<AlertTriangle class="w-8 h-8" />
					</div>
					<h3 class="text-sm font-black uppercase text-black mb-1">Kamera není dostupná</h3>
					<p class="text-xs text-neutral-600 mb-4 max-w-xs">{cameraError}</p>
					<div class="flex gap-2">
						<button
							type="button"
							onclick={startCamera}
							class="px-3 py-2 bg-neutral-100 hover:bg-neutral-200 border-2 border-black font-bold text-xs uppercase cursor-pointer"
						>
							Zkusit znovu
						</button>
						<button
							type="button"
							onclick={() => (isSearchOpen = true)}
							class="px-3 py-2 bg-yellow-400 hover:bg-yellow-500 border-2 border-black font-black text-xs uppercase cursor-pointer"
						>
							Vyhledat ručně
						</button>
					</div>
				</div>
			{/if}
		</div>

		<!-- Bottom Tray: Scanned books to return -->
		<div class="relative z-20 bg-white border-t-2 border-black max-h-[45vh] flex flex-col touch-pan-y">
			<!-- Tray Header -->
			<div class="p-2.5 bg-neutral-100 border-b border-neutral-300 flex items-center justify-between gap-2 shrink-0">
				<div class="flex items-center gap-2 min-w-0">
					<span class="text-xs font-black uppercase tracking-wider text-black">
						K VRÁCENÍ ({scannedBooks.length} ks)
					</span>
				</div>

				<div class="flex items-center gap-2">
					{#if unsoldBooks.length > scannedBooks.length}
						<button
							type="button"
							onclick={addAllRemaining}
							class="px-2 py-1 text-[10px] font-black uppercase tracking-wider bg-white border border-black hover:bg-neutral-200 active:scale-95 cursor-pointer"
						>
							Vrátit všechny zbývající ({unsoldBooks.length - scannedBooks.length})
						</button>
					{/if}
				</div>
			</div>

			<!-- Scanned Books List with Swipe-Right-to-Remove -->
			<div class="overflow-y-auto p-2.5 space-y-2 flex-1 touch-pan-y">
				{#if scannedBooks.length === 0}
					<div class="p-6 text-center text-neutral-400 font-bold text-xs uppercase tracking-wide">
						Naskenujte nálepku učebnice nebo vyhledejte kód ručně
					</div>
				{:else}
					{#each scannedBooks as book (book.id)}
						{@const col = idToColor(book.id)}
						{@const isSwiping = swipingItemId === book.id}
						{@const isRemoving = removingItemIds.has(book.id)}

						<div
							class="relative overflow-hidden border-2 border-black bg-white select-none transition-all duration-200 ease-out {isRemoving
								? 'opacity-0 translate-x-full max-h-0 !my-0 !py-0 !border-0'
								: 'max-h-28'}"
						>
							<!-- Red track revealed on swipe right -->
							<div
								class="absolute inset-0 bg-red-600 flex items-center px-4 gap-2 text-white font-black text-xs uppercase transition-opacity duration-100"
								style="opacity: {isSwiping ? Math.min(1, Math.max(0, swipeDeltaX / 50)) : 0};"
							>
								<Trash2 class="w-4 h-4 shrink-0" />
								<span>ODSTRANIT Z VRACENÍ</span>
							</div>

							<!-- Foreground sliding row -->
							<div
								role="presentation"
								class="relative bg-white p-2.5 flex items-center justify-between gap-3 cursor-grab active:cursor-grabbing touch-pan-y {isSwiping
									? ''
									: 'transition-transform duration-200 ease-out'}"
								style="transform: translateX({isSwiping ? swipeDeltaX : 0}px);"
								onpointerdown={(e) => handleItemPointerDown(e, book.id)}
								onpointermove={(e) => handleItemPointerMove(e, book.id)}
								onpointerup={(e) => handleItemPointerUp(e, book.id)}
								onpointercancel={(e) => handleItemPointerUp(e, book.id)}
							>
								<div class="flex items-center gap-3 min-w-0">
									<!-- Color stripe badge -->
									<div class="w-3.5 h-12 border border-black shrink-0" style="background-color: {col.bg};"></div>

									<!-- Thumbnail -->
									<div class="relative w-10 h-12 border border-black bg-neutral-100 overflow-hidden shrink-0">
										{#if book.photo}
											<img src={getBookThumbnailUrl(book)} alt={book.id} class="w-full h-full object-cover" />
										{/if}
									</div>

									<!-- Details -->
									<div class="min-w-0">
										<span
											class="px-1.5 py-0.2 border text-[10px] font-black uppercase text-white"
											style="background-color: {col.bg}; border-color: {col.border};"
										>
											{book.id}
										</span>
										<div class="text-xs font-black text-black mt-1">
											{book.price} Kč
										</div>
									</div>
								</div>

								<!-- Remove button fallback -->
								<button
									type="button"
									onclick={() => triggerRemoveBook(book.id)}
									class="p-2 text-neutral-400 hover:text-red-600 active:scale-95 cursor-pointer"
									title="Odstranit"
								>
									<Trash2 class="w-4 h-4" />
								</button>
							</div>
						</div>
					{/each}
				{/if}
			</div>

			<!-- Action Confirm Button -->
			<div class="p-3 bg-neutral-50 border-t-2 border-black flex items-center gap-2 shrink-0">
				<button
					type="button"
					onclick={() => (isConfirmOpen = true)}
					disabled={scannedBooks.length === 0}
					class="w-full py-3 px-4 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-40 disabled:cursor-not-allowed text-white font-black text-xs uppercase tracking-wider border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:scale-[0.99] transition-all flex items-center justify-center gap-2 cursor-pointer"
				>
					<Check class="w-4 h-4" />
					<span>OZNAČIT JAKO VRÁCENÉ ({scannedBooks.length} KS)</span>
				</button>
			</div>
		</div>

		<!-- Search / Autocomplete Modal -->
		{#if isSearchOpen}
			<div
				class="fixed inset-0 z-40 bg-black/70 flex items-center justify-center p-4"
				onclick={(e) => {
					if (e.target === e.currentTarget) isSearchOpen = false;
				}}
				onkeydown={(e) => {
					if (e.key === 'Escape') isSearchOpen = false;
				}}
				role="dialog"
				tabindex="-1"
				aria-modal="true"
				aria-label="Hledat kód knihy"
			>
				<div
					class="bg-white border-2 border-black w-full max-w-md max-h-[80vh] flex flex-col shadow-[4px_4px_0px_0px_rgba(0,0,0,1)]"
				>
					<div class="p-3 bg-neutral-100 border-b-2 border-black flex items-center justify-between">
						<span class="text-xs font-black uppercase tracking-wider text-black">HLEDAT KÓD KNIHY</span>
						<button
							type="button"
							onclick={() => (isSearchOpen = false)}
							class="p-1 border border-black hover:bg-neutral-200"
						>
							<X class="w-4 h-4" />
						</button>
					</div>

					<div class="p-3 border-b border-neutral-300">
						<input
							type="text"
							placeholder="Zadejte kód..."
							bind:value={searchQuery}
							class="w-full px-3 py-2 border-2 border-black text-xs font-mono font-bold focus:outline-none focus:bg-yellow-50"
						/>
					</div>

					<div class="overflow-y-auto p-2 space-y-1.5 flex-1 max-h-72">
						{#if availableForSearch.length === 0}
							<p class="text-center text-xs text-neutral-400 font-bold p-4 uppercase">
								Žádné zbývající neprodané knihy
							</p>
						{:else}
							{#each availableForSearch as book}
								{@const col = idToColor(book.id)}
								<button
									type="button"
									onclick={() => addBookManually(book)}
									class="w-full p-2 border border-black bg-white hover:bg-neutral-100 flex items-center justify-between cursor-pointer text-left"
								>
									<div class="flex items-center gap-2">
										<div class="w-2.5 h-8 border border-black" style="background-color: {col.bg};"></div>
										<div>
											<span class="font-mono text-xs font-bold">{book.id}</span>
											<p class="text-[11px] font-bold text-neutral-500">{book.price} Kč</p>
										</div>
									</div>
									<span class="text-[10px] font-black uppercase px-2 py-1 bg-neutral-200 border border-black">
										+ PŘIDAT
									</span>
								</button>
							{/each}
						{/if}
					</div>
				</div>
			</div>
		{/if}

		<!-- Final Confirmation Dialog -->
		{#if isConfirmOpen}
			<div
				class="fixed inset-0 z-50 bg-black/80 flex items-center justify-center p-4"
				onclick={(e) => {
					if (e.target === e.currentTarget) isConfirmOpen = false;
				}}
				onkeydown={(e) => {
					if (e.key === 'Escape') isConfirmOpen = false;
				}}
				role="dialog"
				tabindex="-1"
				aria-modal="true"
				aria-label="Potvrzení vrácení knih"
			>
				<div
					class="bg-white border-4 border-black w-full max-w-sm p-4 shadow-[6px_6px_0px_0px_rgba(0,0,0,1)]"
				>
					<div class="flex items-center gap-2 text-red-600 mb-2 font-black text-sm uppercase">
						<AlertTriangle class="w-5 h-5" />
						<span>POTVRZENÍ VRÁCENÍ KNIH</span>
					</div>

					<p class="text-xs font-medium text-neutral-700 mb-4">
						Opravdu chcete označit <strong class="text-black font-black">{scannedBooks.length} ks</strong>
						učebnic jako <strong class="text-red-700 font-black">VRÁCENÉ</strong> prodejci
						<strong class="text-black font-black">{seller.name || seller.email}</strong>?
					</p>

					{#if errorMessage}
						<div class="p-2 bg-red-100 border border-red-500 text-red-700 text-xs font-bold mb-3">
							{errorMessage}
						</div>
					{/if}

					<div class="flex gap-2">
						<button
							type="button"
							onclick={() => (isConfirmOpen = false)}
							disabled={isSubmitting}
							class="flex-1 py-2.5 bg-neutral-200 hover:bg-neutral-300 font-bold text-xs uppercase border-2 border-black cursor-pointer"
						>
							Zrušit
						</button>
						<button
							type="button"
							onclick={confirmReturn}
							disabled={isSubmitting}
							class="flex-1 py-2.5 bg-red-600 hover:bg-red-700 text-white font-black text-xs uppercase border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:scale-95 cursor-pointer flex items-center justify-center gap-1.5"
						>
							{#if isSubmitting}
								<RefreshCw class="w-3.5 h-3.5 animate-spin" />
								<span>Ukládám...</span>
							{:else}
								<Check class="w-3.5 h-3.5" />
								<span>Potvrdit vrácení</span>
							{/if}
						</button>
					</div>
				</div>
			</div>
		{/if}
	</div>
{/if}
