import bwipjs from 'bwip-js';
import QRCode from 'qrcode';

export function renderDataMatrix(canvas: HTMLCanvasElement, text: string): Promise<void> {
	return new Promise((resolve, reject) => {
		try {
			bwipjs.toCanvas(canvas, {
				bcid: 'datamatrix',
				text: text,
				scale: 6,
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
}

export function formatSpaydString(params: SpaydParams): string {
	const ibanClean = params.iban.replace(/\s+/g, '').toUpperCase();
	const amountClean = params.amount.toFixed(2);
	const currency = params.currency || 'CZK';
	// Standard Czech Banking Association SPAYD format
	return `SPD*1.0*ACC:${ibanClean}*AM:${amountClean}*CC:${currency}*X-VS:${params.vs}*MSG:${params.paymentId}*`;
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
