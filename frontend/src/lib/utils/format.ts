/**
 * Display helpers for monetary values, percentages and quantities.
 *
 * The backend returns all numbers as strings to avoid float precision loss.
 * These helpers parse them safely for display only — never for arithmetic.
 */

export function formatMoney(amount: string, currency: string, locale = 'en-US'): string {
	const n = Number(amount);
	if (Number.isNaN(n)) return `${amount} ${currency}`;
	const decimals = smartDecimals(n);
	const formatter = new Intl.NumberFormat(locale, {
		minimumFractionDigits: 2,
		maximumFractionDigits: decimals
	});
	return `${formatter.format(n)} ${currency}`;
}

/** Return enough decimal places so small values stay meaningful. */
function smartDecimals(n: number): number {
	const abs = Math.abs(n);
	if (abs === 0 || abs >= 1) return 2;
	// Count leading zeros after the decimal point, then show 2 significant digits
	const digits = -Math.floor(Math.log10(abs)) + 1;
	return Math.min(digits, 8);
}

/**
 * Format a Money value with the user-chosen display currency as the primary
 * line and the original native currency as the secondary line. The user picks
 * the display currency from the header toggle, so it should always be the one
 * they see first.
 */
export function formatDualMoney(money: {
	amount: string;
	currency: string;
	display?: { amount: string; currency: string };
}): { primary: string; secondary: string | null } {
	if (!money.display || money.display.currency === money.currency) {
		return { primary: formatMoney(money.amount, money.currency), secondary: null };
	}
	return {
		primary: formatMoney(money.display.amount, money.display.currency),
		secondary: `≈ ${formatMoney(money.amount, money.currency)}`
	};
}

export function formatQuantity(qty: string): string {
	const n = Number(qty);
	if (Number.isNaN(n)) return qty;
	return new Intl.NumberFormat('en-US', {
		minimumFractionDigits: 0,
		maximumFractionDigits: 8
	}).format(n);
}

/**
 * Format basis points (1% = 100 bp) into a percent string with sign.
 */
export function formatPctBP(bp: number): string {
	const pct = bp / 100;
	const sign = pct > 0 ? '+' : '';
	return `${sign}${pct.toFixed(2)}%`;
}

/**
 * Returns a Tailwind class indicating P&L direction.
 */
export function pnlClass(amountStr: string): string {
	const n = Number(amountStr);
	if (Number.isNaN(n) || n === 0) return 'text-muted-foreground';
	return n > 0 ? 'text-success' : 'text-destructive';
}
