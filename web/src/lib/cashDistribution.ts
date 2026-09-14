export type DenominationType = 'banknote' | 'coin';

export interface Denomination {
	value: number;
	type: DenominationType;
	label: string;
	colorClass: string;
	bgClass: string;
}

export const CZK_DENOMINATIONS: readonly Denomination[] = [
	{ value: 5000, type: 'banknote', label: '5 000 Kč', colorClass: 'text-purple-900 border-purple-800', bgClass: 'bg-purple-50' },
	{ value: 2000, type: 'banknote', label: '2 000 Kč', colorClass: 'text-emerald-900 border-emerald-800', bgClass: 'bg-emerald-50' },
	{ value: 1000, type: 'banknote', label: '1 000 Kč', colorClass: 'text-blue-900 border-blue-800', bgClass: 'bg-blue-50' },
	{ value: 500, type: 'banknote', label: '500 Kč', colorClass: 'text-amber-900 border-amber-800', bgClass: 'bg-amber-50' },
	{ value: 200, type: 'banknote', label: '200 Kč', colorClass: 'text-orange-900 border-orange-800', bgClass: 'bg-orange-50' },
	{ value: 100, type: 'banknote', label: '100 Kč', colorClass: 'text-teal-900 border-teal-800', bgClass: 'bg-teal-50' },
	{ value: 50, type: 'coin', label: '50 Kč', colorClass: 'text-amber-950 border-amber-900', bgClass: 'bg-amber-100/70' },
	{ value: 20, type: 'coin', label: '20 Kč', colorClass: 'text-amber-800 border-amber-700', bgClass: 'bg-amber-50' },
	{ value: 10, type: 'coin', label: '10 Kč', colorClass: 'text-orange-800 border-orange-700', bgClass: 'bg-orange-50' },
	{ value: 5, type: 'coin', label: '5 Kč', colorClass: 'text-neutral-800 border-neutral-700', bgClass: 'bg-neutral-100' },
	{ value: 2, type: 'coin', label: '2 Kč', colorClass: 'text-neutral-800 border-neutral-700', bgClass: 'bg-neutral-100' },
	{ value: 1, type: 'coin', label: '1 Kč', colorClass: 'text-neutral-800 border-neutral-700', bgClass: 'bg-neutral-100' }
] as const;

export interface DenominationCount {
	denomination: Denomination;
	count: number;
	totalAmount: number;
}

export interface CashBreakdown {
	amount: number;
	totalPieces: number;
	banknotePieces: number;
	coinPieces: number;
	items: DenominationCount[];
}

export interface SellerCashInput {
	id: string;
	name: string;
	email: string;
	amount: number;
	payoutToBank: boolean;
}

export interface SellerBreakdownItem extends SellerCashInput {
	breakdown: CashBreakdown;
}

export interface AggregatedCashDistribution {
	sellerCount: number;
	totalAmount: number;
	totalPieces: number;
	banknotePieces: number;
	coinPieces: number;
	items: DenominationCount[];
	sellerBreakdowns: SellerBreakdownItem[];
}

/**
 * Calculates optimal minimal piece breakdown for a single monetary amount in CZK.
 */
export function calculateCashDistribution(amount: number): CashBreakdown {
	const rounded = Math.max(0, Math.round(amount));
	let remaining = rounded;

	let totalPieces = 0;
	let banknotePieces = 0;
	let coinPieces = 0;

	const items: DenominationCount[] = CZK_DENOMINATIONS.map((denomination) => {
		let count = 0;
		if (remaining >= denomination.value) {
			count = Math.floor(remaining / denomination.value);
			remaining %= denomination.value;
		}

		totalPieces += count;
		if (denomination.type === 'banknote') {
			banknotePieces += count;
		} else {
			coinPieces += count;
		}

		return {
			denomination,
			count,
			totalAmount: count * denomination.value
		};
	});

	return {
		amount: rounded,
		totalPieces,
		banknotePieces,
		coinPieces,
		items
	};
}

/**
 * Aggregates cash distributions for a group of sellers.
 * Crucially, it computes the optimal breakdown for EACH individual seller first,
 * and then sums the counts per denomination so that the cashier has exact change
 * to pay each seller separately.
 */
export function aggregateCashDistributions(sellers: SellerCashInput[]): AggregatedCashDistribution {
	const countsMap = new Map<number, number>();
	for (const d of CZK_DENOMINATIONS) {
		countsMap.set(d.value, 0);
	}

	let totalAmount = 0;
	let totalPieces = 0;
	let banknotePieces = 0;
	let coinPieces = 0;

	const sellerBreakdowns: SellerBreakdownItem[] = [];

	for (const seller of sellers) {
		const positiveAmount = Math.max(0, Math.round(seller.amount));
		if (positiveAmount <= 0) continue;

		totalAmount += positiveAmount;
		const breakdown = calculateCashDistribution(positiveAmount);
		sellerBreakdowns.push({
			...seller,
			amount: positiveAmount,
			breakdown
		});

		for (const item of breakdown.items) {
			if (item.count > 0) {
				const current = countsMap.get(item.denomination.value) || 0;
				countsMap.set(item.denomination.value, current + item.count);

				totalPieces += item.count;
				if (item.denomination.type === 'banknote') {
					banknotePieces += item.count;
				} else {
					coinPieces += item.count;
				}
			}
		}
	}

	const items: DenominationCount[] = CZK_DENOMINATIONS.map((denomination) => {
		const count = countsMap.get(denomination.value) || 0;
		return {
			denomination,
			count,
			totalAmount: count * denomination.value
		};
	});

	return {
		sellerCount: sellerBreakdowns.length,
		totalAmount,
		totalPieces,
		banknotePieces,
		coinPieces,
		items,
		sellerBreakdowns
	};
}

/**
 * Formats an aggregated breakdown or single breakdown into clean plaintext for copying/printing.
 */
export function formatCashBreakdownSummary(result: AggregatedCashDistribution, title = 'Rozpis hotovosti pro pokladnu'): string {
	const lines: string[] = [];
	lines.push(`=== ${title.toUpperCase()} ===`);
	lines.push(`Celková částka: ${result.totalAmount.toLocaleString('cs-CZ')} Kč`);
	lines.push(`Počet prodejců: ${result.sellerCount}`);
	lines.push(`Celkem kusů: ${result.totalPieces} ks (bankovky: ${result.banknotePieces} ks, mince: ${result.coinPieces} ks)`);
	lines.push('');

	lines.push('--- BANKOVKY ---');
	const banknotes = result.items.filter((i) => i.denomination.type === 'banknote');
	for (const b of banknotes) {
		if (b.count > 0) {
			lines.push(`  ${b.denomination.label.padEnd(9)} : ${String(b.count).padStart(4)} ks  (= ${b.totalAmount.toLocaleString('cs-CZ')} Kč)`);
		}
	}

	lines.push('');
	lines.push('--- MINCE ---');
	const coins = result.items.filter((i) => i.denomination.type === 'coin');
	for (const c of coins) {
		if (c.count > 0) {
			lines.push(`  ${c.denomination.label.padEnd(9)} : ${String(c.count).padStart(4)} ks  (= ${c.totalAmount.toLocaleString('cs-CZ')} Kč)`);
		}
	}

	if (result.sellerBreakdowns.length > 0) {
		lines.push('');
		lines.push('--- JEDNOTLIVÍ PRODEJCI ---');
		for (const s of result.sellerBreakdowns) {
			const activeChips = s.breakdown.items
				.filter((i) => i.count > 0)
				.map((i) => `${i.count}× ${i.denomination.label}`)
				.join(', ');
			lines.push(`- ${s.name || s.email} (${s.amount} Kč): ${activeChips}`);
		}
	}

	return lines.join('\n');
}
