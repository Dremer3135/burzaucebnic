<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth, priceStore } from '$lib/stores.svelte';
	import {
		CAMERA_CONSTRAINTS,
		getVideoTransform,
		scanFrameForAllDataMatrices,
		drawPricePolygon,
		type ScanMatch
	} from '$lib/scanner';
	import { Camera, RefreshCw, AlertCircle } from '@lucide/svelte';

	let videoElement = $state<HTMLVideoElement | null>(null);
	let captureCanvas = $state<HTMLCanvasElement | null>(null);
	let overlayCanvas = $state<HTMLCanvasElement | null>(null);

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
		if (!auth.user) {
			goto('/');
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
			const displayW = overlayCanvas.clientWidth;
			const displayH = overlayCanvas.clientHeight;

			if (overlayCanvas.width !== displayW * dpr || overlayCanvas.height !== displayH * dpr) {
				overlayCanvas.width = displayW * dpr;
				overlayCanvas.height = displayH * dpr;
			}

			const ctx = overlayCanvas.getContext('2d');
			if (ctx) {
				ctx.save();
				ctx.scale(dpr, dpr);
				ctx.clearRect(0, 0, displayW, displayH);

				const transform = getVideoTransform(
					videoElement.videoWidth,
					videoElement.videoHeight,
					displayW,
					displayH
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

<div class="relative flex-1 w-full h-[calc(100vh-65px)] bg-black overflow-hidden flex items-center justify-center select-none">
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

	<!-- Top instruction overlay badge -->
	{#if isCameraReady && !cameraError}
		<div class="absolute top-4 inset-x-0 flex justify-center pointer-events-none z-10 px-4">
			<div class="bg-white/95 text-black border-2 border-black px-4 py-2 text-xs font-black uppercase tracking-wider shadow-none flex items-center gap-2">
				<Camera class="w-4 h-4 text-black" />
				<span>Namiřte kameru na samolepky učebnic</span>
			</div>
		</div>
	{/if}

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
</div>
