/**
 * IBAN & Czech Bank Account utilities
 * ISO 13616 & ISO 7064 MOD 97-10
 */

/**
 * Converts a standard Czech bank account number ([prefix-]account/bankCode)
 * into a valid ISO 13616 Czech IBAN (CZ...) with MOD 97 check digits.
 *
 * Example: "2101234567/2010" -> "CZ9120100000002101234567"
 * Example: "19-1234567890/0100" -> "CZ8201000000191234567890"
 */
export function czechAccountToIban(accountStr: string): string | null {
	if (!accountStr) return null;

	const clean = accountStr.replace(/^IBAN:?/i, '').replace(/\s+/g, '');
	const match = clean.match(/^(?:([0-9]{1,6})-)?([0-9]{1,10})\/([0-9]{3,4})$/);
	if (!match) return null;

	const prefixRaw = match[1] || '';
	const accountRaw = match[2];
	const bankCode = match[3].padStart(4, '0');

	// Disallow all-zero account or bank code
	if (parseInt(accountRaw, 10) === 0 || bankCode === '0000') {
		return null;
	}

	// Pad prefix to 6 digits, account to 10 digits
	const prefix = prefixRaw.padStart(6, '0');
	const account = accountRaw.padStart(10, '0');

	// BBAN = Bank code (4) + Prefix (6) + Account number (10) = 20 digits
	const bban = `${bankCode}${prefix}${account}`;

	// ISO 7064 MOD 97-10 calculation for 'CZ' (C=12, Z=35 -> 1235)
	// Check string: BBAN + 1235 + '00'
	const checkString = `${bban}123500`;

	try {
		const remainder = Number(BigInt(checkString) % 97n);
		const checkDigits = 98 - remainder;
		const checkDigitsStr = checkDigits < 10 ? `0${checkDigits}` : `${checkDigits}`;

		return `CZ${checkDigitsStr}${bban}`;
	} catch {
		return null;
	}
}

/**
 * Validates any IBAN using the standard ISO 7064 MOD 97-10 algorithm.
 * Supports national length rules for common EU countries (Czech Republic, Slovakia, Germany, etc.).
 */
export function validateIban(iban: string): boolean {
	if (!iban) return false;

	const clean = iban.replace(/\s+/g, '').toUpperCase();

	// Basic structural format: 2 letters, 2 digits, up to 30 alphanumeric characters
	if (!/^[A-Z]{2}[0-9]{2}[A-Z0-9]{11,30}$/.test(clean)) {
		return false;
	}

	// Specific length checks for common countries
	const country = clean.slice(0, 2);
	const expectedLengths: Record<string, number> = {
		CZ: 24,
		SK: 24,
		DE: 22,
		AT: 20,
		PL: 28,
		FR: 27,
		IT: 27,
		ES: 24,
		GB: 22,
		NL: 18,
		BE: 16,
		CH: 21,
		LT: 20
	};

	if (expectedLengths[country] && clean.length !== expectedLengths[country]) {
		return false;
	}

	// For CZ and SK, remainder after country code must only be numbers
	if ((country === 'CZ' || country === 'SK') && !/^[A-Z]{2}[0-9]{22}$/.test(clean)) {
		return false;
	}

	// Rearrange: move the 4 initial characters to the end
	const rearranged = clean.slice(4) + clean.slice(0, 4);

	// Convert letters to numbers (A=10, B=11, ..., Z=35)
	const numericStr = rearranged.replace(/[A-Z]/g, (ch) => `${ch.charCodeAt(0) - 55}`);

	try {
		return BigInt(numericStr) % 97n === 1n;
	} catch {
		return false;
	}
}

/**
 * Formats an IBAN with spaces every 4 characters for readability.
 * Example: "CZ9120100000002101234567" -> "CZ91 2010 0000 0021 0123 4567"
 */
export function formatIban(iban: string): string {
	if (!iban) return '';
	const clean = iban.replace(/[^A-Z0-9]/gi, '').toUpperCase();
	return clean.replace(/(.{4})(?!$)/g, '$1 ');
}

export interface AccountParseResult {
	type: 'czech' | 'iban' | 'invalid' | 'empty';
	rawIban: string | null;
	formattedIban: string | null;
	error?: string;
}

/**
 * Parses user input which can be either a Czech account number or an IBAN.
 */
export function parseAccountInput(input: string): AccountParseResult {
	const trimmed = input.trim();
	if (!trimmed) {
		return { type: 'empty', rawIban: null, formattedIban: null };
	}

	const clean = trimmed.replace(/^IBAN:?/i, '').replace(/\s+/g, '');

	// If it contains a slash or looks like a local Czech account (e.g. 1234567890/2010)
	if (clean.includes('/') || /^[0-9]+-[0-9]+\/[0-9]+$/.test(clean) || /^[0-9]+\/[0-9]+$/.test(clean)) {
		const converted = czechAccountToIban(clean);
		if (converted && validateIban(converted)) {
			return {
				type: 'czech',
				rawIban: converted,
				formattedIban: formatIban(converted)
			};
		}
		return {
			type: 'invalid',
			rawIban: null,
			formattedIban: null,
			error: 'Neplatné číslo účtu. Zadejte formát např. 2101234567/2010 nebo 19-1234567890/0100'
		};
	}

	// If it starts with 2 letters, evaluate as IBAN
	if (/^[A-Za-z]{2}/.test(clean)) {
		const upper = clean.toUpperCase();
		if (validateIban(upper)) {
			return {
				type: 'iban',
				rawIban: upper,
				formattedIban: formatIban(upper)
			};
		}
		return {
			type: 'invalid',
			rawIban: null,
			formattedIban: null,
			error: 'Neplatný IBAN nebo chybný kontrolní součet.'
		};
	}

	return {
		type: 'invalid',
		rawIban: null,
		formattedIban: null,
		error: 'Zadejte české číslo účtu (např. 2101234567/2010) nebo IBAN (např. CZ...)'
	};
}
