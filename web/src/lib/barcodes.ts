import bwipjs from 'bwip-js';
import QRCode from 'qrcode';

export function renderDataMatrix(canvas: HTMLCanvasElement, text: string, scale = 6): Promise<void> {
	return new Promise((resolve, reject) => {
		try {
			bwipjs.toCanvas(canvas, {
				bcid: 'datamatrix',
				text: text,
				scale: scale,
				backgroundcolor: 'ffffff'
			});
			resolve();
		} catch (err) {
			reject(err);
		}
	});
}

export interface SpaydParams {
	iban: string;
	amount: number;
	vs: number;
	paymentId: string;
	currency?: string;
	payerEmail?: string;
}

export function formatSpaydString(params: SpaydParams): string {
	const ibanClean = params.iban.replace(/\s+/g, '').toUpperCase();
	const amountClean = params.amount.toFixed(2);
	const currency = params.currency || 'CZK';

	let rawMsg = params.paymentId || '';
	if (params.payerEmail && params.payerEmail.trim()) {
		const email = params.payerEmail.trim();
		rawMsg = rawMsg ? `${rawMsg} ${email}` : email;
	}

	// SPAYD message limit: max 60 characters
	// Reserved characters: '*' -> '%2A', '@' -> '%40', '%' -> '%25'
	let cleanMsg = rawMsg
		.replace(/%/g, '%25')
		.replace(/\*/g, '%2A')
		.replace(/@/g, '%40');

	if (cleanMsg.length > 60) {
		cleanMsg = cleanMsg.slice(0, 60);
		// Strip incomplete trailing percent escape if slice cut in the middle
		cleanMsg = cleanMsg.replace(/%[0-9A-Fa-f]?$/, '');
	}

	// Standard Czech Banking Association SPAYD format
	return `SPD*1.0*ACC:${ibanClean}*AM:${amountClean}*CC:${currency}*X-VS:${params.vs}*MSG:${cleanMsg}*`;
}

export async function renderSpaydQRCode(canvas: HTMLCanvasElement, params: SpaydParams): Promise<string> {
	const spaydStr = formatSpaydString(params);
	await QRCode.toCanvas(canvas, spaydStr, {
		width: 280,
		margin: 2,
		color: {
			dark: '#000000',
			light: '#ffffff'
		},
		errorCorrectionLevel: 'M'
	});
	return spaydStr;
}
