const COLORS: Record<string, string> = {
	BTC: '#f7931a',
	ETH: '#627eea',
	SOL: '#9945ff',
	BNB: '#f3ba2f',
	DOGE: '#c2a633',
	XRP: '#00aae4',
	ADA: '#0033ad',
	AVAX: '#e84142',
	DOT: '#e6007a',
	MATIC: '#8247e5'
};

export function assetColor(symbol: string): string {
	return (
		COLORS[symbol] ??
		`hsl(${[...symbol].reduce((a, c) => a + c.charCodeAt(0), 0) % 360}, 60%, 55%)`
	);
}
