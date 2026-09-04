<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { auth, priceStore } from '$lib/stores.svelte';
	import {
		CAMERA_CONSTRAINTS,
		getVideoTransform,
		scanFrameForAllDataMatrices,
		drawPricePolygon,
		type ScanMatch
	} from '$lib/scanner';
	import { renderDataMatrix } from '$lib/barcodes';
	import { RefreshCw, AlertCircle, X } from '@lucide/svelte';

	let videoElement = $state<HTMLVideoElement | null>(null);
	let captureCanvas = $state<HTMLCanvasElement | null>(null);
	let overlayCanvas = $state<HTMLCanvasElement | null>(null);

	let isUserCodeModalOpen = $state(false);
	let miniBadgeCanvas = $state<HTMLCanvasElement | null>(null);
	let modalCodeCanvas = $state<HTMLCanvasElement | null>(null);

	let mediaStream = $state<MediaStream | null>(null);
	let isScanningLoopActive = false;
	let renderAnimId = $state<number | null>(null);

	let cameraError = $state<string | null>(null);
	let isCameraReady = $state(false);

	// Multi-detection persistence map to prevent flicker
	interface TrackedMatch {
		match: ScanMatch;
		lastSeen: number;
	}
	const trackedMatches = new Map<string, TrackedMatch>();



	$effect(() => {
		if (miniBadgeCanvas && auth.user?.id) {
			renderDataMatrix(miniBadgeCanvas, auth.user.id, 2).catch(console.error);
		}
	});

	$effect(() => {
		if (isUserCodeModalOpen && modalCodeCanvas && auth.user?.id) {
			renderDataMatrix(modalCodeCanvas, auth.user.id, 8).catch(console.error);
		}
	});

	onMount(() => {
		startCamera();
	});

	onDestroy(() => {
		stopCamera();
	});

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
			cameraError = 'Nelze přistoupit ke kameře. Povolte prosím přístup ke kameře v prohlížeči.';
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
		if (!isDetecting && videoElement && captureCanvas && videoElement.videoWidth > 0 && now - lastDetectTime >= 65) {
			isDetecting = true;
			lastDetectTime = now;

			try {
				const detected = await scanFrameForAllDataMatrices(captureCanvas, videoElement, 8);
				const seenTime = performance.now();

				for (const match of detected) {
					const code = match.text.trim();
					if (code) {
						// Trigger background cache fetch if not in cache
						if (!priceStore.has(code)) {
							priceStore.fetchPrice(code);
						}
						trackedMatches.set(code, { match, lastSeen: seenTime });
					}
				}

				// Evict codes not seen in the last 220ms
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

				// Draw live price polygons for all verified books
				for (const { match } of trackedMatches.values()) {
					const code = match.text.trim();
					const priceInfo = priceStore.get(code);

					// If found and available in database, render solid opaque white polygon with price
					if (priceInfo) {
						drawPricePolygon(ctx, match.position, `${priceInfo.price} Kč`, transform);
					}
					// If null (not found in DB), do NOT draw a polygon
				}

				ctx.restore();
			}
		}

		renderAnimId = requestAnimationFrame(runRenderLoop);
	}
</script>

<div class="relative flex-1 w-full h-full bg-black overflow-hidden flex items-center justify-center select-none touch-none">
	<!-- Full-screen video element -->
	<video
		bind:this={videoElement}
		playsinline
		autoplay
		muted
		class="w-full h-full object-cover"
	></video>

	<!-- Off-screen processing canvas -->
	<canvas bind:this={captureCanvas} class="hidden"></canvas>

	<!-- High-DPI Real-time Overlay Canvas -->
	<canvas
		bind:this={overlayCanvas}
		class="absolute inset-0 pointer-events-none w-full h-full"
	></canvas>

	<!-- Camera Error State -->
	{#if cameraError}
		<div class="absolute inset-0 bg-white flex flex-col items-center justify-center p-6 text-center z-20">
			<div class="p-3 bg-red-100 text-red-700 border-2 border-red-700 mb-4">
				<AlertCircle class="w-8 h-8" />
			</div>
			<h2 class="text-xl font-black uppercase tracking-tight text-black mb-2">Kamera není dostupná</h2>
			<p class="text-sm font-semibold text-neutral-600 max-w-sm mb-6 uppercase">
				{cameraError}
			</p>
			<button
				onclick={startCamera}
				class="inline-flex items-center gap-2 py-3 px-6 bg-black text-white font-black text-xs uppercase tracking-wider hover:bg-neutral-800 border-2 border-black transition-colors cursor-pointer"
			>
				<RefreshCw class="w-4 h-4" />
				ZKUSIT ZNOVU
			</button>
		</div>
	{/if}

	<!-- Buyer ID Corner Badge -->
	{#if auth.user}
		<button
			onclick={() => (isUserCodeModalOpen = true)}
			class="absolute bottom-6 right-4 z-20 flex items-center gap-2 bg-white text-black border-2 border-black p-2 shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] hover:bg-neutral-100 active:scale-95 transition-transform cursor-pointer"
			title="Zobrazit můj identifikační kód pro pokladnu"
		>
			<div class="w-8 h-8 border border-black bg-white flex items-center justify-center overflow-hidden shrink-0">
				<canvas bind:this={miniBadgeCanvas} class="w-full h-full"></canvas>
			</div>
			<div class="text-left pr-1">
				<div class="text-[10px] font-black uppercase tracking-wider leading-none">MŮJ KÓD</div>
				<div class="text-[9px] font-bold text-neutral-500 uppercase leading-tight">PRO POKLADNU</div>
			</div>
		</button>
	{/if}

	<!-- Enlarged High-Contrast Buyer ID Modal -->
	{#if isUserCodeModalOpen && auth.user}
		<div
			class="fixed inset-0 bg-black/80 backdrop-blur-xs flex items-center justify-center p-4 z-50 select-text"
			role="dialog"
			tabindex="-1"
			aria-modal="true"
			onkeydown={(e) => { if (e.key === 'Escape') isUserCodeModalOpen = false; }}
		>
			<!-- Backdrop click to dismiss -->
			<div
				class="fixed inset-0"
				onclick={() => (isUserCodeModalOpen = false)}
				role="presentation"
			></div>

			<div
				class="bg-white border-4 border-black p-6 relative flex items-center justify-center text-black shadow-[6px_6px_0px_0px_rgba(0,0,0,1)] z-10"
			>
				<button
					type="button"
					onclick={() => (isUserCodeModalOpen = false)}
					class="absolute top-2 right-2 p-1.5 border-2 border-black bg-white hover:bg-neutral-100 text-black cursor-pointer z-10"
					aria-label="Zavřít"
				>
					<X class="w-5 h-5" />
				</button>

				<!-- High-contrast Data Matrix Canvas -->
				<div class="bg-white p-2">
					<canvas bind:this={modalCodeCanvas} class="w-64 h-64 sm:w-72 sm:h-72"></canvas>
				</div>
			</div>
		</div>
	{/if}
</div>
