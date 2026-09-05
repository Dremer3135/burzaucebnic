<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { pb, getBookThumbnailUrl, getBookFullImageUrl } from '$lib/pocketbase';
	import {
		CAMERA_CONSTRAINTS,
		getVideoTransform,
		scanFrameForAllDataMatrices,
		drawPricePolygon,
		idToColor,
		type ScanMatch
	} from '$lib/scanner';
	import {
		ScanLine,
		List,
		Search,
		Check,
		X,
		AlertCircle,
		RefreshCw,
		BookOpen,
		Eye
	} from '@lucide/svelte';
	import type { Book } from '$lib/types';

	// View mode: 'scan' (camera intake) or 'list' (overview list)
	let activeTab = $state<'scan' | 'list'>('scan');

	// List filters: 'all' | 'unaccepted' | 'accepted'
	let listFilter = $state<'unaccepted' | 'accepted' | 'all'>('unaccepted');
	let searchQuery = $state('');

	// Data
	let books = $state<Book[]>([]);
	let isLoading = $state(true);
	let actionLoadingId = $state<string | null>(null);
	let errorMessage = $state('');
	let toastMessage = $state('');
	let toastTimeout: any = null;

	// Photo modal preview
	let selectedPhotoBook = $state<Book | null>(null);

	// Camera & Scanner state (non-reactive for stream & loop to prevent reactive loops)
	let videoElement = $state<HTMLVideoElement | null>(null);
	let captureCanvas = $state<HTMLCanvasElement | null>(null);
	let overlayCanvas = $state<HTMLCanvasElement | null>(null);
	let mediaStream: MediaStream | null = null;
	let isScanningLoopActive = false;
	let renderAnimId: number | null = null;
	let cameraError = $state<string | null>(null);
	let isCameraReady = $state(false);

	// Multi-detection persistence map to prevent flicker
	interface TrackedMatch {
		match: ScanMatch;
		lastSeen: number;
	}
	const trackedMatches = new Map<string, TrackedMatch>();
	const codeLookupCache = new Map<string, any>();
	const inFlightLookups = new Set<string>();

	// All books currently seen in the camera frame
	let currentlySeenBooks = $state<Book[]>([]);

	// Statistics for list filters
	let totalCount = $derived(books.length);
	let unacceptedCount = $derived(books.filter((b) => !b.accepted).length);
	let acceptedCount = $derived(books.filter((b) => b.accepted).length);

	// Filtered books for list view
	let filteredBooks = $derived(
		books.filter((b) => {
			if (listFilter === 'unaccepted' && b.accepted) return false;
			if (listFilter === 'accepted' && !b.accepted) return false;

			if (!searchQuery.trim()) return true;
			const q = searchQuery.toLowerCase().trim();
			const matchId = b.id.toLowerCase().includes(q);
			const matchPrice = String(b.price).includes(q);
			const matchSellerName = b.expand?.seller?.name?.toLowerCase().includes(q);
			const matchSellerEmail = b.expand?.seller?.email?.toLowerCase().includes(q);
			return matchId || matchPrice || matchSellerName || matchSellerEmail;
		})
	);

	function showToast(msg: string) {
		toastMessage = msg;
		if (toastTimeout) clearTimeout(toastTimeout);
		toastTimeout = setTimeout(() => {
			toastMessage = '';
		}, 3000);
	}

	// ----------------------------------------------------
	// TAB SWITCHING
	// ----------------------------------------------------
	function switchTab(newTab: 'scan' | 'list') {
		if (activeTab === newTab) return;
		activeTab = newTab;
		if (newTab === 'scan') {
			startCamera();
		} else {
			stopCamera();
		}
	}

	// ----------------------------------------------------
	// LIFECYCLE & DATA LOADING
	// ----------------------------------------------------
	onMount(() => {
		loadBooks();
		setupRealtimeSubscription();
		startCamera();
	});

	onDestroy(() => {
		stopCamera();
		pb.collection('books').unsubscribe('*').catch(console.error);
		if (toastTimeout) clearTimeout(toastTimeout);
	});

	async function loadBooks() {
		isLoading = true;
		errorMessage = '';
		try {
			const res = await pb.collection('books').getFullList<Book>({
				sort: '-created',
				expand: 'seller,buyer'
			});
			books = res;
			// Prepopulate lookup cache
			for (const b of res) {
				codeLookupCache.set(b.id, {
					type: 'book',
					book: b
				});
			}
		} catch (err: any) {
			console.error('Failed to load books for acceptance:', err);
			errorMessage = 'Nepodařilo se načíst knihy.';
		} finally {
			isLoading = false;
		}
	}

	function setupRealtimeSubscription() {
		pb.collection('books')
			.subscribe<Book>('*', (e) => {
				if (e.action === 'create') {
					books = [e.record, ...books.filter((b) => b.id !== e.record.id)];
					codeLookupCache.set(e.record.id, { type: 'book', book: e.record });
				} else if (e.action === 'update') {
					books = books.map((b) => (b.id === e.record.id ? { ...b, ...e.record } : b));
					const cached = codeLookupCache.get(e.record.id);
					if (cached?.type === 'book') {
						codeLookupCache.set(e.record.id, { ...cached, book: { ...cached.book, ...e.record } });
					}
					currentlySeenBooks = currentlySeenBooks.map((b) =>
						b.id === e.record.id ? { ...b, ...e.record } : b
					);
				} else if (e.action === 'delete') {
					books = books.filter((b) => b.id !== e.record.id);
					codeLookupCache.delete(e.record.id);
					trackedMatches.delete(e.record.id);
					currentlySeenBooks = currentlySeenBooks.filter((b) => b.id !== e.record.id);
				}
			}, { expand: 'seller,buyer' })
			.catch((err) => console.error('Books realtime subscribe error:', err));
	}

	// ----------------------------------------------------
	// CAMERA & SCANNER ENGINE
	// ----------------------------------------------------
	async function startCamera() {
		stopCamera();
		cameraError = null;
		isCameraReady = false;

		try {
			let stream: MediaStream;
			try {
				stream = await navigator.mediaDevices.getUserMedia(CAMERA_CONSTRAINTS);
			} catch (firstErr) {
				console.warn('Initial camera constraints failed, attempting fallback...', firstErr);
				stream = await navigator.mediaDevices.getUserMedia({
					video: { facingMode: 'environment' },
					audio: false
				});
			}

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
			cameraError = 'Nelze přistoupit ke kameře. Povolte prosím přístup ke kameře v prohlížeči.';
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
		currentlySeenBooks = [];
	}

	let isDetecting = false;
	let lastDetectTime = 0;

	async function runDetectionLoop() {
		if (!isScanningLoopActive) return;

		const now = performance.now();
		if (!isDetecting && now - lastDetectTime >= 40) {
			isDetecting = true;
			lastDetectTime = now;

			try {
				if (captureCanvas && videoElement && videoElement.videoWidth > 0) {
					const matches = await scanFrameForAllDataMatrices(captureCanvas, videoElement, 8);
					const currentTime = Date.now();

					// Record detected matches
					for (const match of matches) {
						const code = match.text.trim();
						trackedMatches.set(code, {
							match,
							lastSeen: currentTime
						});
						handleScannedCode(code);
					}

					// Prune matches not seen for 1000ms
					for (const [code, tracked] of trackedMatches.entries()) {
						if (currentTime - tracked.lastSeen > 1000) {
							trackedMatches.delete(code);
						}
					}

					updateCurrentlySeenBooks();
				}
			} catch (err) {
				console.error('Intake detection error:', err);
			} finally {
				isDetecting = false;
			}
		}

		if (isScanningLoopActive) {
			setTimeout(runDetectionLoop, 25);
		}
	}

	function updateCurrentlySeenBooks() {
		const seen: Book[] = [];
		for (const [code] of trackedMatches.entries()) {
			const cached = codeLookupCache.get(code);
			if (cached?.type === 'book') {
				seen.push(cached.book);
			}
		}

		// Stable sort by ID so cards never change order when accepted or rejected
		seen.sort((a, b) => a.id.localeCompare(b.id));

		const isDifferent =
			seen.length !== currentlySeenBooks.length ||
			seen.some((b, i) => b.id !== currentlySeenBooks[i].id || b.accepted !== currentlySeenBooks[i].accepted);

		if (isDifferent) {
			currentlySeenBooks = seen;
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

				for (const { match } of trackedMatches.values()) {
					const code = match.text.trim();
					const cached = codeLookupCache.get(code);

					if (cached?.type === 'book') {
						const book: Book = cached.book;
						const color = idToColor(book.id);
						drawPricePolygon(ctx, match.position, `${book.price} Kč`, transform, color);
					}
				}

				ctx.restore();
			}
		}

		renderAnimId = requestAnimationFrame(runRenderLoop);
	}

	async function handleScannedCode(code: string) {
		let cached = codeLookupCache.get(code);
		if (!cached) {
			if (inFlightLookups.has(code)) return;
			inFlightLookups.add(code);

			try {
				const info = await pb.send<any>(`/api/cashier/lookup-code?code=${encodeURIComponent(code)}`, {
					method: 'GET'
				});
				codeLookupCache.set(code, info);
				cached = info;
			} catch (err) {
				console.error('Lookup code error on intake:', err);
				return;
			} finally {
				inFlightLookups.delete(code);
			}
		}

		if (cached?.type === 'book') {
			updateCurrentlySeenBooks();
		}
	}

	// ----------------------------------------------------
	// ACCEPT / REVOKE ACTION
	// ----------------------------------------------------
	async function toggleBookAccepted(book: Book, targetStatus?: boolean) {
		actionLoadingId = book.id;
		errorMessage = '';

		const newStatus = targetStatus !== undefined ? targetStatus : !book.accepted;

		try {
			const res = await pb.send<any>('/api/cashier/toggle-book-accepted', {
				method: 'POST',
				body: {
					bookId: book.id,
					accepted: newStatus
				}
			});

			const updatedAccepted = res.accepted ?? newStatus;

			// Update books in local state
			books = books.map((b) => (b.id === book.id ? { ...b, accepted: updatedAccepted } : b));

			// Update cache
			const cached = codeLookupCache.get(book.id);
			if (cached?.type === 'book') {
				cached.book.accepted = updatedAccepted;
			}

			// Update currently seen books
			currentlySeenBooks = currentlySeenBooks.map((b) =>
				b.id === book.id ? { ...b, accepted: updatedAccepted } : b
			);

			if (navigator.vibrate) {
				navigator.vibrate(updatedAccepted ? [60] : [30, 30]);
			}

			showToast(updatedAccepted ? `Kniha #${book.id} byla přijata k prodeji.` : `Přijetí knihy #${book.id} bylo zrušeno.`);
		} catch (err: any) {
			console.error('Failed to toggle book acceptance:', err);
			errorMessage = err?.message || 'Chyba při změně stavu knihy.';
		} finally {
			actionLoadingId = null;
		}
	}
</script>

<div class="flex-1 w-full h-full bg-neutral-100 text-black overflow-hidden relative flex flex-col select-none">
	<!-- Top Navigation Bar -->
	<div class="bg-white border-b-2 border-black px-3 py-2 shrink-0 z-30">
		<div class="max-w-4xl mx-auto flex items-center justify-between">
			<!-- Mode switch: SKENER vs SEZNAM -->
			<div class="flex border-2 border-black bg-white p-0.5 text-xs font-black uppercase w-full sm:w-auto">
				<button
					onclick={() => switchTab('scan')}
					class="flex-1 sm:flex-initial flex items-center justify-center gap-1.5 px-4 py-1.5 transition-all cursor-pointer {activeTab === 'scan'
						? 'bg-black text-white'
						: 'text-black hover:bg-neutral-100'}"
				>
					<ScanLine class="w-3.5 h-3.5" />
					<span>SKENER PŘÍJMU</span>
				</button>
				<button
					onclick={() => switchTab('list')}
					class="flex-1 sm:flex-initial flex items-center justify-center gap-1.5 px-4 py-1.5 transition-all cursor-pointer {activeTab === 'list'
						? 'bg-black text-white'
						: 'text-black hover:bg-neutral-100'}"
				>
					<List class="w-3.5 h-3.5" />
					<span>SEZNAM KNIH</span>
				</button>
			</div>
		</div>
	</div>

	<!-- Global error / toast notification -->
	{#if errorMessage}
		<div class="bg-red-600 text-white px-3 py-1.5 text-xs font-bold flex items-center justify-between z-30 shrink-0 border-b border-black">
			<div class="flex items-center gap-2">
				<AlertCircle class="w-4 h-4 shrink-0" />
				<span>{errorMessage}</span>
			</div>
			<button onclick={() => (errorMessage = '')} class="p-0.5 hover:bg-red-700 cursor-pointer">
				<X class="w-4 h-4" />
			</button>
		</div>
	{/if}

	{#if toastMessage}
		<div class="absolute top-16 left-1/2 -translate-x-1/2 z-50 bg-black text-white px-4 py-2 border-2 border-white shadow-[3px_3px_0px_0px_rgba(0,0,0,1)] text-xs font-black uppercase tracking-wider flex items-center gap-2">
			<Check class="w-4 h-4 text-emerald-400" />
			<span>{toastMessage}</span>
		</div>
	{/if}

	<!-- ==================================================== -->
	<!-- CAMERA SCANNER VIEWPORT (Intake Camera Mode) -->
	<!-- ==================================================== -->
	<div class="flex-1 relative w-full h-full bg-black overflow-hidden flex flex-col {activeTab === 'scan' ? '' : 'hidden'}">
		<!-- Hidden canvas for capturing frame to pass to zxing-wasm -->
		<canvas bind:this={captureCanvas} class="hidden"></canvas>

		<!-- Camera Video -->
		<!-- svelte-ignore a11y_media_has_caption -->
		<video
			bind:this={videoElement}
			class="absolute inset-0 w-full h-full object-cover"
			playsinline
			muted
			autoplay
		></video>

		<!-- AR Overlay Canvas -->
		<canvas
			bind:this={overlayCanvas}
			class="absolute inset-0 w-full h-full pointer-events-none z-10"
		></canvas>

		<!-- Camera Error Display -->
		{#if cameraError}
			<div class="absolute inset-0 z-20 flex items-center justify-center p-6 bg-black/85 text-white">
				<div class="max-w-md bg-white text-black border-2 border-black p-4 shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] text-center space-y-3">
					<AlertCircle class="w-8 h-8 mx-auto text-red-600" />
					<p class="text-xs font-bold uppercase">{cameraError}</p>
					<button
						onclick={startCamera}
						class="w-full py-2 bg-black text-white text-xs font-black uppercase border-2 border-black hover:bg-neutral-800 cursor-pointer"
					>
						ZKUSIT ZNOVU
					</button>
				</div>
			</div>
		{/if}

		<!-- Bottom Floating Widget for All Currently Seen Books (Max height ~50vh, scrollable) -->
		{#if currentlySeenBooks.length > 0}
			<div class="absolute bottom-3 inset-x-3 sm:max-w-lg sm:mx-auto z-20 pointer-events-auto max-h-[50vh] flex flex-col">
				<!-- Scrollable Container -->
				<div class="overflow-y-auto space-y-2 max-h-[50vh] pr-0.5" style="overscroll-behavior: contain;">
					{#each currentlySeenBooks as book (book.id)}
						{@const col = idToColor(book.id)}
						<div class="bg-white border-2 border-black shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] p-2.5 sm:p-3 flex items-center justify-between gap-3 relative overflow-hidden">
							<!-- Left: Hash Color Stripe -->
							<div
								class="w-3 self-stretch -my-2.5 -ml-2.5 sm:-my-3 sm:-ml-3 border-r-2 border-black shrink-0"
								style="background-color: {col.bg};"
							></div>

							<!-- Thumbnail photo -->
							{#if book.photo}
								<button
									type="button"
									onclick={() => (selectedPhotoBook = book)}
									class="relative w-12 h-16 bg-neutral-100 border-2 border-black shrink-0 overflow-hidden group cursor-pointer"
									title="Zvětšit foto"
								>
									<img
										src={getBookThumbnailUrl(book)}
										alt="Kniha"
										class="w-full h-full object-cover group-hover:scale-105 transition-transform"
									/>
									<div class="absolute inset-0 bg-black/30 opacity-0 group-hover:opacity-100 flex items-center justify-center transition-opacity">
										<Eye class="w-3.5 h-3.5 text-white" />
									</div>
								</button>
							{:else}
								<div class="w-12 h-16 bg-neutral-100 border-2 border-black shrink-0 flex items-center justify-center text-neutral-400">
									<BookOpen class="w-5 h-5" />
								</div>
							{/if}

							<!-- Middle details: Price, ID, Seller info -->
							<div class="min-w-0 flex-1 flex flex-col justify-center">
								<div class="flex items-baseline justify-between gap-2">
									<span class="text-base sm:text-lg font-black text-black leading-none">
										{book.price} Kč
									</span>
									<span
										class="text-[10px] font-mono font-black px-1.5 py-0.5 border border-black truncate max-w-[120px]"
										style="background-color: {col.lightBg}; color: {col.border};"
									>
										#{book.id}
									</span>
								</div>

								<div class="mt-1 text-xs font-black uppercase text-black truncate leading-tight">
									{book.expand?.seller?.name || 'Prodejce'}
								</div>
								{#if book.expand?.seller?.email}
									<div class="mt-0.5 text-[11px] font-mono text-neutral-500 truncate leading-tight">
										{book.expand?.seller?.email}
									</div>
								{/if}
							</div>

							<!-- Right: Big Action Button -->
							<div class="shrink-0 flex items-center">
								{#if !book.accepted}
									<button
										type="button"
										disabled={actionLoadingId === book.id}
										onclick={() => toggleBookAccepted(book, true)}
										class="h-11 px-4 bg-emerald-600 hover:bg-emerald-700 text-white font-black text-xs uppercase tracking-wider border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:translate-x-0.5 active:translate-y-0.5 active:shadow-none transition-all flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-60 min-w-[110px]"
									>
										{#if actionLoadingId === book.id}
											<RefreshCw class="w-4 h-4 animate-spin" />
										{:else}
											<Check class="w-4 h-4 stroke-[3]" />
											<span>PŘIJMOUT</span>
										{/if}
									</button>
								{:else}
									<button
										type="button"
										disabled={actionLoadingId === book.id}
										onclick={() => toggleBookAccepted(book, false)}
										class="h-11 px-3 bg-white hover:bg-red-50 hover:text-red-700 hover:border-red-600 text-neutral-600 font-black text-xs uppercase tracking-wider border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:translate-x-0.5 active:translate-y-0.5 active:shadow-none transition-all flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-60 min-w-[110px]"
									>
										{#if actionLoadingId === book.id}
											<RefreshCw class="w-3.5 h-3.5 animate-spin" />
										{:else}
											<X class="w-4 h-4 stroke-[2.5]" />
											<span>ZRUŠIT</span>
										{/if}
									</button>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	<!-- ==================================================== -->
	<!-- OVERVIEW LIST VIEWPORT (Only shown when activeTab === 'list') -->
	<!-- ==================================================== -->
	{#if activeTab === 'list'}
		<div class="flex-1 max-w-4xl w-full mx-auto p-3 sm:p-4 flex flex-col overflow-y-auto">
			<!-- Filter & Search Controls -->
			<div class="flex flex-col sm:flex-row gap-2 items-stretch sm:items-center justify-between mb-3">
				<!-- Filter Pills -->
				<div class="flex border-2 border-black bg-white p-0.5 text-xs font-black">
					<button
						onclick={() => (listFilter = 'unaccepted')}
						class="flex-1 sm:flex-initial px-3 py-1.5 uppercase transition-all cursor-pointer {listFilter === 'unaccepted'
							? 'bg-amber-500 text-black'
							: 'text-black hover:bg-neutral-100'}"
					>
						<span>K PŘIJETÍ</span>
						{#if unacceptedCount > 0}
							<span class="ml-1 px-1.5 py-0.2 bg-black text-white text-[10px]">
								{unacceptedCount}
							</span>
						{/if}
					</button>
					<button
						onclick={() => (listFilter = 'accepted')}
						class="flex-1 sm:flex-initial px-3 py-1.5 uppercase transition-all cursor-pointer {listFilter === 'accepted'
							? 'bg-emerald-600 text-white'
							: 'text-black hover:bg-neutral-100'}"
					>
						<span>PŘIJATÉ</span>
						{#if acceptedCount > 0}
							<span class="ml-1 px-1.5 py-0.2 {listFilter === 'accepted' ? 'bg-white text-black' : 'bg-black text-white'} text-[10px]">
								{acceptedCount}
							</span>
						{/if}
					</button>
					<button
						onclick={() => (listFilter = 'all')}
						class="flex-1 sm:flex-initial px-3 py-1.5 uppercase transition-all cursor-pointer {listFilter === 'all'
							? 'bg-black text-white'
							: 'text-black hover:bg-neutral-100'}"
					>
						<span>VŠECHNY</span>
						{#if totalCount > 0}
							<span class="ml-1 px-1.5 py-0.2 {listFilter === 'all' ? 'bg-white text-black' : 'bg-black text-white'} text-[10px]">
								{totalCount}
							</span>
						{/if}
					</button>
				</div>

				<!-- Search Box -->
				<div class="relative flex-1 sm:max-w-xs">
					<Search class="w-3.5 h-3.5 text-black absolute left-2.5 top-2.5" />
					<input
						type="text"
						bind:value={searchQuery}
						placeholder="Hledat ID, prodejce, cenu..."
						class="w-full bg-white border-2 border-black pl-8 pr-7 py-1.5 text-xs font-black uppercase text-black focus:outline-none"
					/>
					{#if searchQuery}
						<button
							onclick={() => (searchQuery = '')}
							class="absolute right-2 top-2 text-neutral-400 hover:text-black cursor-pointer"
						>
							<X class="w-3.5 h-3.5" />
						</button>
					{/if}
				</div>
			</div>

			<!-- Loading State -->
			{#if isLoading && books.length === 0}
				<div class="flex-1 flex items-center justify-center py-16">
					<RefreshCw class="w-8 h-8 animate-spin text-black" />
				</div>
			{:else if filteredBooks.length === 0}
				<div class="flex-1 flex flex-col items-center justify-center py-12 text-center">
					<BookOpen class="w-8 h-8 text-neutral-300 mb-2" />
					<p class="text-xs font-bold uppercase text-neutral-400">Žádné knihy neodpovídají filtru</p>
				</div>
			{:else}
				<!-- Books Grid / List -->
				<div class="space-y-2.5 pb-16">
					{#each filteredBooks as book (book.id)}
						<div class="bg-white border-2 border-black p-3 text-black transition-all flex flex-col sm:flex-row sm:items-center justify-between gap-3">
							<!-- Left: Thumbnail & Details -->
							<div class="flex items-center gap-3 min-w-0">
								{#if book.photo}
									<button
										type="button"
										onclick={() => (selectedPhotoBook = book)}
										class="relative w-12 h-16 bg-neutral-100 border-2 border-black shrink-0 overflow-hidden group cursor-pointer"
										title="Zvětšit foto"
									>
										<img
											src={getBookThumbnailUrl(book)}
											alt="Kniha"
											class="w-full h-full object-cover group-hover:scale-105 transition-transform"
										/>
										<div class="absolute inset-0 bg-black/30 opacity-0 group-hover:opacity-100 flex items-center justify-center transition-opacity">
											<Eye class="w-3.5 h-3.5 text-white" />
										</div>
									</button>
								{:else}
									<div class="w-12 h-16 bg-neutral-100 border-2 border-black shrink-0 flex items-center justify-center text-neutral-400">
										<BookOpen class="w-5 h-5" />
									</div>
								{/if}

								<div class="min-w-0">
									<div class="flex items-center gap-2">
										<span class="text-xs font-mono font-bold text-neutral-600">
											#{book.id}
										</span>
										<span class="text-base font-black text-black">
											{book.price} Kč
										</span>
									</div>

									<div class="text-xs font-black uppercase text-black truncate">
										{book.expand?.seller?.name || 'Prodejce'}
									</div>
									<div class="text-[11px] font-mono text-neutral-500 truncate">
										{book.expand?.seller?.email || ''}
									</div>

									<div class="mt-1 flex items-center gap-2 text-[10px]">
										{#if book.accepted}
											<span class="font-black uppercase text-emerald-700 bg-emerald-50 border border-emerald-600 px-1.5 py-0.2">
												✓ PŘIJATO
											</span>
										{:else}
											<span class="font-black uppercase text-amber-700 bg-amber-50 border border-amber-600 px-1.5 py-0.2">
												✗ K PŘIJETÍ
											</span>
										{/if}

										<span class="text-neutral-400 uppercase">
											{book.status === 'available' ? 'Dostupná' : book.status === 'checkout' ? 'V pokladně' : 'Prodáno'}
										</span>
									</div>
								</div>
							</div>

							<!-- Right: Action Button -->
							<div class="shrink-0 flex sm:flex-col items-center sm:items-end justify-end gap-2 border-t sm:border-t-0 pt-2 sm:pt-0 border-neutral-100">
								{#if !book.accepted}
									<button
										type="button"
										disabled={actionLoadingId === book.id}
										onclick={() => toggleBookAccepted(book, true)}
										class="w-full sm:w-auto px-4 py-2 bg-emerald-600 hover:bg-emerald-700 text-white font-black text-xs uppercase tracking-wider border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:translate-x-0.5 active:translate-y-0.5 active:shadow-none transition-all flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-60"
									>
										{#if actionLoadingId === book.id}
											<RefreshCw class="w-3.5 h-3.5 animate-spin" />
											<span>UKLÁDÁM...</span>
										{:else}
											<Check class="w-4 h-4 stroke-[3]" />
											<span>PŘIJMOUT</span>
										{/if}
									</button>
								{:else}
									<button
										type="button"
										disabled={actionLoadingId === book.id}
										onclick={() => toggleBookAccepted(book, false)}
										class="w-full sm:w-auto px-3 py-1.5 bg-neutral-100 hover:bg-red-50 hover:text-red-700 hover:border-red-600 text-neutral-600 font-black text-[11px] uppercase tracking-wider border-2 border-black transition-all flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-60"
									>
										{#if actionLoadingId === book.id}
											<RefreshCw class="w-3 h-3 animate-spin" />
											<span>UKLÁDÁM...</span>
										{:else}
											<X class="w-3.5 h-3.5" />
											<span>ZRUŠIT PŘIJETÍ</span>
										{/if}
									</button>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>

<!-- Photo Zoom Modal -->
{#if selectedPhotoBook}
	<div
		class="fixed inset-0 z-50 bg-black/80 backdrop-blur-xs flex items-center justify-center p-4 cursor-pointer"
		onclick={() => (selectedPhotoBook = null)}
		role="presentation"
	>
		<div
			class="bg-white border-2 border-black max-w-lg w-full p-4 shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] cursor-default text-black flex flex-col gap-3"
			onclick={(e) => e.stopPropagation()}
			role="presentation"
		>
			<div class="flex items-center justify-between border-b-2 border-black pb-2">
				<div>
					<div class="text-sm font-black uppercase tracking-wide">#{selectedPhotoBook.id}</div>
					<div class="text-xs text-neutral-600 font-bold">{selectedPhotoBook.price} Kč</div>
				</div>
				<button
					onclick={() => (selectedPhotoBook = null)}
					class="p-1 hover:bg-neutral-100 border-2 border-black cursor-pointer"
				>
					<X class="w-5 h-5" />
				</button>
			</div>

			<div class="w-full bg-neutral-100 border-2 border-black flex items-center justify-center max-h-[65vh] overflow-hidden">
				<img
					src={getBookFullImageUrl(selectedPhotoBook)}
					alt="Detail knihy"
					class="w-full h-auto max-h-[65vh] object-contain"
				/>
			</div>

			<div class="text-xs flex items-center justify-between">
				<div class="text-neutral-600 font-medium">
					{selectedPhotoBook.expand?.seller?.name || selectedPhotoBook.expand?.seller?.email || 'Neznámý prodejce'}
				</div>
				{#if selectedPhotoBook.accepted}
					<span class="font-black text-emerald-700 uppercase">✓ PŘIJATO K PRODEJI</span>
				{:else}
					<span class="font-black text-amber-700 uppercase">✗ ČEKÁ NA PŘIJETÍ</span>
				{/if}
			</div>
		</div>
	</div>
{/if}
