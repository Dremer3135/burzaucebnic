import { readBarcodes, type Position, type ReaderOptions } from 'zxing-wasm/reader';

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

// Exact camera constraints proven in strelavlna3
export const CAMERA_CONSTRAINTS: MediaStreamConstraints = {
	video: {
		facingMode: 'environment',
		width: { ideal: 1920 },
		height: { ideal: 1080 },
		// @ts-ignore
		advanced: [{ focusMode: 'continuous' }]
	},
	audio: false
};

// Exact scan options proven in strelavlna3
export const SCAN_OPTIONS: ReaderOptions = {
	formats: ['DataMatrix', 'QRCode'],
	tryHarder: true, // Crucial for rotation/tilt
	maxNumberOfSymbols: 1
};

export function getVideoTransform(
	videoW: number,
	videoH: number,
	elementW: number,
	elementH: number,
	offsetX = 0,
	offsetY = 0
): VideoTransform {
	if (!videoW || !videoH || !elementW || !elementH) {
		return { scale: 1, offsetX: 0, offsetY: 0 };
	}
	const scale = Math.max(elementW / videoW, elementH / videoH);
	const totalOffsetX = (elementW - videoW * scale) / 2 + offsetX;
	const totalOffsetY = (elementH - videoH * scale) / 2 + offsetY;
	return { scale, offsetX: totalOffsetX, offsetY: totalOffsetY };
}

export async function scanFrameForDataMatrix(
	canvas: HTMLCanvasElement,
	video: HTMLVideoElement
): Promise<ScanMatch | null> {
	if (!video.videoWidth || !video.videoHeight) return null;

	const width = video.videoWidth;
	const height = video.videoHeight;

	if (canvas.width !== width || canvas.height !== height) {
		canvas.width = width;
		canvas.height = height;
	}

	const ctx = canvas.getContext('2d', { willReadFrequently: true });
	if (!ctx) return null;

	ctx.drawImage(video, 0, 0, width, height);
	const imageData = ctx.getImageData(0, 0, width, height);

	try {
		const results = await readBarcodes(imageData, SCAN_OPTIONS);

		if (results && results.length > 0) {
			const res = results[0];
			const pos = res.position;

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

export async function scanFrameForAllDataMatrices(
	canvas: HTMLCanvasElement,
	video: HTMLVideoElement,
	maxSymbols = 8
): Promise<ScanMatch[]> {
	if (!video.videoWidth || !video.videoHeight) return [];

	const width = video.videoWidth;
	const height = video.videoHeight;

	if (canvas.width !== width || canvas.height !== height) {
		canvas.width = width;
		canvas.height = height;
	}

	const ctx = canvas.getContext('2d', { willReadFrequently: true });
	if (!ctx) return [];

	ctx.drawImage(video, 0, 0, width, height);
	const imageData = ctx.getImageData(0, 0, width, height);

	try {
		const results = await readBarcodes(imageData, {
			formats: ['DataMatrix'],
			tryHarder: true,
			maxNumberOfSymbols: maxSymbols
		});

		if (results && results.length > 0) {
			return results.map((res) => {
				const pos = res.position;
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
			});
		}
	} catch (err) {
		// No code in frame or wasm error
	}

	return [];
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

	// Draw sharp corner squares for minimalist sharp design
	const points = [pos.topLeft, pos.topRight, pos.bottomRight, pos.bottomLeft];
	ctx.fillStyle = '#ffffff';
	for (const p of points) {
		const px = p.x * scaleX + offsetX;
		const py = p.y * scaleY + offsetY;
		ctx.fillRect(px - 4, py - 4, 8, 8);
		ctx.strokeStyle = color;
		ctx.lineWidth = 2;
		ctx.strokeRect(px - 4, py - 4, 8, 8);
	}
	ctx.restore();
}

export function drawStatusTag(
	ctx: CanvasRenderingContext2D,
	pos: Position,
	transform: VideoTransform,
	status: 'used' | 'available' | 'checking'
) {
	const { scale, offsetX, offsetY } = transform;
	const p1 = { x: pos.topLeft.x * scale + offsetX, y: pos.topLeft.y * scale + offsetY };
	const p2 = { x: pos.topRight.x * scale + offsetX, y: pos.topRight.y * scale + offsetY };
	const midX = (p1.x + p2.x) / 2;
	const minY = Math.min(p1.y, p2.y) - 6;

	let text = 'VOLNÝ';
	let bgColor = '#059669';
	if (status === 'used') {
		text = 'JIŽ POUŽITO';
		bgColor = '#dc2626';
	} else if (status === 'checking') {
		text = 'OVĚŘUJI...';
		bgColor = '#d97706';
	}

	ctx.save();
	ctx.font = '900 12px monospace, system-ui, sans-serif';
	const tm = ctx.measureText(text);
	const boxW = tm.width + 14;
	const boxH = 22;
	const boxX = Math.round(midX - boxW / 2);
	let boxY = Math.round(minY - boxH);
	if (boxY < 4) {
		boxY = Math.round(Math.min(p1.y, p2.y) + 6);
	}

	ctx.fillStyle = bgColor;
	ctx.fillRect(boxX, boxY, boxW, boxH);
	ctx.strokeStyle = '#000000';
	ctx.lineWidth = 2;
	ctx.strokeRect(boxX, boxY, boxW, boxH);

	ctx.fillStyle = '#ffffff';
	ctx.textAlign = 'center';
	ctx.textBaseline = 'middle';
	ctx.fillText(text, Math.round(midX), boxY + boxH / 2);
	ctx.restore();
}

export interface PolygonColor {
	bg: string;
	border: string;
	text: string;
	lightBg: string;
}

export function idToColor(id: string): PolygonColor {
	let hash = 0;
	for (let i = 0; i < id.length; i++) {
		hash = (hash << 5) - hash + id.charCodeAt(i);
		hash |= 0;
	}
	const hue = Math.abs(hash) % 360;
	return {
		bg: `hsl(${hue}, 80%, 38%)`,
		border: `hsl(${hue}, 90%, 22%)`,
		text: '#ffffff',
		lightBg: `hsl(${hue}, 65%, 92%)`
	};
}

export function drawPricePolygon(
	ctx: CanvasRenderingContext2D,
	pos: Position,
	priceText: string,
	transform: VideoTransform,
	color: PolygonColor = {
		bg: '#ffffff',
		border: '#000000',
		text: '#000000',
		lightBg: '#f5f5f5'
	}
) {
	const { scale, offsetX, offsetY } = transform;

	const p1 = { x: pos.topLeft.x * scale + offsetX, y: pos.topLeft.y * scale + offsetY };
	const p2 = { x: pos.topRight.x * scale + offsetX, y: pos.topRight.y * scale + offsetY };
	const p3 = { x: pos.bottomRight.x * scale + offsetX, y: pos.bottomRight.y * scale + offsetY };
	const p4 = { x: pos.bottomLeft.x * scale + offsetX, y: pos.bottomLeft.y * scale + offsetY };

	ctx.save();

	// 1. Draw solid opaque colored polygon
	ctx.beginPath();
	ctx.moveTo(p1.x, p1.y);
	ctx.lineTo(p2.x, p2.y);
	ctx.lineTo(p3.x, p3.y);
	ctx.lineTo(p4.x, p4.y);
	ctx.closePath();

	ctx.fillStyle = color.bg;
	ctx.fill();

	// 2. Sharp border
	ctx.lineWidth = 2.5;
	ctx.strokeStyle = color.border;
	ctx.lineJoin = 'miter';
	ctx.stroke();

	// 3. Center point of polygon
	const centerX = (p1.x + p2.x + p3.x + p4.x) / 4;
	const centerY = (p1.y + p2.y + p3.y + p4.y) / 4;

	// Dimensions of the quadrilateral
	const widthTop = Math.hypot(p2.x - p1.x, p2.y - p1.y);
	const widthBottom = Math.hypot(p3.x - p4.x, p3.y - p4.y);
	const avgWidth = (widthTop + widthBottom) / 2;

	const heightLeft = Math.hypot(p4.x - p1.x, p4.y - p1.y);
	const heightRight = Math.hypot(p3.x - p2.x, p3.y - p2.y);
	const avgHeight = (heightLeft + heightRight) / 2;

	const minDim = Math.min(avgWidth, avgHeight);

	// Angle from top edge (p1 -> p2)
	let angle = Math.atan2(p2.y - p1.y, p2.x - p1.x);
	// Keep text upright (within -90 to +90 degrees)
	if (angle > Math.PI / 2) angle -= Math.PI;
	if (angle < -Math.PI / 2) angle += Math.PI;

	// Text size scaled to fit polygon (approx 28% of min dimension)
	let fontSize = Math.max(10, Math.min(54, Math.floor(minDim * 0.28)));

	ctx.translate(centerX, centerY);
	ctx.rotate(angle);

	ctx.fillStyle = color.text;
	ctx.font = `900 ${fontSize}px ui-sans-serif, system-ui, -apple-system, sans-serif`;

	// Clamp text size if it exceeds polygon width
	const textWidth = ctx.measureText(priceText).width;
	const maxTextWidth = avgWidth * 0.82;
	if (textWidth > maxTextWidth && textWidth > 0) {
		fontSize = Math.max(8, Math.floor(fontSize * (maxTextWidth / textWidth)));
		ctx.font = `900 ${fontSize}px ui-sans-serif, system-ui, -apple-system, sans-serif`;
	}

	ctx.textAlign = 'center';
	ctx.textBaseline = 'middle';
	ctx.fillText(priceText, 0, 0);

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
