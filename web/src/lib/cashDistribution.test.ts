import assert from 'node:assert';
import {
	calculateCashDistribution,
	aggregateCashDistributions,
	formatCashBreakdownSummary,
	CZK_DENOMINATIONS
} from './cashDistribution.ts';

console.log('Testing calculateCashDistribution...');

// Test 0 Kč
const b0 = calculateCashDistribution(0);
assert.strictEqual(b0.amount, 0);
assert.strictEqual(b0.totalPieces, 0);
assert.strictEqual(b0.items.every((i) => i.count === 0), true);

// Test 165 Kč: 100 + 50 + 10 + 5
const b165 = calculateCashDistribution(165);
assert.strictEqual(b165.amount, 165);
const map165 = new Map(b165.items.map((i) => [i.denomination.value, i.count]));
assert.strictEqual(map165.get(100), 1);
assert.strictEqual(map165.get(50), 1);
assert.strictEqual(map165.get(10), 1);
assert.strictEqual(map165.get(5), 1);
assert.strictEqual(b165.totalPieces, 4);
assert.strictEqual(b165.banknotePieces, 1);
assert.strictEqual(b165.coinPieces, 3);

// Invariant: sum of item value equals amount
function verifyInvariant(amount: number) {
	const b = calculateCashDistribution(amount);
	const sum = b.items.reduce((acc, item) => acc + item.count * item.denomination.value, 0);
	assert.strictEqual(sum, Math.round(amount), `Invariant failed for ${amount}`);
}

[1, 2, 3, 7, 19, 45, 88, 165, 380, 750, 1370, 2450, 6800, 12345].forEach(verifyInvariant);
console.log('Single amount invariants verified!');

// Test aggregateCashDistributions
console.log('Testing aggregateCashDistributions...');
const sellers = [
	{ id: '1', name: 'Petr', email: 'petr@test.cz', amount: 600, payoutToBank: false },
	{ id: '2', name: 'Jana', email: 'jana@test.cz', amount: 700, payoutToBank: false },
	{ id: '3', name: 'Karel', email: 'karel@test.cz', amount: 0, payoutToBank: false } // should be skipped
];

const agg = aggregateCashDistributions(sellers);
assert.strictEqual(agg.sellerCount, 2);
assert.strictEqual(agg.totalAmount, 1300);

const aggMap = new Map(agg.items.map((i) => [i.denomination.value, i.count]));
assert.strictEqual(aggMap.get(500), 2, 'Should have 2x 500 Kč banknotes');
assert.strictEqual(aggMap.get(200), 1, 'Should have 1x 200 Kč banknote');
assert.strictEqual(aggMap.get(100), 1, 'Should have 1x 100 Kč banknote');
assert.strictEqual(aggMap.get(1000), 0, 'Should have 0x 1000 Kč banknotes because neither individual needed 1000');

// Test formatCashBreakdownSummary
const formatted = formatCashBreakdownSummary(agg, 'Test Rozpis');
const normalizedFormatted = formatted.replace(/\s+/g, ' ');
assert.ok(normalizedFormatted.includes('Celková částka: 1 300 Kč') || normalizedFormatted.includes('1300 Kč'));
assert.ok(formatted.includes('500 Kč'));
assert.ok(formatted.includes('Petr'));
assert.ok(formatted.includes('Jana'));

console.log('All cash distribution tests passed successfully!');
