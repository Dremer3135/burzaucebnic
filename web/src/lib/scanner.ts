import { readBarcodes, type Position } from 'zxing-wasm/reader';

export interface ScanMatch {
	text: string;
	position: Position;
	// Screen coordinates calculated for overlay
	box: {
		x: number;
		y: number;
		width: number;
		height: number;
	};
}

export interface VideoTransform {
	scale: number;
	offsetX: number;
	offsetY: number;
}

export function getVideoTransform(
	videoW: number,
	videoH: number,
	containerW: number,
	containerH: number
): VideoTransform {
	if (!videoW || !videoH || !containerW || !containerH) {
		return { scale: 1, offsetX: 0, offsetY: 0 };
	}
	const scale = Math.max(containerW / videoW, containerH / videoH);
	const offsetX = (containerW - videoW * scale) / 2;
	const offsetY = (containerH - videoH * scale) / 2;
	return { scale, offsetX, offsetY };
}

export async function scanFrameForDataMatrix(
	canvas: HTMLCanvasElement,
	video: HTMLVideoElement
): Promise<ScanMatch | null> {
	if (!video.videoWidth || !video.videoHeight) return null;

	// Scale down large frames (e.g. 1080p) to max 720px for rapid WASM decoding without mobile frame drop
	const maxDim = 720;
	const originalW = video.videoWidth;
	const originalH = video.videoHeight;
	const scale = Math.min(1, maxDim / Math.max(originalW, originalH));

	const targetW = Math.round(originalW * scale);
	const targetH = Math.round(originalH * scale);

	if (canvas.width !== targetW || canvas.height !== targetH) {
		canvas.width = targetW;
		canvas.height = targetH;
	}

	const ctx = canvas.getContext('2d', { willReadFrequently: true });
	if (!ctx) return null;

	ctx.drawImage(video, 0, 0, targetW, targetH);
	const imageData = ctx.getImageData(0, 0, targetW, targetH);

	try {
		const results = await readBarcodes(imageData, {
			formats: ['DataMatrix'],
			tryHarder: false,
			maxNumberOfSymbols: 1
		});

		if (results && results.length > 0) {
			const res = results[0];
			// Map coordinates back to original video dimensions
			const invScale = 1 / scale;
			const pos: Position = {
				topLeft: { x: res.position.topLeft.x * invScale, y: res.position.topLeft.y * invScale },
				topRight: { x: res.position.topRight.x * invScale, y: res.position.topRight.y * invScale },
				bottomRight: { x: res.position.bottomRight.x * invScale, y: res.position.bottomRight.y * invScale },
				bottomLeft: { x: res.position.bottomLeft.x * invScale, y: res.position.bottomLeft.y * invScale }
			};

			const xs = [pos.topLeft.x, pos.topRight.x, pos.bottomRight.x, pos.bottomLeft.x];
			const ys = [pos.topLeft.y, pos.topRight.y, pos.bottomRight.y, pos.bottomLeft.y];
			const minX = Math.min(...xs);
			const maxX = Math.max(...xs);
			const minY = Math.min(...ys);
			const maxY = Math.max(...ys);

			return {
				text: res.text,
				position: pos,
				box: {
					x: minX,
					y: minY,
					width: maxX - minX,
					height: maxY - minY
				}
			};
		}
	} catch (err) {
		// No code in frame or wasm error
	}

	return null;
}

export function drawBoundingBox(
	ctx: CanvasRenderingContext2D,
	pos: Position,
	transformOrScaleX: VideoTransform | number,
	scaleYOrColor?: number | string,
	optionalColor?: string
) {
	let scaleX: number;
	let scaleY: number;
	let offsetX = 0;
	let offsetY = 0;
	let color = '#10b981';

	if (typeof transformOrScaleX === 'object') {
		scaleX = transformOrScaleX.scale;
		scaleY = transformOrScaleX.scale;
		offsetX = transformOrScaleX.offsetX;
		offsetY = transformOrScaleX.offsetY;
		if (typeof scaleYOrColor === 'string') color = scaleYOrColor;
	} else {
		scaleX = transformOrScaleX;
		scaleY = typeof scaleYOrColor === 'number' ? scaleYOrColor : transformOrScaleX;
		if (optionalColor) color = optionalColor;
	}

	ctx.save();
	ctx.beginPath();
	ctx.moveTo(pos.topLeft.x * scaleX + offsetX, pos.topLeft.y * scaleY + offsetY);
	ctx.lineTo(pos.topRight.x * scaleX + offsetX, pos.topRight.y * scaleY + offsetY);
	ctx.lineTo(pos.bottomRight.x * scaleX + offsetX, pos.bottomRight.y * scaleY + offsetY);
	ctx.lineTo(pos.bottomLeft.x * scaleX + offsetX, pos.bottomLeft.y * scaleY + offsetY);
	ctx.closePath();

	ctx.lineWidth = 4;
	ctx.strokeStyle = color;
	ctx.fillStyle = color + '2a';
	ctx.fill();
	ctx.stroke();

	// Draw corner dots
	const points = [pos.topLeft, pos.topRight, pos.bottomRight, pos.bottomLeft];
	ctx.fillStyle = '#ffffff';
	for (const p of points) {
		ctx.beginPath();
		ctx.arc(p.x * scaleX + offsetX, p.y * scaleY + offsetY, 5, 0, Math.PI * 2);
		ctx.fill();
		ctx.strokeStyle = color;
		ctx.lineWidth = 2;
		ctx.stroke();
	}
	ctx.restore();
}


// Snaps a picture from the video element, optionally cropped to the aspect ratio rectangle
export async function capturePhotoFromVideo(video: HTMLVideoElement): Promise<Blob> {
	const canvas = document.createElement('canvas');
	canvas.width = video.videoWidth;
	canvas.height = video.videoHeight;
	const ctx = canvas.getContext('2d');
	if (!ctx) throw new Error('Could not get canvas context');

	ctx.drawImage(video, 0, 0, canvas.width, canvas.height);

	return new Promise((resolve, reject) => {
		canvas.toBlob(
			(blob) => {
				if (blob) resolve(blob);
				else reject(new Error('Failed to create image blob'));
			},
			'image/jpeg',
			0.92
		);
	});
}
