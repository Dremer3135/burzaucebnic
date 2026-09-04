<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, eventStore, sellerBooks, priceStore } from '$lib/stores.svelte';
	import { pb, getBookThumbnailUrl, getBookFullImageUrl } from '$lib/pocketbase';
	import {
		scanFrameForAllDataMatrices,
		capturePhotoFromVideo,
		drawBoundingBox,
		drawStatusTag,
		getVideoTransform,
		CAMERA_CONSTRAINTS,
		type ScanMatch
	} from '$lib/scanner';
	import { Plus, Camera, Check, X, Tag, AlertCircle, RefreshCw, ChevronLeft, AlertTriangle } from '@lucide/svelte';
	import type { Book } from '$lib/types';

	let isModalOpen = $state(false);
	// Steps: 'SCAN_CODE' | 'CAPTURE_COVER' | 'ENTER_PRICE'
	let step = $state<'SCAN_CODE' | 'CAPTURE_COVER' | 'ENTER_PRICE'>('SCAN_CODE');

	let scannedCode = $state('');
	let activeDetectedCode = $state<string | null>(null);
	let activeMatch = $state<ScanMatch | null>(null);
	let activeCodeStatus = $state<'checking' | 'available' | 'used'>('checking');
	let lastSeenCodeTime = 0;
	let lastVibratedStatus = $state<string | null>(null);

	interface TrackedMatch {
		match: ScanMatch;
		lastSeen: number;
	}
	const trackedMatches = new Map<string, TrackedMatch>();

	function getCodeStatus(code: string): 'checking' | 'available' | 'used' {
		if (sellerBooks.books.some((b) => b.id === code)) return 'used';
		const isUsed = priceStore.isUsed(code);
		if (isUsed === true) return 'used';
		if (isUsed === false) return 'available';
		return 'checking';
	}

	function getStatusPriority(status: 'checking' | 'available' | 'used'): number {
		switch (status) {
			case 'available':
				return 2;
			case 'checking':
				return 1;
			case 'used':
				return 0;
		}
	}

	let priceInput = $state<number | ''>('');
	let capturedPhotoBlob = $state<Blob | null>(null);
	let photoPreviewUrl = $state<string | null>(null);

	let videoElement = $state<HTMLVideoElement | null>(null);
	let canvasElement = $state<HTMLCanvasElement | null>(null);
	let overlayCanvas = $state<HTMLCanvasElement | null>(null);
	let mediaStream = $state<MediaStream | null>(null);
	let scanAnimationId = $state<number | null>(null);
	let renderAnimId = $state<number | null>(null);

	let errorMessage = $state('');
	let isSubmitting = $state(false);

	// Full-res preview modal
	let selectedPreviewBook = $state<Book | null>(null);

	onMount(() => {
		if (auth.user) {
			sellerBooks.init(auth.user.id);
		}
	});

	$effect(() => {
		if (!auth.user) {
			goto('/');
		}
	});

	onDestroy(() => {
		stopCamera();
		if (photoPreviewUrl) URL.revokeObjectURL(photoPreviewUrl);
		sellerBooks.cleanup();
	});

	async function openSellModal() {
		if (!eventStore.event) {
			await eventStore.fetchActive();
		}
		step = 'SCAN_CODE';
		scannedCode = '';
		activeDetectedCode = null;
		activeMatch = null;
		activeCodeStatus = 'checking';
		lastSeenCodeTime = 0;
		lastVibratedStatus = null;
		trackedMatches.clear();
		priceInput = '';
		capturedPhotoBlob = null;
		if (photoPreviewUrl) URL.revokeObjectURL(photoPreviewUrl);
		photoPreviewUrl = null;
		errorMessage = '';
		isModalOpen = true;

		setTimeout(startCamera, 100);
	}

	function closeSellModal() {
		stopCamera();
		isModalOpen = false;
		activeDetectedCode = null;
		activeMatch = null;
		activeCodeStatus = 'checking';
		lastVibratedStatus = null;
		trackedMatches.clear();
		if (photoPreviewUrl) URL.revokeObjectURL(photoPreviewUrl);
		photoPreviewUrl = null;
	}

	async function startCamera() {
		stopCamera();
		try {
			const stream = await navigator.mediaDevices.getUserMedia(CAMERA_CONSTRAINTS);
			mediaStream = stream;
			if (videoElement) {
				videoElement.srcObject = stream;
				await videoElement.play();
				if (step === 'SCAN_CODE') {
					startScanningLoop();
					startRenderLoop();
				}
			}
		} catch (err: any) {
			console.error('Camera access error', err);
			errorMessage = 'Nelze přistoupit ke kameře. Povolte prosím oprávnění ke kameře v prohlížeči.';
		}
	}

	function stopCamera() {
		if (scanAnimationId) {
			cancelAnimationFrame(scanAnimationId);
			scanAnimationId = null;
		}
		if (renderAnimId) {
			cancelAnimationFrame(renderAnimId);
			renderAnimId = null;
		}
		if (mediaStream) {
			mediaStream.getTracks().forEach((t) => t.stop());
			mediaStream = null;
		}
		trackedMatches.clear();
	}

	let isScanningFrame = false;
	let lastScanTime = 0;

	async function startScanningLoop() {
		if (!videoElement || !canvasElement || !isModalOpen || step !== 'SCAN_CODE') return;

		const now = performance.now();
		if (!isScanningFrame && now - lastScanTime >= 65 && videoElement.videoWidth > 0) {
			isScanningFrame = true;
			lastScanTime = now;
			try {
				const detected = await scanFrameForAllDataMatrices(canvasElement, videoElement, 6);
				const seenTime = performance.now();

				for (const match of detected) {
					const code = match.text.trim();
					if (code) {
						if (!priceStore.has(code) && !sellerBooks.books.some((b) => b.id === code)) {
							priceStore.fetchPrice(code);
						}
						trackedMatches.set(code, { match, lastSeen: seenTime });
					}
				}

				// Evict stale matches
				for (const [code, item] of trackedMatches.entries()) {
					if (seenTime - item.lastSeen > 450) {
						trackedMatches.delete(code);
					}
				}

				// Pick best code closest to center of the video frame, prioritizing available > checking > used
				if (trackedMatches.size > 0 && videoElement) {
					let bestMatch: ScanMatch | null = null;
					let bestCode: string | null = null;
					let bestPriority = -1;
					let minCenterDist = Infinity;
					const centerX = videoElement.videoWidth / 2;
					const centerY = videoElement.videoHeight / 2;

					for (const [code, item] of trackedMatches.entries()) {
						const status = getCodeStatus(code);
						const priority = getStatusPriority(status);
						const boxCenterX = item.match.box.x + item.match.box.width / 2;
						const boxCenterY = item.match.box.y + item.match.box.height / 2;
						const dist = Math.hypot(boxCenterX - centerX, boxCenterY - centerY);

						if (
							priority > bestPriority ||
							(priority === bestPriority && dist < minCenterDist)
						) {
							bestPriority = priority;
							minCenterDist = dist;
							bestMatch = item.match;
							bestCode = code;
						}
					}

					if (bestCode && bestMatch) {
						const status = getCodeStatus(bestCode);
						const vibrationKey = `${bestCode}:${status}`;
						if (lastVibratedStatus !== vibrationKey && navigator.vibrate) {
							lastVibratedStatus = vibrationKey;
							if (status === 'used') {
								navigator.vibrate([150, 80, 150]);
							} else if (status === 'available') {
								navigator.vibrate([60]);
							}
						}

						activeDetectedCode = bestCode;
						activeMatch = bestMatch;
						activeCodeStatus = status;
						lastSeenCodeTime = seenTime;
					}
				} else {
					if (seenTime - lastSeenCodeTime > 1500) {
						activeDetectedCode = null;
						activeMatch = null;
						lastVibratedStatus = null;
					}
				}
			} catch (err) {
				console.error(err);
			} finally {
				isScanningFrame = false;
			}
		}

		if (isModalOpen && step === 'SCAN_CODE') {
			scanAnimationId = requestAnimationFrame(startScanningLoop);
		}
	}

	function startRenderLoop() {
		if (!isModalOpen || step !== 'SCAN_CODE') return;

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

				const now = performance.now();
				const vRect = videoElement.getBoundingClientRect();
				const transform = getVideoTransform(
					videoElement.videoWidth,
					videoElement.videoHeight,
					vRect.width,
					vRect.height,
					vRect.left - cRect.left,
					vRect.top - cRect.top
				);

				for (const [code, item] of trackedMatches.entries()) {
					if (now - item.lastSeen < 450) {
						const status = getCodeStatus(code);
						// Do not highlight unavailable/used codes if they are not the active/selected one
						if (status === 'used' && code !== activeDetectedCode) {
							continue;
						}

						let color = '#10b981'; // Green for available
						if (status === 'used') {
							color = '#ef4444'; // Red for used
						} else if (status === 'checking') {
							color = '#f59e0b'; // Amber for checking
						}

						drawBoundingBox(ctx, item.match.position, transform, color);
						drawStatusTag(ctx, item.match.position, transform, status);
					}
				}
				ctx.restore();
			}
		}

		if (isModalOpen && step === 'SCAN_CODE') {
			renderAnimId = requestAnimationFrame(startRenderLoop);
		}
	}

	function confirmScannedCode() {
		if (!activeDetectedCode || activeCodeStatus === 'used') return;
		if (navigator.vibrate) navigator.vibrate([100]);
		scannedCode = activeDetectedCode;
		step = 'CAPTURE_COVER';
		if (scanAnimationId) {
			cancelAnimationFrame(scanAnimationId);
			scanAnimationId = null;
		}
		if (renderAnimId) {
			cancelAnimationFrame(renderAnimId);
			renderAnimId = null;
		}
		if (overlayCanvas) {
			const ctx = overlayCanvas.getContext('2d');
			if (ctx) ctx.clearRect(0, 0, overlayCanvas.width, overlayCanvas.height);
		}
	}

	async function takeBookPhoto() {
		if (!videoElement) return;
		try {
			const blob = await capturePhotoFromVideo(videoElement);
			capturedPhotoBlob = blob;
			photoPreviewUrl = URL.createObjectURL(blob);
			stopCamera();
			step = 'ENTER_PRICE';
		} catch (err) {
			console.error(err);
			errorMessage = 'Nepodařilo se pořídit fotografii.';
		}
	}

	function retakePhoto() {
		if (photoPreviewUrl) URL.revokeObjectURL(photoPreviewUrl);
		photoPreviewUrl = null;
		capturedPhotoBlob = null;
		step = 'CAPTURE_COVER';
		startCamera();
	}

	function rescanCode() {
		step = 'SCAN_CODE';
		scannedCode = '';
		activeDetectedCode = null;
		activeMatch = null;
		activeCodeStatus = 'checking';
		lastSeenCodeTime = 0;
		lastVibratedStatus = null;
		trackedMatches.clear();
		if (!mediaStream) {
			startCamera();
		} else {
			startScanningLoop();
			startRenderLoop();
		}
	}

	async function submitBook(e: SubmitEvent) {
		e.preventDefault();
		if (!auth.user || !scannedCode || !capturedPhotoBlob || !priceInput || priceInput <= 0) {
			errorMessage = 'Vyplňte prosím všechny údaje a zadejte platnou cenu.';
			return;
		}

		if (!eventStore.event) {
			errorMessage = 'Nebyla nalezena žádná aktivní burza.';
			return;
		}

		isSubmitting = true;
		errorMessage = '';

		try {
			const formData = new FormData();
			formData.append('id', scannedCode.trim());
			formData.append('seller', auth.user.id);
			formData.append('event', eventStore.event.id);
			formData.append('price', String(priceInput));
			formData.append('status', 'available');
			formData.append('photo', capturedPhotoBlob, `book_${scannedCode.trim()}.jpg`);

			await pb.collection('books').create(formData);

			// Mark as used in priceStore immediately
			priceStore.set(scannedCode.trim(), {
				id: scannedCode.trim(),
				price: Number(priceInput),
				status: 'available'
			});

			await sellerBooks.refresh();
			closeSellModal();
		} catch (err: any) {
			console.error('Book submission error', err);
			const msg = String(err?.message || '').toLowerCase();
			if (err?.status === 400 || msg.includes('unique') || msg.includes('id') || msg.includes('exist')) {
				errorMessage = `Kód '${scannedCode.trim()}' je již v databázi zaregistrován. Použijte prosím jinou samolepku.`;
			} else {
				errorMessage = err?.message || 'Chyba při ukládání učebnice. Zkontrolujte, zda kód již neexistuje.';
			}
		} finally {
			isSubmitting = false;
		}
	}

	function getStatusBadge(status: string) {
		switch (status) {
			case 'available':
				return { label: 'K PRODEJI', cls: 'bg-black text-white border-black' };
			case 'checkout':
				return { label: 'V REZERVACI', cls: 'bg-neutral-200 text-black border-black' };
			case 'bought':
				return { label: 'PRODÁNO', cls: 'bg-white text-neutral-400 border-neutral-300 line-through' };
			default:
				return { label: status.toUpperCase(), cls: 'bg-white text-black border-black' };
		}
	}
</script>

<div class="flex-1 max-w-4xl w-full mx-auto p-4 flex flex-col pb-28 bg-white text-black overflow-y-auto">
	<!-- Page Header -->
	<div class="flex items-center justify-between mb-4 border-b-2 border-black pb-3">
		<div>
			<h1 class="text-2xl font-black uppercase tracking-tight text-black">MOJE UČEBNICE K PRODEJI</h1>
			<p class="text-xs font-bold text-neutral-600 uppercase">
				Přehled vámi nabízených učebnic a jejich aktuální stav
			</p>
		</div>

		<button
			onclick={() => sellerBooks.refresh()}
			class="p-2.5 bg-white text-black hover:bg-neutral-100 border-2 border-black transition-colors cursor-pointer"
			title="Obnovit"
		>
			<RefreshCw class="w-4 h-4 {sellerBooks.isLoading ? 'animate-spin' : ''}" />
		</button>
	</div>

	<!-- Books List -->
	{#if sellerBooks.isLoading && sellerBooks.books.length === 0}
		<div class="flex-1 flex items-center justify-center py-16">
			<RefreshCw class="w-8 h-8 animate-spin text-black" />
		</div>
	{:else if sellerBooks.books.length === 0}
		<div class="flex-1 flex flex-col items-center justify-center p-8 bg-neutral-50 border-2 border-dashed border-black text-center my-6">
			<div class="p-4 bg-white border-2 border-black mb-3">
				<Tag class="w-8 h-8 text-black" />
			</div>
			<h3 class="text-lg font-black uppercase tracking-tight text-black mb-1">Žádné učebnice k prodeji</h3>
			<p class="text-xs font-bold text-neutral-600 uppercase max-w-xs mb-6">
				Klepněte na tlačítko níže pro naskenování kódu a přidání první učebnice.
			</p>
			<button
				onclick={openSellModal}
				class="inline-flex items-center gap-2 py-3 px-6 bg-black text-white font-black text-sm uppercase tracking-wider hover:bg-neutral-800 active:bg-neutral-900 border-2 border-black transition-colors cursor-pointer"
			>
				<Plus class="w-5 h-5" />
				PŘIDAT UČEBNICI DO PRODEJE
			</button>
		</div>
	{:else}
		<div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
			{#each sellerBooks.books as book (book.id)}
				{@const badge = getStatusBadge(book.status)}
				<div
					class="bg-white border-2 border-black p-3 flex gap-3 text-black transition-all"
				>
					<!-- Thumbnail -->
					<button
						type="button"
						onclick={() => (selectedPreviewBook = book)}
						class="w-20 h-28 shrink-0 border-2 border-black overflow-hidden bg-neutral-100 relative group cursor-pointer"
					>
						{#if book.photo}
							<img
								src={getBookThumbnailUrl(book)}
								alt={book.id}
								class="w-full h-full object-cover"
							/>
						{:else}
							<div class="w-full h-full flex items-center justify-center text-neutral-400">
								<Camera class="w-6 h-6" />
							</div>
						{/if}
						<div class="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center text-[10px] text-white font-black uppercase">
							Detail
						</div>
					</button>

					<!-- Info -->
					<div class="flex-1 flex flex-col justify-between py-0.5">
						<div>
							<div class="flex items-center justify-between gap-1 mb-1.5">
								<span class="text-xs font-black text-black truncate uppercase" title={book.id}>
									{book.id}
								</span>
							</div>

							<div class="text-xl font-black text-black">
								{book.price} Kč
							</div>
						</div>

						<div class="mt-2 flex items-center justify-between">
							<span class="text-[10px] font-black px-2 py-0.5 border-2 {badge.cls}">
								{badge.label}
							</span>
						</div>
					</div>
				</div>
			{/each}
		</div>
	{/if}

	<!-- Plus FAB at Bottom Right -->
	<button
		onclick={openSellModal}
		class="fixed bottom-6 right-6 z-30 w-16 h-16 bg-black text-white border-4 border-black hover:bg-neutral-800 active:bg-neutral-900 flex items-center justify-center transition-all cursor-pointer"
		title="Přidat učebnici k prodeji"
	>
		<Plus class="w-8 h-8 stroke-[3]" />
	</button>
</div>

<!-- FULL-RES IMAGE MODAL -->
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
				alt={selectedPreviewBook.id}
				class="max-w-full max-h-[70vh] object-contain border-4 border-black bg-white mb-4"
			/>
			<div class="text-center bg-white border-2 border-black p-3 w-full">
				<p class="text-base font-black uppercase text-black">{selectedPreviewBook.id}</p>
				<p class="text-2xl font-black text-black">{selectedPreviewBook.price} Kč</p>
			</div>
		</div>
	</div>
{/if}

<!-- ADD BOOK CAMERA / SUBMIT MODAL -->
{#if isModalOpen}
	<div class="fixed inset-0 z-50 bg-black flex flex-col">
		<!-- Top Bar -->
		<div class="flex items-center justify-between px-4 py-3 bg-white border-b-2 border-black z-10 text-black">
			<div class="flex items-center gap-2">
				{#if step !== 'SCAN_CODE'}
					<button
						onclick={step === 'ENTER_PRICE' ? retakePhoto : rescanCode}
						class="p-1.5 border-2 border-black bg-white hover:bg-neutral-100 text-black cursor-pointer"
					>
						<ChevronLeft class="w-5 h-5" />
					</button>
				{/if}
				<h2 class="text-sm font-black uppercase tracking-wider text-black">
					{step === 'SCAN_CODE'
						? '1/3 Naskenujte Data Matrix'
						: step === 'CAPTURE_COVER'
							? '2/3 Vyfoťte obálku knihy'
							: '3/3 Zadejte cenu učebnice'}
				</h2>
			</div>

			<button
				onclick={closeSellModal}
				class="p-1.5 border-2 border-black bg-white hover:bg-neutral-100 text-black cursor-pointer"
			>
				<X class="w-5 h-5" />
			</button>
		</div>

		<!-- Main Camera / Preview Area -->
		<div class="relative flex-1 bg-black flex items-center justify-center overflow-hidden touch-none select-none">
			<video
				bind:this={videoElement}
				playsinline
				autoplay
				muted
				class="w-full h-full object-cover {step === 'ENTER_PRICE' ? 'hidden' : 'block'}"
			></video>

			<canvas bind:this={canvasElement} class="hidden"></canvas>

			<!-- STEP 1 OVERLAY -->
			{#if step === 'SCAN_CODE'}
				<canvas
					bind:this={overlayCanvas}
					class="absolute inset-0 pointer-events-none w-full h-full z-10"
				></canvas>

				{#if !activeDetectedCode}
					<div class="absolute inset-0 pointer-events-none flex flex-col items-center justify-center z-10">
						<div class="w-56 h-56 border-4 border-white relative flex items-center justify-center">
							<span class="text-xs font-black uppercase tracking-wider text-black bg-white px-2.5 py-1 border-2 border-black">
								Hledám kód...
							</span>
						</div>
						<p class="text-xs font-black uppercase tracking-wider text-black bg-white px-3 py-1.5 border-2 border-black mt-6">
							Namiřte kameru na samolepku
						</p>
					</div>
				{/if}

				{#if activeDetectedCode}
					<div class="absolute bottom-6 inset-x-0 flex flex-col items-center gap-2 z-20 px-4">
						{#if activeCodeStatus === 'used'}
							<div class="bg-red-600 text-white border-2 border-black px-4 py-2.5 text-center shadow-none w-full max-w-sm flex items-center justify-center gap-3">
								<AlertCircle class="w-6 h-6 flex-shrink-0 text-white" />
								<div class="text-left">
									<p class="text-xs font-black uppercase tracking-wider">KÓD JE JIŽ POUŽIT!</p>
									<p class="text-[11px] font-bold text-red-100">Kód <span class="font-mono">{activeDetectedCode}</span> už v systému existuje.</p>
								</div>
							</div>
							<div class="w-full max-w-sm py-3.5 bg-black text-white font-black text-xs uppercase tracking-wider border-2 border-black flex items-center justify-center gap-2 text-center px-3">
								<AlertTriangle class="w-4 h-4 text-amber-400" />
								<span>POUŽIJTE JINOU SAMOLEPKU</span>
							</div>
						{:else if activeCodeStatus === 'checking'}
							<div class="bg-white border-2 border-black px-4 py-2 text-xs font-black uppercase text-black flex items-center gap-2">
								<RefreshCw class="w-4 h-4 animate-spin text-neutral-600" />
								<span>OVĚŘUJI KÓD: <span class="font-mono">{activeDetectedCode}</span>...</span>
							</div>
							<button
								type="button"
								disabled
								class="w-full max-w-sm py-4 bg-neutral-200 text-neutral-500 font-black text-sm uppercase tracking-wider border-2 border-neutral-400 flex items-center justify-center gap-2 cursor-not-allowed"
							>
								<span>OVĚŘUJI DOSTUPNOST...</span>
							</button>
						{:else}
							<div class="bg-white border-2 border-black px-4 py-2 text-xs font-black uppercase text-black">
								KÓD JE VOLNÝ: <span class="text-emerald-700 font-mono font-black">{activeDetectedCode}</span>
							</div>
							<button
								type="button"
								onclick={confirmScannedCode}
								class="w-full max-w-sm py-4 bg-emerald-600 hover:bg-emerald-700 active:bg-emerald-800 text-white font-black text-sm uppercase tracking-wider border-2 border-black shadow-none flex items-center justify-center gap-2 cursor-pointer transition-colors"
							>
								<Check class="w-5 h-5" />
								<span>DALŠÍ (POKRAČOVAT NA FOTO)</span>
							</button>
						{/if}
					</div>
				{/if}
			{/if}

			<!-- STEP 2 OVERLAY -->
			{#if step === 'CAPTURE_COVER'}
				<div class="absolute inset-0 pointer-events-none flex flex-col items-center justify-center">
					<div class="w-[70vw] max-w-xs aspect-[1/1.4] border-4 border-white relative">
						<div class="absolute -top-4 left-1/2 -translate-x-1/2 bg-white text-black border-2 border-black text-xs font-black uppercase px-3 py-0.5">
							Kód: {scannedCode}
						</div>
					</div>
					<p class="text-xs font-black uppercase tracking-wider text-black bg-white px-3 py-1.5 border-2 border-black mt-4">
						Zarovnejte obálku do rámečku
					</p>
				</div>

				<!-- Shutter Button -->
				<div class="absolute bottom-6 inset-x-0 flex justify-center items-center z-20">
					<button
						onclick={takeBookPhoto}
						class="w-20 h-20 border-4 border-black bg-white hover:bg-neutral-200 active:bg-neutral-300 transition-all flex items-center justify-center cursor-pointer"
						title="Vyfotit obálku"
					>
						<Camera class="w-10 h-10 text-black" />
					</button>
				</div>
			{/if}

			<!-- STEP 3: Photo Preview and Price Entry -->
			{#if step === 'ENTER_PRICE' && photoPreviewUrl}
				<div class="absolute inset-0 bg-white flex flex-col p-4 overflow-y-auto text-black">
					<div class="flex-1 flex flex-col items-center justify-center max-w-sm mx-auto w-full">
						<div class="relative w-44 aspect-[1/1.4] overflow-hidden border-4 border-black mb-4 bg-neutral-100">
							<img src={photoPreviewUrl} alt="Obálka" class="w-full h-full object-cover" />
							<button
								type="button"
								onclick={retakePhoto}
								class="absolute top-2 right-2 p-1.5 bg-black text-white hover:bg-neutral-800 text-xs font-black uppercase flex items-center gap-1 border border-white"
							>
								<RefreshCw class="w-3.5 h-3.5" />
								Znovu
							</button>
						</div>

						<div class="w-full bg-white p-5 border-4 border-black">
							<div class="text-xs font-black uppercase text-neutral-600 mb-1">KÓD DATA MATRIX:</div>
							<div class="text-sm font-mono font-black text-black mb-4 bg-neutral-100 px-3 py-2 border-2 border-black">
								{scannedCode}
							</div>

							{#if errorMessage}
								<div class="mb-4 p-3 bg-red-50 border-2 border-red-600 text-red-700 text-xs font-bold flex items-start gap-2">
									<AlertCircle class="w-4 h-4 shrink-0 mt-0.5 text-red-600" />
									<span>{errorMessage}</span>
								</div>
							{/if}

							<form onsubmit={submitBook} class="space-y-4">
								<div>
									<label for="book-price" class="block text-xs font-black uppercase text-black mb-1">
										Prodejní cena (v Kč)
									</label>
									<div class="relative">
										<input
											id="book-price"
											type="number"
											bind:value={priceInput}
											min="1"
											max="10000"
											required
											placeholder="Např. 150"
											class="w-full bg-white border-2 border-black px-4 py-3 text-2xl font-black text-black focus:outline-none text-center"
										/>
										<span class="absolute right-4 top-4 text-sm font-black text-black">
											Kč
										</span>
									</div>
								</div>

								<button
									type="submit"
									disabled={isSubmitting}
									class="w-full py-3.5 bg-black text-white hover:bg-neutral-800 active:bg-neutral-900 font-black text-sm uppercase tracking-wider border-2 border-black transition-colors disabled:opacity-50 cursor-pointer flex items-center justify-center gap-2"
								>
									{#if isSubmitting}
										<RefreshCw class="w-4 h-4 animate-spin" />
										<span>UKLÁDÁM...</span>
									{:else}
										<Check class="w-5 h-5" />
										<span>VYSTAVIT K PRODEJI</span>
									{/if}
								</button>
							</form>
						</div>
					</div>
				</div>
			{/if}
		</div>
	</div>
{/if}
