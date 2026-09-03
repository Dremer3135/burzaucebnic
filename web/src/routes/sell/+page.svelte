<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, eventStore, sellerBooks } from '$lib/stores.svelte';
	import { pb, getBookThumbnailUrl, getBookFullImageUrl } from '$lib/pocketbase';
	import { scanFrameForDataMatrix, capturePhotoFromVideo, drawBoundingBox } from '$lib/scanner';
	import { Plus, Camera, Check, X, Tag, AlertCircle, RefreshCw, ChevronLeft } from '@lucide/svelte';
	import type { Book } from '$lib/types';

	let isModalOpen = $state(false);
	// Steps: 'SCAN_CODE' | 'CAPTURE_COVER' | 'ENTER_PRICE'
	let step = $state<'SCAN_CODE' | 'CAPTURE_COVER' | 'ENTER_PRICE'>('SCAN_CODE');

	let scannedCode = $state('');
	let priceInput = $state<number | ''>('');
	let capturedPhotoBlob = $state<Blob | null>(null);
	let photoPreviewUrl = $state<string | null>(null);

	let videoElement = $state<HTMLVideoElement | null>(null);
	let canvasElement = $state<HTMLCanvasElement | null>(null);
	let overlayCanvas = $state<HTMLCanvasElement | null>(null);
	let mediaStream = $state<MediaStream | null>(null);
	let scanAnimationId = $state<number | null>(null);

	let errorMessage = $state('');
	let isSubmitting = $state(false);

	// Full-res preview modal
	let selectedPreviewBook = $state<Book | null>(null);

	$effect(() => {
		if (!auth.user) {
			goto('/');
		} else {
			sellerBooks.init(auth.user.id);
		}
	});

	onDestroy(() => {
		stopCamera();
		if (photoPreviewUrl) URL.revokeObjectURL(photoPreviewUrl);
	});

	async function openSellModal() {
		if (!eventStore.event) {
			await eventStore.fetchActive();
		}
		step = 'SCAN_CODE';
		scannedCode = '';
		priceInput = '';
		capturedPhotoBlob = null;
		if (photoPreviewUrl) URL.revokeObjectURL(photoPreviewUrl);
		photoPreviewUrl = null;
		errorMessage = '';
		isModalOpen = true;

		// Start camera after modal renders
		setTimeout(startCamera, 100);
	}

	function closeSellModal() {
		stopCamera();
		isModalOpen = false;
		if (photoPreviewUrl) URL.revokeObjectURL(photoPreviewUrl);
		photoPreviewUrl = null;
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
			console.error('Camera access error', err);
			errorMessage = 'Nelze přistoupit ke kameře. Povolte prosím oprávnění ke kameře v prohlížeči.';
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

	let isScanningFrame = false;
	let lastScanTime = 0;

	async function startScanningLoop() {
		if (!videoElement || !canvasElement || !isModalOpen) return;

		const now = performance.now();
		if (step === 'SCAN_CODE' && !isScanningFrame && now - lastScanTime >= 65 && videoElement.videoWidth > 0) {
			isScanningFrame = true;
			lastScanTime = now;
			try {
				const match = await scanFrameForDataMatrix(canvasElement, videoElement);
				if (match && match.text) {
					// Vibrate feedback if supported
					if (navigator.vibrate) navigator.vibrate([100]);

					scannedCode = match.text.trim();
					step = 'CAPTURE_COVER';
					return;
				}
			} catch (err) {
				console.error(err);
			} finally {
				isScanningFrame = false;
			}
		}

		scanAnimationId = requestAnimationFrame(startScanningLoop);
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
		if (!mediaStream) startCamera();
		else startScanningLoop();
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
			formData.append('code', scannedCode.trim());
			formData.append('seller', auth.user.id);
			formData.append('event', eventStore.event.id);
			formData.append('price', String(priceInput));
			formData.append('status', 'available');
			formData.append('photo', capturedPhotoBlob, `book_${scannedCode.trim()}.jpg`);

			await pb.collection('books').create(formData);

			// Refresh seller list and close
			await sellerBooks.refresh();
			closeSellModal();
		} catch (err: any) {
			console.error('Book submission error', err);
			errorMessage = err?.message || 'Chyba při ukládání učebnice. Zkontrolujte, zda kód již neexistuje.';
		} finally {
			isSubmitting = false;
		}
	}

	function getStatusBadge(status: string) {
		switch (status) {
			case 'available':
				return { label: 'K prodeji', bg: 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30' };
			case 'checkout':
				return { label: 'V rezervaci', bg: 'bg-amber-500/20 text-amber-300 border-amber-500/30' };
			case 'bought':
				return { label: 'Prodáno', bg: 'bg-slate-700/60 text-slate-300 border-slate-600/50' };
			default:
				return { label: status, bg: 'bg-slate-700 text-slate-300' };
		}
	}
</script>

<div class="flex-1 max-w-4xl w-full mx-auto p-4 flex flex-col pb-24">
	<!-- Page Header -->
	<div class="flex items-center justify-between mb-4">
		<div>
			<h1 class="text-xl sm:text-2xl font-bold text-white tracking-tight">Moje učebnice k prodeji</h1>
			<p class="text-xs text-slate-400">
				Přehled vámi nabízených učebnic a jejich aktuální stav
			</p>
		</div>

		<button
			onclick={() => sellerBooks.refresh()}
			class="p-2 rounded-xl bg-slate-800 text-slate-300 hover:text-white border border-slate-700 transition-colors"
			title="Obnovit"
		>
			<RefreshCw class="w-4 h-4 {sellerBooks.isLoading ? 'animate-spin text-emerald-400' : ''}" />
		</button>
	</div>

	<!-- Books List -->
	{#if sellerBooks.isLoading && sellerBooks.books.length === 0}
		<div class="flex-1 flex items-center justify-center py-16">
			<RefreshCw class="w-6 h-6 animate-spin text-emerald-500" />
		</div>
	{:else if sellerBooks.books.length === 0}
		<div class="flex-1 flex flex-col items-center justify-center p-8 bg-slate-800/40 border border-dashed border-slate-700 rounded-3xl text-center my-6">
			<div class="p-4 bg-emerald-500/10 text-emerald-400 rounded-full mb-3">
				<Tag class="w-8 h-8" />
			</div>
			<h3 class="text-base font-semibold text-white mb-1">Zatím nemáte vystavené žádné učebnice</h3>
			<p class="text-xs text-slate-400 max-w-xs mb-5">
				Klepněte na zelené tlačítko plus v pravém dolním rohu pro naskenování kódu a přidání první učebnice.
			</p>
			<button
				onclick={openSellModal}
				class="inline-flex items-center gap-2 py-2.5 px-4 bg-emerald-600 hover:bg-emerald-500 text-white font-medium rounded-xl text-xs transition-colors shadow-lg shadow-emerald-950 cursor-pointer"
			>
				<Plus class="w-4 h-4" />
				Přidat učebnici do prodeje
			</button>
		</div>
	{:else}
		<div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
			{#each sellerBooks.books as book (book.id)}
				{@const badge = getStatusBadge(book.status)}
				<div
					class="bg-slate-800/80 border border-slate-700/70 rounded-2xl p-3 flex gap-3 shadow-md hover:border-slate-600 transition-all backdrop-blur"
				>
					<!-- Thumbnail with click to preview full res -->
					<button
						type="button"
						onclick={() => (selectedPreviewBook = book)}
						class="w-20 h-28 shrink-0 rounded-xl overflow-hidden bg-slate-900 border border-slate-700/80 relative group cursor-pointer"
					>
						{#if book.photo}
							<img
								src={getBookThumbnailUrl(book)}
								alt={book.code}
								class="w-full h-full object-cover group-hover:scale-105 transition-transform"
							/>
						{:else}
							<div class="w-full h-full flex items-center justify-center text-slate-600">
								<Camera class="w-6 h-6" />
							</div>
						{/if}
						<div class="absolute inset-0 bg-black/30 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center text-[10px] text-white font-medium">
							Detail
						</div>
					</button>

					<!-- Info -->
					<div class="flex-1 flex flex-col justify-between py-0.5">
						<div>
							<div class="flex items-center justify-between gap-1 mb-1.5">
								<span class="text-xs font-semibold text-slate-200 truncate" title={book.code}>
									{book.code}
								</span>
							</div>

							<div class="text-lg font-bold text-emerald-400">
								{book.price} Kč
							</div>
						</div>

						<div class="mt-2 flex items-center justify-between">
							<span class="text-[11px] px-2 py-0.5 rounded-full border {badge.bg} font-medium">
								{badge.label}
							</span>
						</div>
					</div>
				</div>
			{/each}
		</div>
	{/if}

	<!-- Intuitive Green Circular Plus Button (FAB) at Bottom Right -->
	<button
		onclick={openSellModal}
		class="fixed bottom-6 right-6 z-30 w-14 h-14 rounded-full bg-emerald-500 hover:bg-emerald-400 active:scale-95 text-slate-950 font-bold shadow-xl shadow-emerald-950/50 flex items-center justify-center transition-all cursor-pointer"
		title="Přidat učebnici k prodeji"
	>
		<Plus class="w-7 h-7 stroke-[2.5]" />
	</button>
</div>

<!-- FULL-RES IMAGE MODAL -->
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

<!-- ADD BOOK CAMERA / SUBMIT MODAL -->
{#if isModalOpen}
	<div class="fixed inset-0 z-50 bg-slate-950 flex flex-col">
		<!-- Top Bar -->
		<div class="flex items-center justify-between px-4 py-3 bg-slate-900 border-b border-slate-800 z-10">
			<div class="flex items-center gap-2">
				{#if step !== 'SCAN_CODE'}
					<button
						onclick={step === 'ENTER_PRICE' ? retakePhoto : rescanCode}
						class="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800"
					>
						<ChevronLeft class="w-5 h-5" />
					</button>
				{/if}
				<h2 class="text-sm font-semibold text-white">
					{step === 'SCAN_CODE'
						? '1/3 Naskenujte Data Matrix'
						: step === 'CAPTURE_COVER'
							? '2/3 Vyfoťte obálku knihy'
							: '3/3 Zadejte cenu učebnice'}
				</h2>
			</div>

			<button
				onclick={closeSellModal}
				class="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800"
			>
				<X class="w-5 h-5" />
			</button>
		</div>

		<!-- Main Camera / Preview Area -->
		<div class="relative flex-1 bg-black flex items-center justify-center overflow-hidden">
			<!-- Live Video -->
			<video
				bind:this={videoElement}
				playsinline
				autoplay
				muted
				class="w-full h-full object-cover {step === 'ENTER_PRICE' ? 'hidden' : 'block'}"
			></video>

			<!-- Hidden helper canvas for barcode processing -->
			<canvas bind:this={canvasElement} class="hidden"></canvas>

			<!-- STEP 1 OVERLAY: Scanning for Data Matrix -->
			{#if step === 'SCAN_CODE'}
				<div class="absolute inset-0 pointer-events-none flex flex-col items-center justify-center">
					<div class="w-56 h-56 border-2 border-emerald-400/80 rounded-2xl relative animate-pulse flex items-center justify-center">
						<div class="absolute inset-0 bg-emerald-500/10 rounded-2xl"></div>
						<!-- Corner accents -->
						<div class="absolute -top-1 -left-1 w-6 h-6 border-t-4 border-l-4 border-emerald-400 rounded-tl-lg"></div>
						<div class="absolute -top-1 -right-1 w-6 h-6 border-t-4 border-r-4 border-emerald-400 rounded-tr-lg"></div>
						<div class="absolute -bottom-1 -left-1 w-6 h-6 border-b-4 border-l-4 border-emerald-400 rounded-bl-lg"></div>
						<div class="absolute -bottom-1 -right-1 w-6 h-6 border-b-4 border-r-4 border-emerald-400 rounded-br-lg"></div>
						<span class="text-xs font-semibold text-emerald-300 bg-slate-900/80 px-2 py-1 rounded">
							Hledám Data Matrix...
						</span>
					</div>
					<p class="text-xs text-white/90 bg-slate-900/80 px-3 py-1.5 rounded-full mt-6 backdrop-blur shadow">
						Namiřte kameru na samolepku s kódem
					</p>
				</div>
			{/if}

			<!-- STEP 2 OVERLAY: Book Aspect Ratio Guide Rectangle -->
			{#if step === 'CAPTURE_COVER'}
				<div class="absolute inset-0 pointer-events-none flex flex-col items-center justify-center">
					<!-- Standard Book Aspect Ratio ~ 1:1.41 (A-format or B-format book) -->
					<div class="w-[70vw] max-w-xs aspect-[1/1.4] border-2 border-white/90 rounded-2xl shadow-2xl relative">
						<!-- Translucent guide lines -->
						<div class="absolute inset-0 border border-dashed border-white/40 rounded-2xl m-2"></div>
						<!-- Badge at top -->
						<div class="absolute -top-3 left-1/2 -translate-x-1/2 bg-emerald-600 text-white text-[11px] font-bold px-2.5 py-0.5 rounded-full shadow">
							Kód: {scannedCode}
						</div>
					</div>
					<p class="text-xs text-white bg-slate-900/80 px-3 py-1.5 rounded-full mt-4 backdrop-blur shadow">
						Zarovnejte přední obálku do rámečku
					</p>
				</div>

				<!-- Shutter Button -->
				<div class="absolute bottom-6 inset-x-0 flex justify-center items-center z-20">
					<button
						onclick={takeBookPhoto}
						class="w-18 h-18 rounded-full border-4 border-white bg-emerald-500 hover:bg-emerald-400 active:scale-95 transition-all shadow-2xl flex items-center justify-center cursor-pointer"
						title="Vyfotit obálku"
					>
						<Camera class="w-8 h-8 text-slate-950" />
					</button>
				</div>
			{/if}

			<!-- STEP 3: Photo Preview and Price Entry -->
			{#if step === 'ENTER_PRICE' && photoPreviewUrl}
				<div class="absolute inset-0 bg-slate-900 flex flex-col p-4 overflow-y-auto">
					<div class="flex-1 flex flex-col items-center justify-center max-w-sm mx-auto w-full">
						<div class="relative w-44 aspect-[1/1.4] rounded-2xl overflow-hidden shadow-2xl border-2 border-slate-700 mb-4 bg-slate-950">
							<img src={photoPreviewUrl} alt="Obálka" class="w-full h-full object-cover" />
							<button
								type="button"
								onclick={retakePhoto}
								class="absolute top-2 right-2 p-1.5 rounded-lg bg-black/60 text-white hover:bg-black text-xs flex items-center gap-1"
							>
								<RefreshCw class="w-3.5 h-3.5" />
								Znovu
							</button>
						</div>

						<div class="w-full bg-slate-800/90 rounded-2xl p-4 border border-slate-700/80 shadow-xl">
							<div class="text-xs text-slate-400 mb-1">Kód Data Matrix:</div>
							<div class="text-sm font-mono font-bold text-emerald-400 mb-4 bg-slate-900 px-3 py-1.5 rounded-lg border border-slate-800">
								{scannedCode}
							</div>

							{#if errorMessage}
								<div class="mb-3 p-2.5 bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs rounded-xl flex items-start gap-2">
									<AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
									<span>{errorMessage}</span>
								</div>
							{/if}

							<form onsubmit={submitBook} class="space-y-4">
								<div>
									<label for="book-price" class="block text-xs font-medium text-slate-300 mb-1">
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
											class="w-full bg-slate-900 border border-slate-700 rounded-xl px-4 py-2.5 text-lg font-bold text-white focus:outline-none focus:border-emerald-500 text-center"
										/>
										<span class="absolute right-4 top-3 text-sm font-semibold text-slate-400">
											Kč
										</span>
									</div>
								</div>

								<button
									type="submit"
									disabled={isSubmitting}
									class="w-full py-3 rounded-xl bg-emerald-600 hover:bg-emerald-500 active:scale-98 text-white font-bold text-sm transition-colors shadow-lg shadow-emerald-950 disabled:opacity-50 cursor-pointer flex items-center justify-center gap-2"
								>
									{#if isSubmitting}
										<RefreshCw class="w-4 h-4 animate-spin" />
										<span>Ukládám a komprimuji...</span>
									{:else}
										<Check class="w-5 h-5" />
										<span>Vystavit k prodeji</span>
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
