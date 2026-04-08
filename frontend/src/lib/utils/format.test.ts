import { describe, expect, it } from 'vitest';
import { formatMoney, formatPctBP, formatQuantity, pnlClass } from './format';

describe('formatMoney', () => {
	it('formats decimal with currency suffix', () => {
		expect(formatMoney('1234.56', 'USDT')).toBe('1,234.56 USDT');
	});
	it('falls back to raw on NaN', () => {
		expect(formatMoney('not-a-number', 'USDT')).toBe('not-a-number USDT');
	});
});

describe('formatPctBP', () => {
	it('positive', () => expect(formatPctBP(2000)).toBe('+20.00%'));
	it('negative', () => expect(formatPctBP(-150)).toBe('-1.50%'));
	it('zero', () => expect(formatPctBP(0)).toBe('0.00%'));
});

describe('formatQuantity', () => {
	it('thin formatting', () => expect(formatQuantity('0.12345678')).toBe('0.12345678'));
});

describe('pnlClass', () => {
	it('positive is success', () => expect(pnlClass('1')).toContain('success'));
	it('negative is destructive', () => expect(pnlClass('-1')).toContain('destructive'));
	it('zero is muted', () => expect(pnlClass('0')).toContain('muted'));
});
